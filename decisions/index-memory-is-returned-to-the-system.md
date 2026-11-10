# The index's peak memory is handed back to the system instead of kept as collector headroom

Date: 2026-11-10
Status: accepted

## Decision
Once the search index has settled, NEAR returns its free memory to the operating system and
tightens the garbage collector's headroom to a fixed amount rather than a multiple of the
heap. `releaseMemory` (`memory.go`) calls `debug.FreeOSMemory` and then
`debug.SetGCPercent(gcPercentFor(live))`, which leaves `gcHeadroom` = 384 MB of slack over
the live heap, clamped to the Go default of 100 % for small heaps and to 25 % for very large
ones. It runs at the two points where the peak has just passed: the `*EventIndexDone` case in
`App.handleEvent` (reached by a build, a rebuild and the USN catch-up alike) and after each
flat-view compaction in `Index.scheduleFlat`.

The flat view is also sized exactly instead of estimated: `buildFlatIndex` measures the map
before filling the buffers, so `append` never reallocates them.

## Why
With every fixed drive indexed by default (`search-roots-all-fixed-drives.md`) the developer's
index is 7.8 million names, a live heap of about 940 MB — and the process sat at 2.9 GB.
Measurement showed that two thirds of that was not data:

- `buildFlatIndex` guessed the name buffer at 16 bytes per name, 124 MB against 187 MB of
  actual names, so it was copied into a 248 MB buffer mid-build. A compaction paid that on top
  of keeping the old view live, which is the 830 MB step visible 30 s after start-up.
- `GOGC=100` reserves a second gigabyte before collecting, and the runtime releases free pages
  only lazily, so the load and build peaks stayed charged to the process indefinitely.

A fixed headroom rather than a lower `GOGC` matters because the same code runs for a user
whose index is a few megabytes: a flat 40 % would make them collect every few megabytes
during a copy or a directory walk, which the scaling avoids by keeping the default there.

Resident memory with the real 272 MB cache, sampled every 5 s: 2694 MB → 1168 MB settled,
2886 MB → 1345 MB during a compaction, 2886 MB → ~1240 MB steady. That is 1.3× the live heap.

## Consequences
- A file manager holding a whole-machine index costs ~1.2 GB instead of ~2.9 GB.
- Collections are more frequent on a large index. They cost nothing while NEAR waits for a
  key, and the 384 MB of headroom is enough that directory walks and transfers do not notice.
- `FreeOSMemory` is a stop-the-world collection, so it is only called at those two settling
  points, never on a key press or during a transfer.
- Reducing the footprint further means shrinking the live set itself — the names are held
  twice, as the map's `[]string` and as the flat buffer. That is left as
  `todo/index-memory-and-search-speed.md`.

## Where
`memory.go` (`releaseMemory`, `gcPercentFor`, `gcHeadroom`, `gcPercentMin`),
`app.go` (`*EventIndexDone`), `search.go` (`scheduleFlat`), `flatindex.go` (`buildFlatIndex`),
`memory_test.go`, `todo/index-memory-and-search-speed.md`.
