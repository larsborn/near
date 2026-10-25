package main

import (
	"os"
	"strings"
	"testing"
)

// F2 opens the user menu; Ins, F4, Ctrl+Down and Del edit it like FAR, and
// the result is written to menus.toml, which loadConfig prefers.
func TestUserMenuEditing(t *testing.T) {
	a, _ := testApp(t)
	key(a, "F2")
	d, ok := a.top().(*ListDialog)
	if !ok {
		t.Fatalf("F2 should open a menu, got %T", a.top())
	}
	n := len(d.Items)
	if !strings.Contains(screenText(a), "Ins Del F4 Alt+F4 Ctrl+Up Ctrl+Down") {
		t.Fatalf("menu should show the editing keys:\n%s", screenText(a))
	}

	// Ins -> Insert command -> fill in the dialog -> Ctrl+Enter.
	key(a, "Insert")
	if _, ok := a.top().(*ConfirmDialog); !ok || !strings.Contains(screenText(a), "Do you wish to insert a new command or a new menu?") {
		t.Fatalf("Ins should ask command or menu:\n%s", screenText(a))
	}
	key(a, "Enter")
	if _, ok := a.top().(*MenuItemDialog); !ok {
		t.Fatalf("expected the Edit user menu dialog, got %T", a.top())
	}
	s := screenText(a)
	for _, want := range []string{"Edit user menu", "Hot key:", "Label:", "Commands:", "{ OK }", "[ Cancel ]"} {
		if !strings.Contains(s, want) {
			t.Fatalf("dialog is missing %q:\n%s", want, s)
		}
	}
	typeText(a, "x")
	key(a, "Tab")
	typeText(a, "Say hello")
	key(a, "Tab")
	typeText(a, "echo hello")
	key(a, "Enter") // a new command line, not OK
	typeText(a, "echo again")
	key(a, "Ctrl+Enter")
	d, ok = a.top().(*ListDialog)
	if !ok || len(d.Items) != n+1 || d.Items[0].Label != "Say hello" || d.Cursor != 0 {
		t.Fatalf("after insert: %T items=%d first=%q cursor=%d", a.top(), len(d.Items), d.Items[0].Label, d.Cursor)
	}
	m := a.cfg.menu("Quick launch")
	if it := m.Items[0]; it.Hotkey != "x" || it.Command != "echo hello\necho again" {
		t.Fatalf("inserted item = %+v", it)
	}
	if _, err := os.Stat(a.menusFile()); err != nil {
		t.Fatal("menus.toml should have been written")
	}
	cfg2, err := loadConfig(a.cfg.path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Menus[0].Items[0].Title != "Say hello" || cfg2.Menus[0].Items[0].Command != "echo hello\necho again" {
		t.Fatalf("menus.toml did not round-trip: %+v", cfg2.Menus[0].Items[0])
	}

	// F4 edits; Enter in the label accepts.
	key(a, "F4")
	key(a, "Tab")
	key(a, "End")
	typeText(a, "!")
	key(a, "Enter")
	if m.Items[0].Title != "Say hello!" {
		t.Fatalf("after F4: %q", m.Items[0].Title)
	}
	// Ctrl+Down moves the item and keeps the cursor on it.
	key(a, "Ctrl+Down")
	d = a.top().(*ListDialog)
	if m.Items[1].Title != "Say hello!" || d.Items[d.Cursor].Label != "Say hello!" {
		t.Fatalf("after Ctrl+Down: items[1]=%q cursor on %q", m.Items[1].Title, d.Items[d.Cursor].Label)
	}
	// Del asks, then removes.
	key(a, "Delete")
	if !strings.Contains(screenText(a), `Delete menu item "Say hello!"?`) {
		t.Fatalf("Del should ask:\n%s", screenText(a))
	}
	key(a, "Enter")
	if len(m.Items) != n {
		t.Fatalf("after Del: %d items", len(m.Items))
	}
	// Ins -> Insert menu creates a submenu named after the label, at the cursor.
	key(a, "Home")
	key(a, "Insert")
	key(a, "Right")
	key(a, "Enter")
	typeText(a, "s")
	key(a, "Tab")
	typeText(a, "Extras")
	key(a, "Enter")
	if m.Items[0].Submenu != "Extras" || a.cfg.menu("Extras") == nil {
		t.Fatalf("submenu not created: %+v", m.Items[0])
	}
	key(a, "Enter") // opens the (empty) submenu
	if sub, ok := a.top().(*ListDialog); !ok || sub.Title != "Extras" {
		t.Fatalf("Enter should open the submenu, got %T", a.top())
	}
}

// A command naming a file runs once per selected entry; FAR's symbols are
// inserted as they are, several lines run one after another.
func TestMenuCommandRunsPerSelection(t *testing.T) {
	a, _ := testApp(t)
	tab := a.tab()
	tab.Selected["file1.txt"], tab.Selected["file2.txt"] = true, true
	tab.SetCursorOn("file1.txt")
	sep := commandSeparator()
	got := a.menuCommandLine(MenuItem{Command: `pdflatex "!.!"`}, tab.Targets(), "")
	if want := `pdflatex "file1.txt"` + sep + `pdflatex "file2.txt"`; got != want {
		t.Fatalf("per file: %q, want %q", got, want)
	}
	got = a.menuCommandLine(MenuItem{Command: "git status\necho !&"}, tab.Targets(), "")
	if want := "git status" + sep + "echo " + shQuote("file1.txt") + " " + shQuote("file2.txt"); got != want {
		t.Fatalf("once: %q, want %q", got, want)
	}
	got = a.menuCommandLine(MenuItem{Command: "cp !.tex {other}"}, tab.Targets(), "")
	if !strings.HasPrefix(got, "cp file1.tex "+shQuote(a.other().Path)) || !strings.Contains(got, sep+"cp file2.tex ") {
		t.Fatalf("mixed symbols: %q", got)
	}
	for cmd, want := range map[string]bool{"!.!": true, "!": true, "x !~": true, "{file}": true, "echo !&": false, "echo !!": false, "cd !\\": false, "!?Name?!": false, "git status": false} {
		if usesFileSymbols(cmd) != want {
			t.Fatalf("usesFileSymbols(%q) = %v", cmd, !want)
		}
	}
}
