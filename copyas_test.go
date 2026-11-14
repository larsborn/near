package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Shift+F5 is FAR's "copy the entry under the cursor to another name": it
// asks for a name, pre-filled with the current one, and copies beside the
// original rather than to the other panel.
func TestCopyAsAsksForANameAndCopiesBeside(t *testing.T) {
	a, data := testApp(t)
	a.tab().SetCursorOn("file1.txt")
	key(a, "Shift+F5")

	d, ok := a.top().(*InputDialog)
	if !ok {
		t.Fatalf("Shift+F5 should ask for a name, got %T", a.top())
	}
	if d.Edit.String() != "file1.txt" {
		t.Fatalf("the box starts with %q, want the current name", d.Edit.String())
	}
	d.Edit.Set("copy of file1.txt")
	key(a, "Enter")
	waitTransfers(t, a)

	got, err := os.ReadFile(filepath.Join(data, "copy of file1.txt"))
	if err != nil {
		t.Fatal("the copy was not made:", err)
	}
	if string(got) != "hello file1.txt" {
		t.Fatalf("the copy holds %q", got)
	}
	if _, err := os.Stat(filepath.Join(data, "file1.txt")); err != nil {
		t.Fatal("the original is gone:", err)
	}
}

// A path may be typed instead of a bare name, resolved against the entry's
// own directory, and a directory is copied whole.
func TestCopyAsTakesAPathAndCopiesTrees(t *testing.T) {
	a, data := testApp(t)
	a.tab().SetCursorOn("file1.txt")
	key(a, "Shift+F5")
	a.top().(*InputDialog).Edit.Set("alpha/moved.txt")
	key(a, "Enter")
	waitTransfers(t, a)
	if _, err := os.Stat(filepath.Join(data, "alpha", "moved.txt")); err != nil {
		t.Fatal("a relative path should resolve against the directory:", err)
	}

	a.tab().SetCursorOn("gamma")
	key(a, "Shift+F5")
	if d := a.top().(*InputDialog); d.Title != "Copy directory" {
		t.Errorf("a directory should say so, title is %q", d.Title)
	}
	a.top().(*InputDialog).Edit.Set("gamma-copy")
	key(a, "Enter")
	waitTransfers(t, a)
	if _, err := os.Stat(filepath.Join(data, "gamma-copy", "inner", "deep.md")); err != nil {
		t.Fatal("the tree was not copied:", err)
	}
}

// Copying an entry onto itself would truncate it, so it is refused. The
// name is also not allowed to be empty, and Esc does nothing.
func TestCopyAsRefusesItself(t *testing.T) {
	a, data := testApp(t)
	before, err := os.ReadFile(filepath.Join(data, "file1.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"file1.txt", "", "  "} {
		a.tab().SetCursorOn("file1.txt")
		key(a, "Shift+F5")
		a.top().(*InputDialog).Edit.Set(name)
		key(a, "Enter")
		waitTransfers(t, a)
		after, err := os.ReadFile(filepath.Join(data, "file1.txt"))
		if err != nil || string(after) != string(before) {
			t.Fatalf("name %q damaged the file: %q, %v", name, after, err)
		}
	}
	// Esc leaves everything alone too.
	a.tab().SetCursorOn("file1.txt")
	key(a, "Shift+F5")
	key(a, "Esc")
	if len(a.dialogs) != 0 {
		t.Fatalf("Esc should close the box, %d dialogs left", len(a.dialogs))
	}
}

// A name that is already taken is confirmed before it is overwritten, and
// an existing directory means "copy into it", as F5 does.
func TestCopyAsExistingDestination(t *testing.T) {
	a, data := testApp(t)
	a.tab().SetCursorOn("file1.txt")
	key(a, "Shift+F5")
	a.top().(*InputDialog).Edit.Set("file2.txt") // exists, with other contents
	key(a, "Enter")
	if _, ok := a.top().(*ConfirmDialog); !ok {
		t.Fatalf("an existing destination should be confirmed, got %T", a.top())
	}
	key(a, "Esc") // cancel: file2.txt keeps its own contents
	waitTransfers(t, a)
	if got, _ := os.ReadFile(filepath.Join(data, "file2.txt")); string(got) != "hello file2.txt" {
		t.Fatalf("cancelling still overwrote file2.txt: %q", got)
	}

	// Again, and this time overwrite.
	a.tab().SetCursorOn("file1.txt")
	key(a, "Shift+F5")
	a.top().(*InputDialog).Edit.Set("file2.txt")
	key(a, "Enter")
	key(a, "Enter") // "Overwrite" is the first button
	waitTransfers(t, a)
	if got, _ := os.ReadFile(filepath.Join(data, "file2.txt")); string(got) != "hello file1.txt" {
		t.Fatalf("file2.txt holds %q after overwriting", got)
	}

	// An existing directory takes the entry into it.
	a.tab().SetCursorOn("photo.jpg")
	key(a, "Shift+F5")
	a.top().(*InputDialog).Edit.Set("beta")
	key(a, "Enter")
	waitTransfers(t, a)
	if _, err := os.Stat(filepath.Join(data, "beta", "photo.jpg")); err != nil {
		t.Fatal("an existing directory should take the copy:", err)
	}
}

// It works on the cursor entry whatever is selected, like Shift+F6, and so
// it must not clear the selection the way F5 and F6 do.
func TestCopyAsLeavesTheSelectionAlone(t *testing.T) {
	a, data := testApp(t)
	a.tab().SetCursorOn("photo.jpg")
	key(a, "Insert") // select photo.jpg; the cursor moves on
	if len(a.tab().Selected) != 1 {
		t.Fatalf("expected one selected entry, got %d", len(a.tab().Selected))
	}
	a.tab().SetCursorOn("file1.txt")
	key(a, "Shift+F5")
	a.top().(*InputDialog).Edit.Set("one.txt")
	key(a, "Enter")
	waitTransfers(t, a)

	// The cursor entry was copied, not the selected one.
	if _, err := os.Stat(filepath.Join(data, "one.txt")); err != nil {
		t.Fatal("the cursor entry was not copied:", err)
	}
	if !a.tab().Selected["photo.jpg"] {
		t.Errorf("the selection was cleared: %v", a.tab().Selected)
	}
}
