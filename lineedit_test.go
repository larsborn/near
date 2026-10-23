package main

import (
	"testing"
)

func TestLineEditSelection(t *testing.T) {
	a, _ := testApp(t)
	e := newLineEdit("alpha beta gamma")
	press := func(name string) { e.HandleKey(eventFor(name), a) }

	// Ctrl+Shift+Left selects the word before the cursor.
	press("Ctrl+Shift+Left")
	if e.SelectedText() != "gamma" {
		t.Fatalf("Ctrl+Shift+Left selected %q", e.SelectedText())
	}
	press("Ctrl+Shift+Left")
	if e.SelectedText() != "beta gamma" {
		t.Fatalf("second Ctrl+Shift+Left selected %q", e.SelectedText())
	}
	// Ctrl+Shift+Right shrinks it again; plain Right drops the selection.
	press("Ctrl+Shift+Right")
	if e.SelectedText() != "gamma" {
		t.Fatalf("Ctrl+Shift+Right: %q", e.SelectedText())
	}
	press("Right")
	if _, _, ok := e.Selection(); ok || e.Cur != len(e.Text) {
		t.Fatalf("Right should drop the selection and move to its end, cur=%d", e.Cur)
	}
	// Shift+Home selects to the start; typing replaces the selection.
	press("Shift+Home")
	if e.SelectedText() != "alpha beta gamma" {
		t.Fatalf("Shift+Home: %q", e.SelectedText())
	}
	press("x")
	if e.String() != "x" {
		t.Fatalf("typing over a selection: %q", e.String())
	}
	// Shift+Left/Right, copy, cut and paste.
	e.Set("hello world")
	press("Home")
	press("Shift+Right")
	press("Shift+Right")
	press("Shift+Right")
	press("Shift+Right")
	press("Shift+Right")
	if e.SelectedText() != "hello" {
		t.Fatalf("Shift+Right x5: %q", e.SelectedText())
	}
	press("Ctrl+C")
	if a.clipboard != "hello" {
		t.Fatalf("Ctrl+C copied %q", a.clipboard)
	}
	press("Ctrl+X")
	if e.String() != " world" {
		t.Fatalf("Ctrl+X left %q", e.String())
	}
	press("End")
	press("Ctrl+V")
	if e.String() != " worldhello" {
		t.Fatalf("Ctrl+V gave %q", e.String())
	}
	// Backspace with a selection removes just the selection; Ctrl+A selects all.
	press("Ctrl+Shift+Left")
	press("Backspace")
	if e.String() != " " {
		t.Fatalf("Backspace over selection: %q", e.String())
	}
	e.Set("abc")
	press("Ctrl+A")
	if e.SelectedText() != "abc" {
		t.Fatalf("Ctrl+A: %q", e.SelectedText())
	}
	// Selection is drawn in reverse video.
	d := a.input("t", "p:", "sel", nil)
	d.Edit.HandleKey(eventFor("Shift+Home"), a)
	a.draw()
}

func TestCommandLineSelection(t *testing.T) {
	a, _ := testApp(t)
	typeText(a, "git commit -m message")
	key(a, "Ctrl+Shift+Left")
	if a.cmd.SelectedText() != "message" {
		t.Fatalf("command line selection: %q", a.cmd.SelectedText())
	}
	typeText(a, "fix")
	if a.cmd.String() != "git commit -m fix" {
		t.Fatalf("typing over the selection: %q", a.cmd.String())
	}
	key(a, "Shift+Home")
	key(a, "Ctrl+C")
	if a.clipboard != "git commit -m fix" {
		t.Fatalf("copied %q", a.clipboard)
	}
}
