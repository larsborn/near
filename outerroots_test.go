package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Roots inside other roots (and duplicates) are dropped so nothing is
// indexed or watched twice.
func TestOuterRootsDropsNestedRoots(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "Users", "born")
	got := outerRoots([]string{home, root, filepath.Join(root, "Users"), root, filepath.Join(root, "other")})
	want := []string{root}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("outerRoots = %q, want %q", got, want)
	}
	// Unrelated roots are kept in order.
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	if got := outerRoots([]string{a, b}); !reflect.DeepEqual(got, []string{a, b}) {
		t.Fatalf("unrelated roots: %q", got)
	}
}

// "*" in search_roots expands to the fixed drives: existing directories,
// none inside another.
func TestStarExpandsToFixedDrives(t *testing.T) {
	roots := expandRoots([]string{"*"})
	if len(roots) == 0 {
		t.Fatal("no fixed drives found")
	}
	for _, r := range roots {
		if !isDir(r) {
			t.Fatalf("%q is not a directory", r)
		}
	}
	t.Logf("fixed drives: %q", roots)
	// Explicit roots inside a drive are redundant and dropped.
	home, _ := os.UserHomeDir()
	if got := expandRoots([]string{"~", "*"}); len(got) != len(roots) && isDir(home) {
		t.Fatalf("~ next to *: %q", got)
	}
}
