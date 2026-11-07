package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// countingLister makes the app's directory reads countable and pretends
// every path is on a network drive.
func countingLister(t *testing.T, a *App, ttl time.Duration) *atomic.Int32 {
	t.Helper()
	var reads atomic.Int32
	real := listDirFn
	listDirFn = func(path string, showHidden bool, notesFile string) ([]Entry, error) {
		reads.Add(1)
		return real(path, showHidden, notesFile)
	}
	t.Cleanup(func() { listDirFn = real })
	a.dirs = newDirCache(ttl, func(string) bool { return true })
	return &reads
}

// A listing of a network directory is read once and reused while
// navigating; Ctrl+R and anything NEAR changes itself re-read it.
func TestNetworkListingsAreCachedAndRefreshed(t *testing.T) {
	a, data := testApp(t)
	reads := countingLister(t, a, time.Hour)

	a.tab().SetCursorOn("gamma")
	key(a, "Enter") // into gamma
	key(a, "Esc")   // back to data
	first := reads.Load()
	if first == 0 {
		t.Fatal("navigating should have read something")
	}
	key(a, "Enter") // into gamma again: cached
	key(a, "Esc")   // and back: cached
	if got := reads.Load(); got != first {
		t.Fatalf("navigation re-read the disk: %d reads, want %d", got, first)
	}
	if a.tab().CachedAt.IsZero() {
		t.Fatal("the panel should know its listing came from the cache")
	}
	a.draw()
	if !strings.Contains(screenText(a), "cached") {
		t.Fatalf("the panel should say the listing is cached:\n%s", screenText(a))
	}

	// Ctrl+R forgets everything and reads again.
	key(a, "Ctrl+R")
	after := reads.Load()
	if after <= first {
		t.Fatalf("Ctrl+R should re-read: %d reads, was %d", after, first)
	}
	if !a.tab().CachedAt.IsZero() {
		t.Fatal("after Ctrl+R the listing is fresh, not cached")
	}

	// A file made outside NEAR stays invisible until a reload; that is the
	// bargain of the cache.
	os.WriteFile(filepath.Join(data, "appeared.txt"), []byte("x"), 0o644)
	key(a, "Enter")
	key(a, "Esc")
	if strings.Contains(strings.Join(names(a.tab()), " "), "appeared.txt") {
		t.Fatal("a cached listing should not have picked the new file up")
	}
	key(a, "Ctrl+R")
	if !strings.Contains(strings.Join(names(a.tab()), " "), "appeared.txt") {
		t.Fatalf("Ctrl+R should show the new file: %v", names(a.tab()))
	}

	// NEAR's own changes re-read without being asked.
	before := reads.Load()
	a.reloadAll()
	if reads.Load() <= before {
		t.Fatal("reloadAll must go to the disk")
	}
}

// Hidden files are filtered from the cached listing, so switching them on
// needs no second read.
func TestHiddenFilesComeFromTheSameCachedListing(t *testing.T) {
	a, data := testApp(t)
	reads := countingLister(t, a, time.Hour)
	hidden := filepath.Join(data, ".secret")
	os.WriteFile(hidden, []byte("x"), 0o644)
	a.showHidden = false
	key(a, "Ctrl+R")
	if strings.Contains(strings.Join(names(a.tab()), " "), ".secret") {
		t.Fatalf("hidden entries should be filtered: %v", names(a.tab()))
	}
	before := reads.Load()
	a.showHidden = true
	a.loadTab(a.tab())
	if !strings.Contains(strings.Join(names(a.tab()), " "), ".secret") {
		t.Fatalf("hidden entries should appear: %v", names(a.tab()))
	}
	if reads.Load() != before {
		t.Fatal("showing hidden files should not need another read")
	}
}

// A listing older than the time to live is read again.
func TestCachedListingExpires(t *testing.T) {
	a, _ := testApp(t)
	reads := countingLister(t, a, 40*time.Millisecond)
	a.loadTab(a.tab())
	before := reads.Load()
	a.loadTab(a.tab())
	if reads.Load() != before {
		t.Fatal("a fresh listing should be reused")
	}
	time.Sleep(60 * time.Millisecond)
	a.loadTab(a.tab())
	if reads.Load() == before {
		t.Fatal("an expired listing should be read again")
	}
}

// Local directories are never cached, so nothing can go stale on them.
func TestLocalDirectoriesAreNotCached(t *testing.T) {
	a, _ := testApp(t)
	var reads atomic.Int32
	real := listDirFn
	listDirFn = func(path string, showHidden bool, notesFile string) ([]Entry, error) {
		reads.Add(1)
		return real(path, showHidden, notesFile)
	}
	t.Cleanup(func() { listDirFn = real })
	a.dirs = newDirCache(time.Hour, isNetworkPath) // the real test: a temp dir is local
	a.loadTab(a.tab())
	before := reads.Load()
	a.loadTab(a.tab())
	if reads.Load() <= before {
		t.Fatal("a local directory must be read every time")
	}
}
