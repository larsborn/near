package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeZip writes an archive with the given contents; a name ending in "/"
// is an explicit directory entry.
func makeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for name, body := range files {
		if strings.HasSuffix(name, "/") {
			if _, err := w.Create(name); err != nil {
				t.Fatal(err)
			}
			continue
		}
		e, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func zipApp(t *testing.T) (*App, string, string) {
	t.Helper()
	a, data := testApp(t)
	archive := filepath.Join(data, "bundle.zip")
	makeZip(t, archive, map[string]string{
		"readme.txt":      "hello from the archive",
		"docs/guide.md":   "# guide",
		"docs/deep/x.txt": "deep",
		"empty/":          "",
	})
	a.reloadAll()
	return a, data, archive
}

// Enter on a .zip browses it, directories inside open, and .. comes back
// out to the directory the archive is in.
func TestZipNavigation(t *testing.T) {
	a, data, _ := zipApp(t)
	a.tab().SetCursorOn("bundle.zip")
	key(a, "Enter")
	tb := a.tab()
	if tb.Zip == nil {
		t.Fatalf("Enter on a zip should browse it, path is %s", tb.Path)
	}
	if got := strings.Join(names(tb), " "); got != ".. docs empty readme.txt" {
		t.Fatalf("archive root lists %q", got)
	}
	a.draw()
	if s := screenText(a); !strings.Contains(s, "bundle.zip") {
		t.Fatalf("the path row should name the archive:\n%s", s)
	}
	// Into a directory, including one that exists only implicitly.
	tb.SetCursorOn("docs")
	key(a, "Enter")
	if got := strings.Join(names(tb), " "); got != ".. deep guide.md" {
		t.Fatalf("docs lists %q", got)
	}
	if tb.Zip.dir != "docs" {
		t.Fatalf("inner directory is %q", tb.Zip.dir)
	}
	tb.SetCursorOn("deep")
	key(a, "Enter")
	if got := strings.Join(names(tb), " "); got != ".. x.txt" {
		t.Fatalf("docs/deep lists %q", got)
	}
	// Back up twice to the archive root, then out of the archive.
	key(a, "Esc")
	key(a, "Esc")
	if tb.Zip == nil || tb.Zip.dir != "" {
		t.Fatalf("two Escs should reach the archive root, dir=%v", tb.Zip)
	}
	key(a, "Esc")
	if tb.Zip != nil || tb.Path != data {
		t.Fatalf("Esc at the root should leave the archive, path is %s", tb.Path)
	}
	if cur := tb.Current(); cur == nil || cur.Name != "bundle.zip" {
		t.Fatalf("the cursor should return to the archive, it is on %v", cur)
	}
	// Sizes come from the archive.
	key(a, "Enter")
	for _, e := range tb.Entries {
		if e.Name == "readme.txt" && e.Size != int64(len("hello from the archive")) {
			t.Fatalf("readme.txt has size %d", e.Size)
		}
	}
}

// F3 inside an archive shows the file, through a temporary copy.
func TestZipViewFile(t *testing.T) {
	a, _, _ := zipApp(t)
	t.Cleanup(cleanZipTemp)
	a.tab().SetCursorOn("bundle.zip")
	key(a, "Enter")
	a.tab().SetCursorOn("readme.txt")
	key(a, "F3")
	v, ok := a.top().(*Viewer)
	if !ok {
		t.Fatalf("F3 should open the viewer, got %T", a.top())
	}
	if len(v.Lines) == 0 || v.Lines[0] != "hello from the archive" {
		t.Fatalf("the viewer shows %q", v.Lines)
	}
}

// F5 extracts the selection to the other panel; directories bring their
// contents along.
func TestZipExtract(t *testing.T) {
	a, data, _ := zipApp(t)
	out := filepath.Join(filepath.Dir(data), "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.navigate(a.other(), out, ""); err != nil {
		t.Fatal(err)
	}
	a.tab().SetCursorOn("bundle.zip")
	key(a, "Enter")
	a.tab().Selected["docs"] = true
	a.tab().Selected["readme.txt"] = true
	key(a, "F5")
	if _, ok := a.top().(*CopyMoveDialog); !ok {
		t.Fatalf("F5 should ask where to extract, got %T", a.top())
	}
	key(a, "Enter")
	for _, rel := range []string{"readme.txt", "docs/guide.md", "docs/deep/x.txt"} {
		p := filepath.Join(out, filepath.FromSlash(rel))
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s was not extracted: %v", rel, err)
		}
		if rel == "readme.txt" && string(b) != "hello from the archive" {
			t.Fatalf("%s holds %q", rel, b)
		}
	}
	if !strings.Contains(a.status, "Extracted") {
		t.Fatalf("status = %q", a.status)
	}
}

// Nothing that would change the archive is allowed, and nothing on disk
// is touched by trying.
func TestZipIsReadOnly(t *testing.T) {
	a, data, archive := zipApp(t)
	a.tab().SetCursorOn("bundle.zip")
	key(a, "Enter")
	a.tab().SetCursorOn("readme.txt")
	before, err := os.Stat(archive)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"F8", "Shift+F8", "Shift+F6", "F7", "F4", "F6", "Ctrl+F6", "Ctrl+N", "Ctrl+G", "Alt+F7"} {
		a.status, a.statusErr = "", false
		key(a, k)
		if len(a.dialogs) > 0 {
			t.Fatalf("%s opened %T inside an archive", k, a.top())
		}
		if !a.statusErr {
			t.Fatalf("%s should have been refused inside an archive, status %q", k, a.status)
		}
	}
	// The archive and the directory it is in are untouched.
	after, err := os.Stat(archive)
	if err != nil || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("the archive changed: %v", err)
	}
	des, _ := os.ReadDir(data)
	if len(des) != 8 { // the test tree plus bundle.zip
		t.Fatalf("the directory holding the archive changed: %d entries", len(des))
	}
	// The listing inside is still there.
	if got := strings.Join(names(a.tab()), " "); got != ".. docs empty readme.txt" {
		t.Fatalf("listing after the refusals: %q", got)
	}
}
