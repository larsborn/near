//go:build windows

package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The index catches up with changes made "while NEAR was closed" from the
// NTFS change journal instead of rebuilding: files and directories created,
// renamed and deleted below the root are reconciled.
func TestJournalCatchUp(t *testing.T) {
	root := filepath.Join(os.Getenv("LOCALAPPDATA"), "Temp", "near-usn-"+filepath.Base(t.TempDir()))
	if journalVolume(root) == "" {
		t.Skip("no drive letter")
	}
	if err := os.MkdirAll(filepath.Join(root, "keep", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	for _, f := range []string{"a.txt", "keep/b.txt", "keep/sub/c.txt", "gone.txt"} {
		os.WriteFile(filepath.Join(root, filepath.FromSlash(f)), []byte("x"), 0o644)
	}
	os.MkdirAll(filepath.Join(root, "olddir"), 0o755)
	os.WriteFile(filepath.Join(root, "olddir", "d.txt"), []byte("x"), 0o644)

	ix := newIndex()
	ix.roots = []string{root}
	ix.exclude = newExcludeSet(nil)
	ix.Build()
	if len(ix.journals) == 0 {
		t.Skip("change journal not readable on this volume")
	}
	if ix.Count() != 8 {
		t.Fatalf("built %d names, want 8", ix.Count())
	}

	// Changes "while closed".
	os.WriteFile(filepath.Join(root, "new.txt"), []byte("y"), 0o644)
	os.Remove(filepath.Join(root, "gone.txt"))
	os.Rename(filepath.Join(root, "keep", "sub", "c.txt"), filepath.Join(root, "keep", "sub", "c2.txt"))
	os.MkdirAll(filepath.Join(root, "fresh", "deep"), 0o755)
	os.WriteFile(filepath.Join(root, "fresh", "deep", "e.txt"), []byte("z"), 0o644)
	os.RemoveAll(filepath.Join(root, "olddir"))

	changed, err := ix.CatchUp()
	if err != nil {
		t.Fatalf("catch-up failed: %v", err)
	}
	if changed == 0 {
		t.Fatal("catch-up saw no changed directories")
	}
	var got []string
	ix.mu.RLock()
	for d, names := range ix.dirs {
		for _, n := range names {
			rel, _ := filepath.Rel(root, filepath.Join(d, n))
			got = append(got, filepath.ToSlash(rel))
		}
	}
	ix.mu.RUnlock()
	sort.Strings(got)
	want := []string{"a.txt", "fresh", "fresh/deep", "fresh/deep/e.txt", "keep", "keep/b.txt", "keep/sub", "keep/sub/c2.txt", "new.txt"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("after catch-up:\n got %v\nwant %v", got, want)
	}
	if ix.Count() != len(want) {
		t.Fatalf("count = %d, want %d", ix.Count(), len(want))
	}
	if !strings.Contains(ix.CatchUpNote(), "caught up from the change journal") {
		t.Fatalf("note = %q", ix.CatchUpNote())
	}
	// A second catch-up right away finds nothing to do.
	if n, err := ix.CatchUp(); err != nil || n != 0 {
		t.Fatalf("second catch-up: n=%d err=%v", n, err)
	}
	// A journal from another life (different ID) is unusable: the caller rebuilds.
	ix.mu.Lock()
	for v, p := range ix.journals {
		p.ID++
		ix.journals[v] = p
	}
	ix.mu.Unlock()
	if _, err := ix.CatchUp(); err == nil {
		t.Fatal("a recreated journal should be reported as unusable")
	}
}
