package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// In the search dialog the panel's copy-path key copies the selected
// result, and with Shift only the directory it is in. A context menu
// offers the same things with their keys.
func TestSearchDialogCopyAndMenu(t *testing.T) {
	a, data := testApp(t)
	a.index.Configure(a.cfg, nil)
	a.index.Build()
	key(a, "Ctrl+F")
	typeText(a, "deep")
	waitSearch(t, a)
	d, ok := a.top().(*SearchDialog)
	if !ok || len(d.results) == 0 {
		t.Fatalf("expected results in the search dialog, got %T", a.top())
	}
	want := filepath.Join(data, "gamma", "inner", "deep.md")
	if d.current() != want {
		t.Fatalf("selected %q, want %q", d.current(), want)
	}

	key(a, "Ctrl+Alt+Insert")
	if a.clipboard != want {
		t.Fatalf("copy path put %q on the clipboard", a.clipboard)
	}
	a.draw()
	if !strings.Contains(screenText(a), "Copied the path") {
		t.Fatalf("the dialog should say what it copied:\n%s", screenText(a))
	}
	key(a, "Ctrl+Alt+Shift+Insert")
	if a.clipboard != filepath.Dir(want) {
		t.Fatalf("copy directory put %q on the clipboard", a.clipboard)
	}

	// The footer names the keys.
	a.draw()
	key(a, "Down") // clears the note
	a.draw()
	if s := screenText(a); !strings.Contains(s, "Shift+F10: menu") {
		t.Fatalf("the footer should point at the menu:\n%s", s)
	}

	// The context menu offers all four actions with their keys.
	key(a, "Shift+F10")
	m, ok := a.top().(*ListDialog)
	if !ok {
		t.Fatalf("expected the context menu, got %T", a.top())
	}
	labels := ""
	for _, it := range m.Items {
		labels += it.Label + "|" + it.Right + "|"
	}
	for _, s := range []string{"Go to the file|Enter", "Open in a new tab|Ctrl+T", "Copy the path|Ctrl+Alt+Insert", "Copy the directory|Ctrl+Alt+Shift+Insert"} {
		if !strings.Contains(labels, s) {
			t.Fatalf("the menu lacks %q: %s", s, labels)
		}
	}
	// Choosing "Copy the directory" does the same as the key.
	a.clipboard = ""
	for i, it := range m.Items {
		if it.Label == "Copy the directory" {
			m.Cursor = i
		}
	}
	key(a, "Enter")
	if a.clipboard != filepath.Dir(want) {
		t.Fatalf("the menu entry copied %q", a.clipboard)
	}

	// A right click selects a result and opens the menu on it.
	if _, ok := a.top().(*SearchDialog); !ok {
		t.Fatalf("back to the search dialog, got %T", a.top())
	}
	a.draw()
	a.handleEvent(tcell.NewEventMouse(d.box.x+3, d.listTop, tcell.ButtonSecondary, 0))
	if _, ok := a.top().(*ListDialog); !ok {
		t.Fatalf("a right click should open the menu, got %T", a.top())
	}
}

// The copy-path key is recognised through the keymap, so a custom binding
// works too, and Shift is what asks for the directory.
func TestCopyPathKeyLookup(t *testing.T) {
	a, _ := testApp(t)
	ok, dirOnly := a.copyPathKey("Ctrl+Alt+Insert")
	if !ok || dirOnly {
		t.Fatalf("plain key: ok=%v dirOnly=%v", ok, dirOnly)
	}
	ok, dirOnly = a.copyPathKey("Ctrl+Alt+Shift+Insert")
	if !ok || !dirOnly {
		t.Fatalf("with Shift: ok=%v dirOnly=%v", ok, dirOnly)
	}
	if ok, _ := a.copyPathKey("Ctrl+X"); ok {
		t.Fatal("an unrelated key must not count")
	}
	// A rebound copy_path is found as well.
	a.cfg.Keys["copy_path"] = []string{"Ctrl+Y"}
	a.keymap = buildKeymap(a.cfg)
	if ok, _ := a.copyPathKey("Ctrl+Y"); !ok {
		t.Fatal("a rebound key should be recognised")
	}
	if ok, dirOnly := a.copyPathKey("Ctrl+Shift+Y"); !ok || !dirOnly {
		t.Fatalf("Shift with a rebound key: ok=%v dirOnly=%v", ok, dirOnly)
	}
	if got := shiftKey("Ctrl+Alt+Insert"); got != "Ctrl+Alt+Shift+Insert" {
		t.Fatalf("shiftKey = %q", got)
	}
	if got := shiftKey("Ctrl+Shift+Y"); got != "Ctrl+Shift+Y" {
		t.Fatalf("shiftKey should leave an existing Shift alone: %q", got)
	}
}
