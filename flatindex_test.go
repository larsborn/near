package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// mapSearch is the slow reference scan (what Search does without a flat view).
func mapSearch(ix *Index, q string, max int) ([]string, int, bool) {
	ix.mu.Lock()
	f := ix.flat
	ix.flat = nil
	ix.mu.Unlock()
	out, total, capped := ix.Search(q, max)
	ix.mu.Lock()
	ix.flat = f
	ix.mu.Unlock()
	return out, total, capped
}

func TestFlatIndexMatchesMapScan(t *testing.T) {
	a, _ := testApp(t)
	a.index.Configure(a.cfg, nil)
	a.index.Build() // builds the flat view too
	if a.index.flat == nil {
		t.Fatal("flat view not built")
	}
	for _, q := range []string{"file", "FILE1", "deep", "inner/", "gamma/inner", "*.txt", "file *.txt", "photo jpg", "nothing-here", "a"} {
		fo, ft, fc := a.index.Search(q, 100)
		mo, mt, mc := mapSearch(a.index, q, 100)
		sort.Strings(fo)
		sort.Strings(mo)
		if strings.Join(fo, "|") != strings.Join(mo, "|") || ft != mt || fc != mc {
			t.Fatalf("query %q: flat=%v (%d,%v) map=%v (%d,%v)", q, fo, ft, fc, mo, mt, mc)
		}
	}
}

func TestFlatIndexCapAndStaleEntries(t *testing.T) {
	dirs := map[string][]string{}
	n := 0
	for d := 0; d < 30; d++ {
		dir := filepath.Join("/root", fmt.Sprintf("d%02d", d))
		for i := 0; i < 500; i++ {
			dirs[dir] = append(dirs[dir], fmt.Sprintf("File%d.txt", i))
			n++
		}
	}
	f := buildFlatIndex(dirs, n)
	if f.count != n {
		t.Fatalf("count = %d, want %d", f.count, n)
	}
	out, total, capped := f.search(parseTerms("file"), dirs, 5)
	if !capped || total != searchCountCap || len(out) != 5 {
		t.Fatalf("broad query: total=%d capped=%v results=%d", total, capped, len(out))
	}
	out, total, capped = f.search(parseTerms("file499"), dirs, 100)
	if capped || total != 30 || len(out) != 30 {
		t.Fatalf("narrow query: total=%d capped=%v results=%d", total, capped, len(out))
	}
	// Case-insensitive, path terms and globs go through the flat view too.
	if _, total, _ = f.search(parseTerms("FILE499 d07/"), dirs, 10); total != 1 {
		t.Fatalf("path term: total=%d", total)
	}
	if _, total, _ = f.search(parseTerms("file49?.txt"), dirs, 10); total != 300 {
		t.Fatalf("glob: total=%d", total)
	}
	// An entry removed from the map after the view was built is not reported.
	dirs[filepath.Join("/root", "d07")] = dirs[filepath.Join("/root", "d07")][:499]
	if _, total, _ = f.search(parseTerms("file499"), dirs, 100); total != 29 {
		t.Fatalf("stale entry still reported: total=%d", total)
	}
}
