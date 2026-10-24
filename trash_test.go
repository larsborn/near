package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTrashPaths(t *testing.T) {
	if runtime.GOOS != "windows" {
		// Use a private FreeDesktop trash so the test does not touch the real one.
		t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "xdg"))
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "near-trash-test.txt")
	d := filepath.Join(dir, "near-trash-dir")
	os.WriteFile(f, []byte("bye"), 0o644)
	os.MkdirAll(filepath.Join(d, "sub"), 0o755)
	os.WriteFile(filepath.Join(d, "sub", "x"), []byte("x"), 0o644)
	if err := trashPaths([]string{f, d}); err != nil {
		if runtime.GOOS != "windows" && strings.Contains(err.Error(), "cross-device") {
			t.Skip("temp dir on another filesystem than the test trash")
		}
		t.Fatal(err)
	}
	for _, p := range []string{f, d} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Fatalf("%s should be gone", p)
		}
	}
	if runtime.GOOS == "linux" {
		info := filepath.Join(os.Getenv("XDG_DATA_HOME"), "Trash", "info", "near-trash-test.txt.trashinfo")
		data, err := os.ReadFile(info)
		if err != nil || !strings.Contains(string(data), "[Trash Info]") || !strings.Contains(string(data), "DeletionDate=") {
			t.Fatalf("trashinfo missing or wrong: %v %q", err, data)
		}
		if _, err := os.Stat(filepath.Join(os.Getenv("XDG_DATA_HOME"), "Trash", "files", "near-trash-dir", "sub", "x")); err != nil {
			t.Fatal("directory tree should be in the trash")
		}
	}
}

func TestDeleteKeysTrashAndPermanent(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "xdg"))
	}
	a, data := testApp(t)
	// Delete asks to move to the Recycle Bin / trash.
	a.tab().SetCursorOn("file2.txt")
	key(a, "Delete")
	d, ok := a.top().(*ConfirmDialog)
	if !ok || !strings.Contains(d.Title, trashName) {
		t.Fatalf("expected the trash confirmation, got %T %+v", a.top(), d)
	}
	key(a, "Enter")
	if _, err := os.Stat(filepath.Join(data, "file2.txt")); !os.IsNotExist(err) {
		if a.statusErr {
			t.Skip("trash not available here: " + a.status)
		}
		t.Fatal("file2.txt should have been moved to the trash")
	}
	if !strings.Contains(a.status, trashName) {
		t.Fatalf("status = %q", a.status)
	}
	// Shift+Delete asks to delete permanently.
	a.tab().SetCursorOn("file1.txt")
	key(a, "Shift+Delete")
	d, ok = a.top().(*ConfirmDialog)
	if !ok || !strings.Contains(d.Title, "permanently") {
		t.Fatalf("expected the permanent confirmation, got %T %+v", a.top(), d)
	}
	key(a, "Enter")
	if _, err := os.Stat(filepath.Join(data, "file1.txt")); !os.IsNotExist(err) {
		t.Fatal("file1.txt should be deleted")
	}
	if !strings.Contains(a.status, "Deleted 1") {
		t.Fatalf("status = %q", a.status)
	}
}

func TestShiftHomeEndSelect(t *testing.T) {
	a, _ := testApp(t)
	a.tab().SetCursorOn("gamma")
	key(a, "Shift+End")
	sel := a.tab().Selected
	for _, n := range []string{"gamma", "file1.txt", "file2.txt", "file10.txt", "photo.jpg"} {
		if !sel[n] {
			t.Fatalf("Shift+End should select %s: %v", n, sel)
		}
	}
	if sel["alpha"] || a.tab().Current().Name != "photo.jpg" {
		t.Fatalf("Shift+End: alpha selected=%v cursor=%s", sel["alpha"], a.tab().Current().Name)
	}
	// From a selected entry Shift+Home unselects the range up to the top.
	key(a, "Shift+Home")
	if len(a.tab().Selected) != 0 || a.tab().Cursor != 0 {
		t.Fatalf("Shift+Home from a selected entry should unselect: %v cursor=%d", a.tab().Selected, a.tab().Cursor)
	}
	key(a, "Shift+PgDn")
	if len(a.tab().Selected) == 0 {
		t.Fatal("Shift+PgDn should select a page")
	}
}
