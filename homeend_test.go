package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Home and End must move the caret in every box you can type in. They are
// easy to lose: a dialog that also shows a list wants them for the list,
// and the panel binds them to first/last entry. The check is the same
// everywhere — put the caret at the front, type, and the text must start
// with what was typed; then the end, and it must finish with it.
func TestHomeAndEndWorkInEveryTextBox(t *testing.T) {
	// Each case opens a box, then names the LineEdit it is typing into.
	cases := []struct {
		name string
		open func(a *App) *LineEdit
	}{
		{"command line", func(a *App) *LineEdit {
			return a.cmd
		}},
		{"input dialog (F7 make directory)", func(a *App) *LineEdit {
			key(a, "F7")
			return a.top().(*InputDialog).Edit
		}},
		{"search dialog (Ctrl+F)", func(a *App) *LineEdit {
			key(a, "Ctrl+F")
			return a.top().(*SearchDialog).edit
		}},
		{"find file dialog (Alt+F7)", func(a *App) *LineEdit {
			key(a, "Alt+F7")
			return a.top().(*FindDialog).masks
		}},
		{"copy dialog (F5)", func(a *App) *LineEdit {
			a.tab().SetCursorOn("file1.txt")
			key(a, "F5")
			return a.top().(*CopyMoveDialog).dest
		}},
		{"apply command (Ctrl+G)", func(a *App) *LineEdit {
			a.tab().SetCursorOn("file1.txt")
			key(a, "Ctrl+G")
			return a.top().(*ApplyDialog).edit
		}},
		{"mass rename (Ctrl+F6)", func(a *App) *LineEdit {
			a.tab().SetCursorOn("file1.txt")
			key(a, "Ctrl+F6")
			return a.top().(*MassRenameDialog).field()
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, _ := testApp(t)
			e := c.open(a)
			e.Set("middle")

			key(a, "Home")
			typeText(a, "<")
			if got := e.String(); got != "<middle" {
				t.Fatalf("after Home and typing: %q, want %q", got, "<middle")
			}
			key(a, "End")
			typeText(a, ">")
			if got := e.String(); got != "<middle>" {
				t.Fatalf("after End and typing: %q, want %q", got, "<middle>")
			}
		})
	}
}

// The two multi-line editors go to the start and end of the current line,
// not of the whole text.
func TestHomeAndEndInMultiLineEditors(t *testing.T) {
	t.Run("built-in editor (F4)", func(t *testing.T) {
		a, data := testApp(t)
		if err := os.WriteFile(filepath.Join(data, "two.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		a.reloadAll()
		a.tab().SetCursorOn("two.txt")
		key(a, "F4")
		e := a.top().(*Editor)
		key(a, "Down") // the second line
		key(a, "End")
		typeText(a, "!")
		key(a, "Home")
		typeText(a, "<")
		if got := string(e.lines[1]); got != "<two!" {
			t.Fatalf("second line is %q, want %q", got, "<two!")
		}
		if got := string(e.lines[0]); got != "one" {
			t.Fatalf("the first line changed to %q", got)
		}
	})

	t.Run("notes text area (Ctrl+N)", func(t *testing.T) {
		a, _ := testApp(t)
		key(a, "Down") // alpha
		key(a, "Ctrl+N")
		d := a.top().(*TextDialog)
		typeText(a, "one")
		key(a, "Enter")
		typeText(a, "two")
		key(a, "End")
		typeText(a, "!")
		key(a, "Home")
		typeText(a, "<")
		if got := string(d.Lines[1]); got != "<two!" {
			t.Fatalf("second line is %q, want %q", got, "<two!")
		}
		if got := string(d.Lines[0]); got != "one" {
			t.Fatalf("the first line changed to %q", got)
		}
	})
}

// The search dialog shows a result list as well as the query, so the list
// keeps first/last on Ctrl+Home and Ctrl+End.
func TestSearchDialogListJumpsWithCtrl(t *testing.T) {
	a, _ := testApp(t)
	a.index.Configure(a.cfg, nil)
	a.index.Build()
	key(a, "Ctrl+F")
	typeText(a, "file")
	waitSearch(t, a)
	d := a.top().(*SearchDialog)
	if len(d.results) < 2 {
		t.Fatalf("need at least two results to move between, got %d", len(d.results))
	}
	key(a, "Ctrl+End")
	if d.cursor != len(d.results)-1 {
		t.Errorf("Ctrl+End put the cursor at %d, want %d", d.cursor, len(d.results)-1)
	}
	key(a, "Ctrl+Home")
	if d.cursor != 0 {
		t.Errorf("Ctrl+Home put the cursor at %d, want 0", d.cursor)
	}
	// And they must not have gone into the query.
	if d.edit.String() != "file" {
		t.Errorf("the query became %q", d.edit.String())
	}
}
