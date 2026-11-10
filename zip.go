package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Browsing inside a zip archive. A tab with Zip != nil lists one directory
// inside an archive instead of one on disk. The archive is never written
// to: everything that would change it is refused, while files can be
// looked at, opened through a temporary copy, and extracted to the other
// panel.

// zipItem is one entry of an archive, named by its full path inside it.
type zipItem struct {
	name  string // slash separated, no leading or trailing slash
	isDir bool
	size  int64
	mod   time.Time
}

// zipState is what a tab needs to show a place inside an archive.
type zipState struct {
	archive string    // the .zip on disk
	dir     string    // directory inside it, "" at the root
	items   []zipItem // the whole archive, read once when entering
}

// isZipName reports whether a file is an archive NEAR can browse. Only
// plain .zip: other zip-based formats (.docx, .jar) belong to the programs
// that own them, and Enter should hand those over as before.
func isZipName(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".zip")
}

// label is the panel's path row: the archive followed by the place inside
// it, so it reads like a path.
func (z *zipState) label() string {
	if z.dir == "" {
		return z.archive
	}
	return filepath.Join(z.archive, filepath.FromSlash(z.dir))
}

// readZip lists an archive, adding the directories that are only implied
// by the names of the files in them.
func readZip(archive string) ([]zipItem, error) {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var items []zipItem
	dirs := map[string]bool{}
	add := func(it zipItem) {
		if it.isDir {
			if dirs[it.name] {
				return
			}
			dirs[it.name] = true
		}
		items = append(items, it)
	}
	for _, f := range r.File {
		name := strings.Trim(path.Clean(strings.ReplaceAll(f.Name, `\`, "/")), "/")
		if name == "" || name == "." || strings.HasPrefix(name, "../") {
			continue // nothing outside the archive's own tree
		}
		isDir := f.FileInfo().IsDir()
		// Every directory above it exists even when the archive does not
		// say so.
		for p := path.Dir(name); p != "." && p != "/"; p = path.Dir(p) {
			add(zipItem{name: p, isDir: true, mod: f.Modified})
		}
		add(zipItem{name: name, isDir: isDir, size: int64(f.UncompressedSize64), mod: f.Modified})
	}
	return items, nil
}

// list returns the entries of the current directory inside the archive.
func (z *zipState) list() []Entry {
	prefix := ""
	if z.dir != "" {
		prefix = z.dir + "/"
	}
	var out []Entry
	for _, it := range z.items {
		if !strings.HasPrefix(it.name, prefix) {
			continue
		}
		rest := it.name[len(prefix):]
		if rest == "" || strings.Contains(rest, "/") {
			continue // deeper down, shown when that directory is opened
		}
		mode := os.FileMode(0o644)
		if it.isDir {
			mode |= os.ModeDir
		}
		out = append(out, Entry{Name: rest, IsDir: it.isDir, Size: it.size, ModTime: it.mod, Mode: mode})
	}
	sort.Slice(out, func(i, j int) bool { return naturalCompare(out[i].Name, out[j].Name) < 0 })
	return out
}

// find returns the item with the given path inside the archive.
func (z *zipState) find(name string) (zipItem, bool) {
	for _, it := range z.items {
		if it.name == name {
			return it, true
		}
	}
	return zipItem{}, false
}

// inner is the full path inside the archive of an entry of the current
// directory.
func (z *zipState) inner(name string) string {
	if z.dir == "" {
		return name
	}
	return z.dir + "/" + name
}

// ---------------------------------------------------------------- panel

// enterArchive opens an archive in a panel.
func (a *App) enterArchive(t *Tab, archive string) {
	items, err := readZip(archive)
	if err != nil {
		a.setErr(fmt.Errorf("%s: %v", filepath.Base(archive), err))
		return
	}
	if cur := t.Current(); cur != nil {
		t.posMem[t.Path] = cur.Name
	}
	t.Zip = &zipState{archive: archive, items: items}
	t.Path = archive
	t.Find = nil
	t.Cursor, t.Top = 0, 0
	t.Selected = map[string]bool{}
	a.applyZip(t)
	a.setStatus(fmt.Sprintf("%s: %d entries, read only", filepath.Base(archive), len(items)))
}

// leaveArchive returns to the directory the archive is in.
func (a *App) leaveArchive(t *Tab) {
	z := t.Zip
	t.Zip = nil
	dir, name := filepath.Split(z.archive)
	t.Path = filepath.Clean(dir)
	t.Cursor, t.Top = 0, 0
	t.Selected = map[string]bool{}
	a.loadTab(t)
	t.SetCursorOn(name)
}

// zipGoTo moves to a directory inside the archive ("" is its root).
func (a *App) zipGoTo(t *Tab, dir string) {
	if cur := t.Current(); cur != nil {
		t.posMem[t.Zip.label()] = cur.Name
	}
	t.Zip.dir = dir
	t.Cursor, t.Top = 0, 0
	t.Selected = map[string]bool{}
	a.applyZip(t)
	if name := t.posMem[t.Zip.label()]; name != "" {
		t.SetCursorOn(name)
	}
}

// applyZip installs the current archive directory as the tab's listing.
func (a *App) applyZip(t *Tab) {
	t.Apply(t.Zip.list(), nil, nil)
	t.CachedAt = time.Time{}
}

// zipUp goes one level up inside the archive, or out of it at the root.
func (a *App) zipUp(t *Tab) {
	if t.Zip.dir == "" {
		a.leaveArchive(t)
		return
	}
	up := path.Dir(t.Zip.dir)
	if up == "." || up == "/" {
		up = ""
	}
	a.zipGoTo(t, up)
}

// diskDir is the directory a panel stands for on disk: inside an archive
// that is the directory the archive itself is in, so a copy aimed at the
// other panel lands somewhere real instead of inside a file.
func (t *Tab) diskDir() string {
	if t.Zip != nil {
		return filepath.Dir(t.Zip.archive)
	}
	return t.Path
}

// inArchive reports whether the active panel is inside an archive, and
// says so when it is: the archive cannot be changed.
func (a *App) inArchive(what string) bool {
	if a.tab().Zip == nil {
		return false
	}
	a.setErr(fmt.Errorf("%s is not possible inside an archive", what))
	return true
}

// ---------------------------------------------------------------- files

// zipTempDir is where files taken out of an archive to be looked at are
// put; it is removed when NEAR quits.
var zipTempDir string

func (a *App) zipTemp() (string, error) {
	if zipTempDir != "" {
		return zipTempDir, nil
	}
	d, err := os.MkdirTemp("", "near-zip-")
	if err != nil {
		return "", err
	}
	zipTempDir = d
	return d, nil
}

// cleanZipTemp removes the extracted copies; NEAR calls it when quitting.
func cleanZipTemp() {
	if zipTempDir != "" {
		_ = os.RemoveAll(zipTempDir)
		zipTempDir = ""
	}
}

// extractOne writes one file out of an archive to dst.
func extractOne(archive, name, dst string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if strings.Trim(path.Clean(strings.ReplaceAll(f.Name, `\`, "/")), "/") != name || f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			os.Remove(dst)
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		return os.Chtimes(dst, f.Modified, f.Modified)
	}
	return fmt.Errorf("%s is not in the archive", name)
}

// zipCopyOut extracts one file to the temporary directory and returns the
// copy's path, so it can be viewed or opened.
func (a *App) zipCopyOut(z *zipState, name string) (string, error) {
	dir, err := a.zipTemp()
	if err != nil {
		return "", err
	}
	dst := filepath.Join(dir, filepath.FromSlash(name))
	if err := extractOne(z.archive, name, dst); err != nil {
		return "", err
	}
	return dst, nil
}

// extractToDir writes the selected entries (directories with everything
// below them) into dst, reporting progress and stopping on Esc.
func (a *App) extractToDir(t *Tab, targets []Entry, dst string) {
	z := t.Zip
	// Which items the selection covers.
	var wanted []zipItem
	for _, e := range targets {
		inner := z.inner(e.Name)
		for _, it := range z.items {
			if it.name == inner || strings.HasPrefix(it.name, inner+"/") {
				wanted = append(wanted, it)
			}
		}
	}
	base := z.dir
	if base != "" {
		base += "/"
	}
	done, failed := 0, 0
	var lastErr error
	for _, it := range wanted {
		rel := strings.TrimPrefix(it.name, base)
		out := filepath.Join(dst, filepath.FromSlash(rel))
		// An archive may name entries that climb out of the destination.
		if !underAnyRoot(out, []string{dst}) {
			failed++
			lastErr = fmt.Errorf("%s points outside the destination", rel)
			continue
		}
		if it.isDir {
			if err := os.MkdirAll(out, 0o755); err != nil {
				failed++
				lastErr = err
			}
			continue
		}
		if !a.progress("Extracting", rel, -1, -1) {
			a.setStatus(fmt.Sprintf("Extraction stopped: %d files written", done))
			a.reloadAll()
			return
		}
		if err := extractOne(z.archive, it.name, out); err != nil {
			failed++
			lastErr = err
			continue
		}
		done++
	}
	t.Selected = map[string]bool{}
	a.reloadAll()
	if failed > 0 {
		a.setErr(fmt.Errorf("extracted %d files, %d failed: %v", done, failed, lastErr))
		return
	}
	a.setStatus(fmt.Sprintf("Extracted %d files to %s", done, fitRight(dst, 50)))
}
