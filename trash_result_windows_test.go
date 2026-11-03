//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The shell can report success and leave entries behind, so trashPaths
// checks the result itself; stillThere is what it checks with.
func TestTrashReportsWhatSurvived(t *testing.T) {
	dir := t.TempDir()
	gone := filepath.Join(dir, "gone")
	kept := filepath.Join(dir, "kept")
	for _, p := range []string{gone, kept} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if left := stillThere([]string{gone, kept}); len(left) != 2 {
		t.Fatalf("both exist, got %v", left)
	}
	os.Remove(gone)
	left := stillThere([]string{gone, kept})
	if len(left) != 1 || left[0] != "kept" {
		t.Fatalf("stillThere = %v, want [kept]", left)
	}
	// A real delete leaves nothing, so no error is reported.
	if err := trashPaths([]string{kept}); err != nil {
		t.Fatalf("trashPaths on a plain directory: %v", err)
	}
	if _, err := os.Stat(kept); err == nil {
		t.Fatal("the directory should be gone")
	}
	// A path the shell cannot remove is named in the error. A directory
	// held open by this process is one: Windows refuses to delete it.
	held := filepath.Join(dir, "held")
	if err := os.Mkdir(held, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(held)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	err = trashPaths([]string{held})
	if _, statErr := os.Stat(held); statErr != nil {
		t.Skip("the shell removed the open directory; nothing to report")
	}
	// Whether the shell admits the failure itself or stays silent, what
	// must never happen is trashPaths reporting success for an entry that
	// is still on disk.
	if err == nil {
		t.Fatal("an entry that survived must not be reported as deleted")
	}
	t.Logf("surviving entry reported as: %v", err)
}
