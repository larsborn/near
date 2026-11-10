package main

import (
	"runtime"
	"runtime/debug"
)

// NEAR allocates very little to run — a megabyte of live heap with no
// index — but it passes through two large peaks that the runtime then sits
// on, because free pages go back to the system only slowly and an idle file
// manager never allocates enough to trigger another collection.
//
// The first peak is start-up, and it is not NEAR's: on Windows tcell takes
// its initial size from the console's *screen buffer* rather than its
// window (`winTty.NewDevTty` reads dwSize), so with cmd's default 9001-line
// scrollback `tScreen.Init` sizes a 120x9001 cell buffer — about 200 MB —
// and throws it away when the first resize event reports the real 120x30.
// See todo/tcell-initial-size-is-the-scrollback-buffer.md.
//
// The second is the search index: roughly a gigabyte for eight million
// names on every fixed drive, where loading, building or compacting the
// flat view briefly needs a good deal more.
//
// releaseMemory is called once after start-up and whenever the index has
// settled. It returns what is free straight away, then sets the collector's
// headroom to a fixed amount of memory rather than a multiple of the heap:
// a large index no longer reserves a second copy of itself, while a small
// one keeps the default behaviour, so copying and directory walking do not
// start collecting every few megabytes.
func releaseMemory() {
	debug.FreeOSMemory() // collects, so HeapAlloc below is the live set
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	debug.SetGCPercent(gcPercentFor(int64(m.HeapAlloc)))
}

const (
	gcHeadroom   = 384 << 20 // memory the collector may use before running
	gcPercentMin = 25        // never collect more eagerly than this
)

// gcPercentFor is the GOGC value that gives gcHeadroom bytes of slack over
// a live heap of the given size, clamped to the default 100 (small heaps)
// and to gcPercentMin (very large ones).
func gcPercentFor(live int64) int {
	if live <= 0 {
		return 100
	}
	pct := gcHeadroom * 100 / live
	if pct > 100 {
		return 100
	}
	if pct < gcPercentMin {
		return gcPercentMin
	}
	return int(pct)
}
