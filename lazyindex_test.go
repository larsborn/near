package main

import "testing"

// By default the index costs nothing until it is searched: a session that
// never opens the search dialog never loads it, which is the difference
// between 17 MB and a gigabyte on a whole-machine index.
func TestIndexIsNotLoadedUntilSearched(t *testing.T) {
	a, _ := testApp(t) // sets search_index_on_start = false
	a.startIndex()
	if a.indexStarted || a.index.Building() || a.index.Loading() || a.index.Count() != 0 {
		t.Fatalf("start-up touched the index: started=%v building=%v loading=%v count=%d",
			a.indexStarted, a.index.Building(), a.index.Loading(), a.index.Count())
	}
	// Configure still ran, so the roots are known without reading anything.
	if len(a.index.Roots()) == 0 {
		t.Fatal("startIndex did not configure the roots")
	}

	a.showSearch()
	if !a.indexStarted {
		t.Fatal("opening the search dialog did not bring the index up")
	}
	waitFor(t, "the index to come up", func() bool { return a.index.Count() > 0 })

	// Opening it again must not start a second build.
	n := a.index.Count()
	a.pop()
	a.showSearch()
	if c := a.index.Count(); c != n {
		t.Fatalf("reopening rebuilt the index: %d names, was %d", c, n)
	}
}

// search_index_on_start = true buys an instant first search with that memory.
func TestIndexOnStartLoadsEagerly(t *testing.T) {
	a, _ := testApp(t)
	a.cfg.SearchIndexOnStart = true
	a.startIndex()
	if !a.indexStarted {
		t.Fatal("search_index_on_start = true did not bring the index up")
	}
	waitFor(t, "the index to come up", func() bool { return a.index.Count() > 0 })
}

// -reindex rebuilds rather than loading the cache, as its help text says.
func TestReindexFlagRebuilds(t *testing.T) {
	a, _ := testApp(t)
	a.cfg.reindex = true
	a.startIndex()
	if !a.indexStarted {
		t.Fatal("-reindex did not bring the index up")
	}
	waitFor(t, "the rebuild to finish", func() bool { return a.index.Count() > 0 })
}
