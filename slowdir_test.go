package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func TestSlowDirectoryBoxAndCancel(t *testing.T) {
	a, data := testApp(t)
	orig := listDirFn
	defer func() { listDirFn = orig }()

	// A directory that takes a second to list: the box appears, Esc gives up
	// and the panel stays where it was.
	listDirFn = func(path string, showHidden bool, notesFile string) ([]Entry, error) {
		if filepath.Base(path) == "gamma" {
			time.Sleep(1200 * time.Millisecond)
		}
		return orig(path, showHidden, notesFile)
	}
	go func() {
		time.Sleep(600 * time.Millisecond)
		a.screen.(tcell.SimulationScreen).InjectKey(tcell.KeyEscape, 0, 0)
	}()
	start := time.Now()
	a.slowBoxShown = false
	a.tab().SetCursorOn("gamma")
	key(a, "Enter")
	if a.tab().Path != data {
		t.Fatalf("Esc should keep the previous directory, got %s", a.tab().Path)
	}
	if time.Since(start) > 1100*time.Millisecond {
		t.Fatal("Esc should abandon the read before it completes")
	}
	if !a.slowBoxShown {
		t.Fatal("the slow-read box never appeared")
	}
	if !strings.Contains(a.status, "abandoned") {
		t.Fatalf("status = %q", a.status)
	}

	// Without Esc the slow read completes and the panel switches.
	a.slowBoxShown = false
	a.tab().SetCursorOn("gamma")
	key(a, "Enter")
	if filepath.Base(a.tab().Path) != "gamma" || !a.slowBoxShown {
		t.Fatalf("slow read should show the box and complete: %s", a.tab().Path)
	}

	// Fast directories never show the box.
	listDirFn = orig
	a.slowBoxShown = false
	key(a, "Esc")
	if a.slowBoxShown || a.tab().Path != data {
		t.Fatal("box shown for a fast read")
	}
}
