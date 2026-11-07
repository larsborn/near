package main

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

// Listing a directory on a network drive can take seconds, and the same
// directories are walked over and over. Their listings are therefore kept
// for network_cache_minutes and reused while navigating. Anything that
// re-reads on purpose — Ctrl+R, and every change NEAR makes itself — goes
// to the disk and replaces what is cached, so stale entries never survive
// an operation.

// dirCacheMax is how many directory listings are kept; the oldest goes
// first. A few hundred listings is nothing next to the search index.
const dirCacheMax = 300

type dirCacheEntry struct {
	entries []Entry // every entry, hidden ones included
	notes   map[string]string
	at      time.Time
}

type dirCache struct {
	mu        sync.Mutex
	m         map[string]dirCacheEntry
	ttl       time.Duration
	cacheable func(string) bool // which paths are worth caching
}

func newDirCache(ttl time.Duration, cacheable func(string) bool) *dirCache {
	return &dirCache{m: map[string]dirCacheEntry{}, ttl: ttl, cacheable: cacheable}
}

func (c *dirCache) key(path string) string { return foldPath(filepath.Clean(path)) }

// on reports whether this path is cached at all.
func (c *dirCache) on(path string) bool {
	return c != nil && c.ttl > 0 && c.cacheable != nil && c.cacheable(path)
}

// get returns a listing that is still fresh.
func (c *dirCache) get(path string) (dirCacheEntry, bool) {
	if !c.on(path) {
		return dirCacheEntry{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[c.key(path)]
	if !ok || time.Since(e.at) > c.ttl {
		return dirCacheEntry{}, false
	}
	return e, true
}

// put stores a freshly read listing.
func (c *dirCache) put(path string, entries []Entry, notes map[string]string) {
	if !c.on(path) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= dirCacheMax {
		oldest, at := "", time.Now()
		for k, e := range c.m {
			if e.at.Before(at) {
				oldest, at = k, e.at
			}
		}
		delete(c.m, oldest)
	}
	c.m[c.key(path)] = dirCacheEntry{entries: entries, notes: notes, at: time.Now()}
}

// drop forgets one directory, for a change made somewhere NEAR is not
// looking (a background copy into a directory that is not on screen).
func (c *dirCache) drop(path string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, c.key(path))
}

// clear forgets everything; Ctrl+R does this.
func (c *dirCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m = map[string]dirCacheEntry{}
}

// visible copies the entries the panel should show. The cache holds the
// full listing so that showing or hiding hidden files needs no re-read,
// and a copy keeps callers (which sort in place) off the cached slice.
func visible(entries []Entry, showHidden bool) []Entry {
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.Hidden && !showHidden {
			continue
		}
		out = append(out, e)
	}
	return out
}

// fmtAge is a short "how long ago" for the panel: 45s, 12m, 2h.
func fmtAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
}
