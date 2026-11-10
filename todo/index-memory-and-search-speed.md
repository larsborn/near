# Search index: memory footprint on very large trees

## What
The Everything-like index (`search.go`) is `map[directory][]names` plus a flat search view
(`flatindex.go`: one lowercased byte buffer and three int32 arrays). On the developer's
machine it holds ~7.8 million names in ~847,000 directories.

## Status (2026-11-10): the resident footprint is down from 2.9 GB to 1.2 GB

Latency was never the problem: every query on that index takes 1.5–45 ms (flat buffer scan
with `bytes.Index`, glob literals and directory narrowing for path terms, counting capped at
10,000). Memory was, and most of it turned out to be collector slack and build transients
rather than data.

Measured by loading the real 272 MB cache in a test (`Index.LoadCache` plus the flat view,
7,783,059 names in 846,592 directories):

```
after load + flat view   heapAlloc=1125 MB  heapInuse=1290 MB  heapSys=1607 MB
after GC                 heapAlloc= 938 MB  heapInuse=1104 MB  heapSys=1607 MB
map: key bytes 83 MB, name bytes 179 MB, 8.6 M string headers = 131 MB
flat: names 186 MB, off 29 MB, dirIx 29 MB, nameIx 29 MB
```

So the live set is ~940 MB, but the process held 2.9 GB. Two causes, both now fixed:

- `buildFlatIndex` estimated the name buffer at `count*16` bytes — 124 MB for 187 MB of real
  names — so `append` reallocated it to twice the size mid-build and the 124 MB copy became
  garbage. It now measures the map first and allocates exactly (`TestFlatIndexAllocatesExactly`).
- The collector kept 100 % headroom over a gigabyte of index and returned free pages only
  lazily. `releaseMemory` (`memory.go`) now runs after the index settles (`EventIndexDone`)
  and after each flat-view compaction: `debug.FreeOSMemory` plus a GOGC scaled to leave a
  fixed 384 MB of slack instead of a multiple of the heap, so small indexes keep the default.

Resident memory with the real cache, isolated `-config` copy, sampled every 5 s:

| | before | after |
| --- | --- | --- |
| settled after load | 2694 MB | 1168 MB |
| after a flat compaction (30 s) | 2886 MB | 1345 MB |
| steady | 2886 MB | ~1240 MB |

That is 1.3× the live heap, which is about the floor for this data in a GC'd runtime.

## What is left

Reducing the footprint further means reducing the live set, i.e. the 310 MB the map spends on
names (179 MB of bytes, 131 MB of string headers) that the flat view's 186 MB buffer already
holds in lowercased form. Ideas, roughly in order of value:

- Drop the per-directory `[]string` slices in favour of the flat view as the primary store.
  The map is only needed for watcher updates and for verifying results, both of which could
  work off the flat arrays. Worth ~300 MB, but it is the one real refactor here.
- `flat.dirLow` is 846 k lowercased directory strings (~96 MB) used only by path terms; it
  could be a byte buffer with offsets, or computed per query.
- `flat.dirPos` (~40 MB) could be a binary search over the sorted `dirs[:built]` plus a small
  map for directories added after the build.
- Intern directory strings once (the map keys and `flat.dirs` duplicate the headers, not the
  bytes).
- Suggest `AppData` in the generated Windows `search_exclude`; much of the index is there.

## Acceptance
Done for the measurement and the slack: figures recorded above, 2.9 GB → 1.2 GB. The
remaining item is the structural one — serve names from the flat view and drop the map's
`[]string` slices — which should be split into its own task when it is picked up.
