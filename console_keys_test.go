package main

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestConsoleViewKeys(t *testing.T) {
	a, data := testApp(t)
	typeText(a, "echo one")
	key(a, "Enter")
	waitCommand(t, a)
	typeText(a, "echo two")
	key(a, "Enter")
	waitCommand(t, a)
	key(a, "Ctrl+O") // the panels came back after the command; go to the console view
	if !a.hideAll {
		t.Fatal("expected the console view")
	}
	path := a.tab().Path
	cur := a.tab().Cursor

	// Enter on an empty line prints a new prompt and leaves the panel alone.
	before := strings.Count(a.console.Text(), ">")
	key(a, "Enter")
	if a.tab().Path != path || strings.Count(a.console.Text(), ">") != before+1 {
		t.Fatalf("empty Enter: path=%s console=%q", a.tab().Path, a.console.Text())
	}
	// Tab does not switch the invisible panels.
	key(a, "Tab")
	if a.active != 0 {
		t.Fatal("Tab must not switch panels in the console view")
	}
	// Up/Down walk the history instead of moving the panel cursor.
	key(a, "Up")
	if a.cmd.String() != "echo two" || a.tab().Cursor != cur {
		t.Fatalf("Up: cmd=%q cursor=%d", a.cmd.String(), a.tab().Cursor)
	}
	key(a, "Up")
	if a.cmd.String() != "echo one" {
		t.Fatalf("Up twice: %q", a.cmd.String())
	}
	key(a, "Down")
	key(a, "Down")
	if a.cmd.String() != "" {
		t.Fatalf("Down past the end: %q", a.cmd.String())
	}
	// The borrowed status row has the console background, no panel-coloured strip.
	a.draw()
	cells, w, _ := a.screen.(tcell.SimulationScreen).GetContents()
	row := a.h - bottomRows
	_, wantBg, _ := a.theme.Cmd.Decompose()
	for x := 0; x < w; x++ {
		if _, bg, _ := cells[row*w+x].Style.Decompose(); bg != wantBg {
			t.Fatalf("status row cell %d has background %v, want console background %v", x, bg, wantBg)
		}
	}
	_ = data
}
