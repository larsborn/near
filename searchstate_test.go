package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// The search dialog says what state it is in while the list is empty: why
// the index holds nothing, that a query has no matches, what can be typed.
func TestSearchDialogExplainsItsState(t *testing.T) {
	a, _ := testApp(t)
	a.cfg.SearchRoots = []string{filepath.Join(a.tab().Path, "definitely-not-there")}
	a.index.Configure(a.cfg, nil)
	a.index.Build()
	key(a, "Ctrl+F")
	// showSearch starts a rebuild of an empty index; wait for it.
	waitFor(t, "rebuild", func() bool { return !a.index.Building() })
	a.draw()
	if s := screenText(a); !strings.Contains(s, "Indexing failed:") || !strings.Contains(s, "and press Ctrl+R to rebuild the index.") {
		t.Fatalf("missing failure explanation:\n%s", s)
	}
	key(a, "Esc")

	a.cfg.SearchRoots = []string{a.tab().Path}
	a.index.Configure(a.cfg, nil)
	a.index.Build()
	key(a, "Ctrl+F")
	a.draw()
	// The path may be clipped on narrow screens or with CI's long temp paths.
	if s := screenText(a); !strings.Contains(s, "Type to search 9 names in ") {
		t.Fatalf("missing prompt:\n%s", s)
	}
	typeText(a, "nothing-like-this")
	waitSearch(t, a)
	a.draw()
	if s := screenText(a); !strings.Contains(s, `No matches for "nothing-like-this".`) {
		t.Fatalf("missing no-matches line:\n%s", s)
	}
}
