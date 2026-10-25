package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Tab in the console view completes the path at the end of the command
// line: a unique match is filled in, an ambiguous one is listed on the
// second Tab, and nothing happens when nothing matches.
func TestTabCompletesPathsInConsoleView(t *testing.T) {
	a, data := testApp(t)
	key(a, "Ctrl+O")
	if !a.hideAll {
		t.Fatal("console view did not open")
	}
	// Absolute path, unique: completed.
	typeText(a, "cd "+filepath.Join(data, "al"))
	key(a, "Tab")
	if got, want := a.cmd.String(), "cd "+filepath.Join(data, "alpha"); got != want {
		t.Fatalf("absolute: %q, want %q", got, want)
	}
	// Relative to the active panel, case-insensitively on Windows.
	key(a, "Esc")
	typeText(a, "cd gam")
	key(a, "Tab")
	if a.cmd.String() != "cd gamma" {
		t.Fatalf("relative: %q", a.cmd.String())
	}
	// Ambiguous: the first Tab leaves the line alone, the second lists.
	key(a, "Esc")
	typeText(a, "type file")
	key(a, "Tab")
	if a.cmd.String() != "type file" {
		t.Fatalf("ambiguous first Tab changed the line: %q", a.cmd.String())
	}
	a.draw()
	if strings.Contains(screenText(a), "file10.txt") {
		t.Fatal("first Tab should not list")
	}
	key(a, "Tab")
	a.draw()
	s := screenText(a)
	for _, n := range []string{"file1.txt", "file10.txt", "file2.txt"} {
		if !strings.Contains(s, n) {
			t.Fatalf("second Tab did not list %s:\n%s", n, s)
		}
	}
	if a.cmd.String() != "type file" {
		t.Fatalf("listing changed the line: %q", a.cmd.String())
	}
	// Nothing matches: nothing happens.
	key(a, "Esc")
	typeText(a, "cd zzz")
	key(a, "Tab")
	if a.cmd.String() != "cd zzz" {
		t.Fatalf("no match: %q", a.cmd.String())
	}
	// With the panels visible Tab still switches panels.
	key(a, "Ctrl+O")
	key(a, "Esc")
	typeText(a, "cd al")
	key(a, "Tab")
	if a.cmd.String() != "cd al" || a.active != 1 {
		t.Fatalf("panels visible: line %q active %d", a.cmd.String(), a.active)
	}
}
