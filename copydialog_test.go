package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCopyDialogLooksLikeFar(t *testing.T) {
	a, data := testApp(t)
	a.tab().SetCursorOn("photo.jpg")
	key(a, "F5")
	d, ok := a.top().(*CopyMoveDialog)
	if !ok {
		t.Fatalf("expected the copy dialog, got %T", a.top())
	}
	txt := screenText(a)
	for _, want := range []string{"╔", " Copy ", "Copy photo.jpg to", "Already existing files:", "Ask", "{ Copy }", "[ Cancel ]", "Preserve timestamps"} {
		if !strings.Contains(txt, want) {
			t.Fatalf("missing %q in\n%s", want, txt)
		}
	}
	if !strings.Contains(txt, a.other().Path) && !strings.Contains(txt, "…") {
		t.Fatal("destination should be prefilled with the other panel's directory")
	}
	// Tab reaches the policy, Right cycles it, Esc cancels.
	key(a, "Tab")
	key(a, "Right")
	if existingPolicies[d.policy] != "Overwrite" {
		t.Fatalf("policy = %s", existingPolicies[d.policy])
	}
	key(a, "Esc")
	if len(a.dialogs) != 0 {
		t.Fatal("Esc should close the dialog")
	}
	// Move dialog wording.
	key(a, "F6")
	if !strings.Contains(screenText(a), "Rename or move photo.jpg to") || !strings.Contains(screenText(a), "{ Rename }") {
		t.Fatalf("move dialog:\n%s", screenText(a))
	}
	key(a, "Esc")
	// Several entries are described like FAR does.
	a.tab().Selected["alpha"] = true
	a.tab().Selected["photo.jpg"] = true
	a.tab().Selected["file1.txt"] = true
	key(a, "F5")
	if !strings.Contains(screenText(a), "Copy 2 files and 1 folder to") {
		t.Fatalf("multi wording:\n%s", screenText(a))
	}
	key(a, "Esc")
	_ = data
}

func TestCopyPolicies(t *testing.T) {
	a, data := testApp(t)
	other := filepath.Join(filepath.Dir(data), "dest")
	os.MkdirAll(other, 0o755)
	os.WriteFile(filepath.Join(other, "photo.jpg"), []byte("OLD"), 0o644)
	os.WriteFile(filepath.Join(other, "file1.txt"), []byte("OLD"), 0o644)
	a.navigate(a.other(), other, "")
	a.tab().Selected["photo.jpg"] = true
	a.tab().Selected["file1.txt"] = true

	// Skip: nothing overwritten, no question asked.
	key(a, "F5")
	key(a, "Tab")
	key(a, "Right")
	key(a, "Right") // Skip
	key(a, "Enter")
	if len(a.dialogs) != 0 {
		if _, ok := a.top().(*ConfirmDialog); ok {
			t.Fatal("Skip must not ask")
		}
	}
	waitTransfers(t, a)
	if b, _ := os.ReadFile(filepath.Join(other, "photo.jpg")); string(b) != "OLD" {
		t.Fatal("Skip overwrote a file")
	}
	// Only newer: the source is newer than one target only.
	past := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(other, "photo.jpg"), past, past)
	a.tab().Selected["photo.jpg"] = true
	a.tab().Selected["file1.txt"] = true
	key(a, "F5")
	key(a, "Tab")
	key(a, "Left") // Only newer (wraps backwards from Ask)
	key(a, "Enter")
	waitTransfers(t, a)
	if b, _ := os.ReadFile(filepath.Join(other, "photo.jpg")); string(b) == "OLD" {
		t.Fatal("Only newer should have overwritten the older target")
	}
	// Overwrite: everything replaced without asking.
	a.tab().Selected["file1.txt"] = true
	key(a, "F5")
	key(a, "Tab")
	key(a, "Right") // Overwrite
	key(a, "Enter")
	waitTransfers(t, a)
	if b, _ := os.ReadFile(filepath.Join(other, "file1.txt")); string(b) == "OLD" {
		t.Fatal("Overwrite should have replaced the file")
	}
	// Ask: with a conflict, the question appears.
	a.tab().Selected["file1.txt"] = true
	key(a, "F5")
	key(a, "Enter")
	if _, ok := a.top().(*ConfirmDialog); !ok {
		t.Fatalf("Ask should confirm on conflicts, got %T", a.top())
	}
	key(a, "Esc")
}

func TestShiftUpDownSelect(t *testing.T) {
	a, _ := testApp(t)
	a.tab().SetCursorOn("alpha")
	key(a, "Shift+Down")
	key(a, "Shift+Down")
	if !a.tab().Selected["alpha"] || !a.tab().Selected["beta"] || a.tab().Current().Name != "gamma" {
		t.Fatalf("Shift+Down: selected=%v cursor=%s", a.tab().Selected, a.tab().Current().Name)
	}
	key(a, "Shift+Up")
	if !a.tab().Selected["gamma"] || a.tab().Current().Name != "beta" {
		t.Fatalf("Shift+Up: selected=%v cursor=%s", a.tab().Selected, a.tab().Current().Name)
	}
	key(a, "Shift+Up") // toggles beta off again
	if a.tab().Selected["beta"] {
		t.Fatal("Shift+Up should toggle the selection off")
	}
}
