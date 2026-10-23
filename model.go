package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

// SortMode selects the panel sort order.
type SortMode int

const (
	SortName SortMode = iota
	SortExt
	SortSize
	SortTime
)

var sortNames = []string{"Name", "Extension", "Size", "Modified"}

// Entry is one row of a panel.
type Entry struct {
	Name    string
	IsDir   bool
	IsLink  bool
	Hidden  bool
	IsUp    bool // the ".." pseudo entry
	Size    int64
	ModTime time.Time
	Mode    fs.FileMode
}

// Ext returns the extension without the dot, "" for directories.
func (e *Entry) Ext() string {
	if e.IsDir {
		return ""
	}
	return strings.TrimPrefix(filepath.Ext(e.Name), ".")
}

// Tab is one tab of a panel: a directory listing with cursor and selection.
type Tab struct {
	Path     string
	Entries  []Entry
	Cursor   int
	Top      int
	Selected map[string]bool
	Sort     SortMode
	SortRev  bool
	Notes    map[string]string
	Err      error
	posMem   map[string]string // directory -> name the cursor was on
}

func newTab(path string) *Tab {
	return &Tab{Path: path, Selected: map[string]bool{}, posMem: map[string]string{}}
}

// Load (re)reads the directory, keeping cursor and selection where possible.
func (t *Tab) Load(showHidden bool, notesFile string) {
	var keep string
	if cur := t.Current(); cur != nil {
		keep = cur.Name
	}
	entries, err := listDir(t.Path, showHidden, notesFile)
	t.Err = err
	if parent := filepath.Dir(t.Path); parent != t.Path {
		entries = append([]Entry{{Name: "..", IsDir: true, IsUp: true}}, entries...)
	}
	sortEntries(entries, t.Sort, t.SortRev)
	t.Entries = entries
	t.Notes = loadNotes(t.Path, notesFile)
	for name := range t.Selected {
		if t.indexOf(name) < 0 {
			delete(t.Selected, name)
		}
	}
	if keep != "" {
		if i := t.indexOf(keep); i >= 0 {
			t.Cursor = i
		}
	}
	t.clamp()
}

func (t *Tab) indexOf(name string) int {
	for i := range t.Entries {
		if t.Entries[i].Name == name {
			return i
		}
	}
	return -1
}

func (t *Tab) clamp() {
	if t.Cursor >= len(t.Entries) {
		t.Cursor = len(t.Entries) - 1
	}
	if t.Cursor < 0 {
		t.Cursor = 0
	}
}

// Current returns the entry under the cursor, or nil for an empty listing.
func (t *Tab) Current() *Entry {
	if t.Cursor < 0 || t.Cursor >= len(t.Entries) {
		return nil
	}
	return &t.Entries[t.Cursor]
}

func (t *Tab) MoveCursor(delta int) {
	t.Cursor += delta
	t.clamp()
}

// SetCursorOn places the cursor on the named entry if present.
func (t *Tab) SetCursorOn(name string) bool {
	if i := t.indexOf(name); i >= 0 {
		t.Cursor = i
		return true
	}
	return false
}

// EnsureVisible adjusts Top so that the cursor is within rows visible rows.
func (t *Tab) EnsureVisible(rows int) {
	if rows < 1 {
		rows = 1
	}
	if t.Cursor < t.Top {
		t.Top = t.Cursor
	}
	if t.Cursor >= t.Top+rows {
		t.Top = t.Cursor - rows + 1
	}
	if t.Top > len(t.Entries)-rows {
		t.Top = len(t.Entries) - rows
	}
	if t.Top < 0 {
		t.Top = 0
	}
}

func (t *Tab) ToggleSelect() {
	e := t.Current()
	if e == nil || e.IsUp {
		return
	}
	if t.Selected[e.Name] {
		delete(t.Selected, e.Name)
	} else {
		t.Selected[e.Name] = true
	}
}

// Targets returns the selected entries, or the current one if nothing is
// selected. The ".." entry is never a target.
func (t *Tab) Targets() []Entry {
	var out []Entry
	if len(t.Selected) > 0 {
		for _, e := range t.Entries {
			if t.Selected[e.Name] {
				out = append(out, e)
			}
		}
		return out
	}
	if e := t.Current(); e != nil && !e.IsUp {
		out = append(out, *e)
	}
	return out
}

// SelectedStats returns count and total size of selected files.
func (t *Tab) SelectedStats() (int, int64) {
	var n int
	var size int64
	for _, e := range t.Entries {
		if t.Selected[e.Name] {
			n++
			size += e.Size
		}
	}
	return n, size
}

func listDir(path string, showHidden bool, notesFile string) ([]Entry, error) {
	des, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(des))
	for _, de := range des {
		name := de.Name()
		if name == notesFile {
			continue
		}
		e := Entry{Name: name}
		info, ierr := de.Info()
		if ierr == nil {
			e.Mode = info.Mode()
			e.Size = info.Size()
			e.ModTime = info.ModTime()
			e.Hidden = strings.HasPrefix(name, ".") || hasHiddenAttr(info)
		}
		if de.Type()&fs.ModeSymlink != 0 {
			e.IsLink = true
			if st, serr := os.Stat(filepath.Join(path, name)); serr == nil {
				e.IsDir = st.IsDir()
				if !e.IsDir {
					e.Size = st.Size()
				}
			}
		} else {
			e.IsDir = de.IsDir()
		}
		if e.Hidden && !showHidden {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func sortEntries(entries []Entry, mode SortMode, rev bool) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := &entries[i], &entries[j]
		if a.IsUp != b.IsUp {
			return a.IsUp
		}
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		var less, eq bool
		switch mode {
		case SortExt:
			c := naturalCompare(a.Ext(), b.Ext())
			less, eq = c < 0, c == 0
		case SortSize:
			less, eq = a.Size < b.Size, a.Size == b.Size
		case SortTime:
			less, eq = a.ModTime.Before(b.ModTime), a.ModTime.Equal(b.ModTime)
		}
		if eq || mode == SortName {
			c := naturalCompare(a.Name, b.Name)
			less = c < 0
		}
		if rev {
			return !less && naturalCompare(a.Name, b.Name) != 0
		}
		return less
	})
}

// naturalCompare compares case-insensitively with embedded numbers compared
// by value, so "file2" sorts before "file10".
func naturalCompare(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	i, j := 0, 0
	for i < len(ra) && j < len(rb) {
		ca, cb := ra[i], rb[j]
		if unicode.IsDigit(ca) && unicode.IsDigit(cb) {
			si := i
			for i < len(ra) && unicode.IsDigit(ra[i]) {
				i++
			}
			sj := j
			for j < len(rb) && unicode.IsDigit(rb[j]) {
				j++
			}
			na := strings.TrimLeft(string(ra[si:i]), "0")
			nb := strings.TrimLeft(string(rb[sj:j]), "0")
			if len(na) != len(nb) {
				if len(na) < len(nb) {
					return -1
				}
				return 1
			}
			if na != nb {
				if na < nb {
					return -1
				}
				return 1
			}
			continue
		}
		la, lb := unicode.ToLower(ca), unicode.ToLower(cb)
		if la != lb {
			if la < lb {
				return -1
			}
			return 1
		}
		i++
		j++
	}
	switch {
	case len(ra)-i < len(rb)-j:
		return -1
	case len(ra)-i > len(rb)-j:
		return 1
	}
	return strings.Compare(a, b)
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func absPath(p string) string {
	p = expandPath(p)
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
