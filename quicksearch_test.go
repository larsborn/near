package main

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// The quick search only accepts characters that still name something in
// the panel: a letter that would leave no match is dropped, and a first
// letter that matches nothing does not start a search at all.
func TestQuickSearchRejectsCharactersWithoutMatches(t *testing.T) {
	a, _ := testApp(t)
	alt := func(r rune) { a.handleEvent(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModAlt)) }

	alt('p') // photo.jpg
	if !a.qsActive || string(a.qsText) != "p" {
		t.Fatalf("Alt+p should start the search: active=%v text=%q", a.qsActive, string(a.qsText))
	}
	if cur := a.tab().Current(); cur == nil || cur.Name != "photo.jpg" {
		t.Fatalf("cursor on %v, want photo.jpg", cur)
	}
	alt('h')
	if string(a.qsText) != "ph" {
		t.Fatalf("ph should be accepted, text=%q", string(a.qsText))
	}
	// Nothing is called "phz", so the z is dropped and the cursor stays.
	alt('z')
	if string(a.qsText) != "ph" {
		t.Fatalf("a character without matches should be dropped, text=%q", string(a.qsText))
	}
	if cur := a.tab().Current(); cur == nil || cur.Name != "photo.jpg" {
		t.Fatalf("the cursor should not have moved, it is on %v", cur)
	}
	if !a.qsActive {
		t.Fatal("the search should still be running")
	}
	// Backspace still works on what was accepted.
	key(a, "Backspace")
	if string(a.qsText) != "p" {
		t.Fatalf("after Backspace: %q", string(a.qsText))
	}

	// A first letter that matches nothing does not start a search.
	key(a, "Esc")
	a.tab().Cursor = 0
	alt('q')
	if a.qsActive || len(a.qsText) != 0 {
		t.Fatalf("Alt+q matches nothing and must not start a search: active=%v text=%q", a.qsActive, string(a.qsText))
	}
	if a.tab().Cursor != 0 {
		t.Fatalf("the cursor should not have moved, it is at %d", a.tab().Cursor)
	}
}
