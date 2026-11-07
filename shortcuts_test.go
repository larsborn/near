package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Ctrl+D lists the folder shortcuts, numbered. Ins adds the panel's
// directory, Del removes one, a number goes there, and the list is kept in
// shortcuts.toml so it survives a restart.
func TestFolderShortcuts(t *testing.T) {
	a, data := testApp(t)
	gamma := filepath.Join(data, "gamma")
	a.cfg.Bookmarks = nil // start from an empty list, not the default Home

	key(a, "Ctrl+D")
	d, ok := a.top().(*ListDialog)
	if !ok || d.Title != "Folder shortcuts" {
		t.Fatalf("Ctrl+D should open the shortcuts, got %T", a.top())
	}
	if !strings.Contains(screenText(a), "No shortcuts yet") {
		t.Fatalf("an empty list should say so:\n%s", screenText(a))
	}

	// Ins remembers the panel's directory.
	key(a, "Insert")
	if len(a.cfg.Bookmarks) != 1 || a.cfg.Bookmarks[0].Path != data {
		t.Fatalf("Ins should add %s, list is %+v", data, a.cfg.Bookmarks)
	}
	d = a.top().(*ListDialog)
	if d.Items[0].Hotkey != '1' {
		t.Fatalf("entries should be numbered, first hotkey is %q", d.Items[0].Hotkey)
	}
	a.draw()
	if s := screenText(a); !strings.Contains(s, "1   "+filepath.Base(data)) {
		t.Fatalf("the number should stand before the name:\n%s", s)
	}
	// Adding the same directory again does not duplicate it.
	key(a, "Insert")
	if len(a.cfg.Bookmarks) != 1 {
		t.Fatalf("a duplicate should not be added: %+v", a.cfg.Bookmarks)
	}

	// Add a second one from another directory.
	key(a, "Esc")
	if err := a.navigate(a.tab(), gamma, ""); err != nil {
		t.Fatal(err)
	}
	key(a, "Ctrl+D")
	key(a, "Insert")
	if len(a.cfg.Bookmarks) != 2 || a.cfg.Bookmarks[1].Name != "gamma" {
		t.Fatalf("second shortcut: %+v", a.cfg.Bookmarks)
	}

	// The written file is what a restart would read.
	marks, err := loadShortcutsFile(a.shortcutsFile())
	if err != nil {
		t.Fatalf("shortcuts.toml: %v", err)
	}
	if len(marks) != 2 || marks[1].Path != gamma {
		t.Fatalf("saved shortcuts: %+v", marks)
	}
	cfg2, err := loadConfig(a.cfg.path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg2.Bookmarks) != 2 || cfg2.Bookmarks[0].Path != data {
		t.Fatalf("the file should replace the config's bookmarks: %+v", cfg2.Bookmarks)
	}

	// A number goes to that shortcut.
	if err := a.navigate(a.tab(), data, ""); err != nil {
		t.Fatal(err)
	}
	key(a, "Ctrl+D")
	typeText(a, "2")
	if a.tab().Path != gamma {
		t.Fatalf("2 should have gone to %s, panel is in %s", gamma, a.tab().Path)
	}
	if len(a.dialogs) != 0 {
		t.Fatal("going to a shortcut should close the list")
	}

	// Del removes the one under the cursor.
	key(a, "Ctrl+D")
	d = a.top().(*ListDialog)
	d.Cursor = 0
	key(a, "Delete")
	if len(a.cfg.Bookmarks) != 1 || a.cfg.Bookmarks[0].Name != "gamma" {
		t.Fatalf("Del should have removed the first: %+v", a.cfg.Bookmarks)
	}
	if marks, _ := loadShortcutsFile(a.shortcutsFile()); len(marks) != 1 {
		t.Fatalf("the removal should be saved: %+v", marks)
	}
}

// A drive or file system root keeps its path as the label, since it has no
// useful base name.
func TestShortcutLabel(t *testing.T) {
	for path, want := range map[string]string{
		`C:\`:                   `C:\`,
		filepath.Join("a", "b"): "b",
	} {
		if got := shortcutLabel(path); got != want {
			t.Fatalf("shortcutLabel(%q) = %q, want %q", path, got, want)
		}
	}
}
