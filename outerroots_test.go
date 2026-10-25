package main

import (
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
