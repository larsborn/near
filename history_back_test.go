package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Each panel remembers where it has been as a single path: Ctrl+Alt+Left
// steps back, Ctrl+Alt+Right forward, and opening another directory drops
// whatever lay ahead.
func TestPanelBackAndForward(t *testing.T) {
	a, data := testApp(t)
	gamma := filepath.Join(data, "gamma")
	inner := filepath.Join(gamma, "inner")
	alpha := filepath.Join(data, "alpha")

	if err := a.navigate(a.tab(), gamma, ""); err != nil {
		t.Fatal(err)
	}
	if err := a.navigate(a.tab(), inner, ""); err != nil {
		t.Fatal(err)
	}
	// data -> gamma -> inner, so back twice is data.
	key(a, "Ctrl+Alt+Left")
	if a.tab().Path != gamma {
		t.Fatalf("one back: %s, want %s", a.tab().Path, gamma)
	}
	key(a, "Ctrl+Alt+Left")
	if a.tab().Path != data {
		t.Fatalf("two back: %s, want %s", a.tab().Path, data)
	}
	key(a, "Ctrl+Alt+Left")
	if a.tab().Path != data {
		t.Fatalf("past the start: %s", a.tab().Path)
	}
	if a.status == "" {
		t.Fatal("going back past the start should say so")
	}
	// Forward again, all the way.
	key(a, "Ctrl+Alt+Right")
	if a.tab().Path != gamma {
		t.Fatalf("one forward: %s, want %s", a.tab().Path, gamma)
	}
	key(a, "Ctrl+Alt+Right")
	if a.tab().Path != inner {
		t.Fatalf("two forward: %s, want %s", a.tab().Path, inner)
	}
	key(a, "Ctrl+Alt+Right")
	if a.tab().Path != inner {
		t.Fatalf("past the end: %s", a.tab().Path)
	}

	// Going back and then somewhere else drops what lay ahead.
	key(a, "Ctrl+Alt+Left") // gamma
	if err := a.navigate(a.tab(), alpha, ""); err != nil {
		t.Fatal(err)
	}
	key(a, "Ctrl+Alt+Right")
	if a.tab().Path != alpha {
		t.Fatalf("forward should be gone, went to %s", a.tab().Path)
	}
	key(a, "Ctrl+Alt+Left")
	if a.tab().Path != gamma {
		t.Fatalf("back after the new branch: %s, want %s", a.tab().Path, gamma)
	}

	// The other panel keeps its own path.
	key(a, "Tab")
	key(a, "Ctrl+Alt+Left")
	if a.tab().Path == gamma {
		t.Fatal("the other panel must not follow this one's history")
	}

	// A directory that has gone away is dropped rather than retried.
	key(a, "Tab")
	t2 := a.tab()
	if err := a.navigate(t2, inner, ""); err != nil {
		t.Fatal(err)
	}
	if err := a.navigate(t2, data, ""); err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(inner)
	before := len(t2.hist)
	key(a, "Ctrl+Alt+Left")
	if len(t2.hist) >= before {
		t.Fatalf("a vanished directory should be dropped: %v", t2.hist)
	}
}
