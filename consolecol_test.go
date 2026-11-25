package main

import (
	"fmt"
	"strings"
	"testing"
)

// consoleOut fills the console with numbered lines and opens the band, as
// running a command does.
func consoleOut(a *App, n int) {
	a.draw() // the band is sized from the screen, which the first draw learns
	for i := 0; i < n; i++ {
		a.console.Println(fmt.Sprintf("output line %02d", i), a.theme.Cmd)
	}
	a.ensureConsoleBand()
	a.draw()
}

// countRows is how many screen rows contain s.
func countRows(a *App, s string) int {
	n := 0
	for _, line := range strings.Split(screenText(a), "\n") {
		if strings.Contains(line, s) {
			n++
		}
	}
	return n
}

// columnOf is the column where s starts, or -1.
func columnOf(a *App, s string) int {
	for _, line := range strings.Split(screenText(a), "\n") {
		if c := strings.Index(line, s); c >= 0 {
			return c
		}
	}
	return -1
}

// Hiding one panel frees a column, and the console moves into it: the
// whole tail becomes readable instead of the few rows a band under the
// panels can show, which is what the freed space is good for.
func TestConsoleFillsTheColumnOfAHiddenPanel(t *testing.T) {
	const lines = 20
	a, _ := testApp(t)
	consoleOut(a, lines)

	band := countRows(a, "output line")
	if band == 0 || band >= lines {
		t.Fatalf("with both panels a band should show some but not all of %d lines, got %d", lines, band)
	}

	key(a, "Ctrl+P") // hides the inactive panel, which is the right one
	if !a.hidden[1] || a.hidden[0] {
		t.Fatalf("Ctrl+P should hide the right panel: hidden=%v", a.hidden)
	}
	a.draw()
	if n := countRows(a, "output line"); n != lines {
		t.Fatalf("the freed column should show all %d lines, got %d:\n%s", lines, n, screenText(a))
	}
	if col := columnOf(a, "output line 00"); col <= a.panelSplit() {
		t.Errorf("the output should be in the freed right column, it starts at column %d (split %d)",
			col, a.panelSplit())
	}
	// The remaining panel takes the whole height: a band under it would
	// only repeat the same tail.
	if h := a.panelRect[0].h; h != a.panelArea() {
		t.Errorf("the lone panel is %d rows, want the full %d", h, a.panelArea())
	}

	// Showing it again puts the console back under the panels.
	key(a, "Ctrl+P")
	a.draw()
	if n := countRows(a, "output line"); n != band {
		t.Errorf("back to two panels should show the band again: %d rows, want %d", n, band)
	}
}

// The same when it is the left panel that goes.
func TestConsoleFillsTheLeftColumn(t *testing.T) {
	a, _ := testApp(t)
	consoleOut(a, 20)
	key(a, "Ctrl+F1") // hide the left panel
	if !a.hidden[0] {
		t.Fatalf("Ctrl+F1 should hide the left panel: hidden=%v", a.hidden)
	}
	a.draw()
	col := columnOf(a, "output line 00")
	if col < 0 || col >= a.panelSplit() {
		t.Errorf("the output should be in the freed left column, it starts at column %d (split %d)",
			col, a.panelSplit())
	}
	if h := a.panelRect[1].h; h != a.panelArea() {
		t.Errorf("the lone panel is %d rows, want the full %d", h, a.panelArea())
	}
}

// With nothing to show the freed column stays empty rather than drawing an
// empty console, and hiding both panels is still the full-screen view.
func TestConsoleColumnOnlyWhenThereIsOutput(t *testing.T) {
	a, _ := testApp(t)
	key(a, "Ctrl+P")
	a.draw()
	if a.conArea.h != 0 {
		t.Errorf("no output, so no console area: %+v", a.conArea)
	}
	// Panels stay the height they were without a band to make room for.
	if h := a.panelRect[0].h; h != a.panelArea() {
		t.Errorf("the lone panel is %d rows, want the full %d", h, a.panelArea())
	}

	consoleOut(a, 5)
	a.hideAll = true
	a.draw()
	if a.conArea.x != 0 || a.conArea.w != a.w {
		t.Errorf("hiding everything should give the console the full width: %+v", a.conArea)
	}
}
