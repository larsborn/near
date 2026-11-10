package main

import (
	"runtime"
	"runtime/debug"
)

// The search index is the only large thing NEAR holds: roughly a gigabyte
// for eight million names on every fixed drive. Two things used to turn
// that into nearly three gigabytes of process memory. The collector keeps
// as much again as headroom before it runs, and it hands free pages back to
// the system only slowly, so the peak of a build, a cache load or a flat
// view compaction stayed charged to the process long after it was over.
//
// releaseMemory is called once the index has settled. It returns what is
// free straight away, then sets the collector's headroom to a fixed amount
// of memory rather than a multiple of the heap: a large index no longer
// reserves a second copy of itself, while a small one keeps the default
// behaviour, so copying and directory walking do not start collecting
// every few megabytes.
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
