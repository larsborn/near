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

## Where the memory actually goes (measured 2026-11-10)

None of it is the file manager. Started with `search_roots = []` and no cache, NEAR sits at
**17 MB working set / 24 MB private**. Everything above that is the name index, so the
question is only ever "how much does a name cost" times "how many names are indexed".

Per name, on the developer's index: **127 bytes of live heap, 167 bytes resident**. The map
and the flat view each hold a full copy of every name — 179 MB of original bytes plus 186 MB
lowercased — and on top of that every name carries a 16-byte string header and 12 bytes of
`off`/`dirIx`/`nameIx`. That is ~75 of the 127 bytes; the rest is the per-directory
structures (846 k keys at 83 MB, `dirLow` ~96 MB, `dirPos` ~40 MB, slice headers plus the
capacity slack `append` leaves in 846 k `[]string` slices).

How many names is the other half of the question. With the default `search_roots = ["*"]`
(every fixed drive) and the default excludes (`.git`, `node_modules`, `__pycache__`,
`.cache`) the index is 7.78 M names, and it is dominated by two subtrees:

```
3395685 names  290905 dirs   C:/Users/born/Projects      (44 %, mostly build output)
1156439 names  133460 dirs   C:/Users/born/AppData       (15 %)
 155664 names   14225 dirs   C:/Users/born/Programs
 135441 names   11413 dirs   D:/Backup/TjockBackup-pre2013/Projects
 110774 names  110518 dirs   C:/Windows/WinSxS/Temp      (one name per directory)
```

So 59 % of the footprint is one developer's project tree and one application-data tree.

## What is left — four directions, not yet chosen

**A. Index fewer names (cheap, changes what is findable).** Add build-output directory names
to the default `search_exclude` (`target`, `bin`, `obj`, `dist`, `build`, `.venv`, `.tox`,
`vendor`, `.gradle`, `.next`) and `AppData`/`Library` per platform. On the measured index that
is plausibly 2–3 M names, i.e. 300–500 MB. Risk: a user who searches for a built artefact no
longer finds it, and the index silently differs from the filesystem. Needs a decision file.

**B. Stop storing every name twice (the real engineering fix, ~300–400 MB).** The map's
`[]string` duplicates what `flat.names` already holds. Make the flat view the only store:
names in one buffer, original case either recovered from the lowercased bytes plus a case
mask or kept in a second buffer, and `Index.dirs` reduced to `map[string]range`. Watcher
updates and result verification both have to work off the arrays instead of the slices — that
is the whole difficulty, since `apply` currently relies on cheap per-directory slice edits.
Target ~60–70 bytes per name, i.e. ~550 MB for this index, and a smaller gob cache too.

**C. Let the OS own the index (`mmap`, the most promising for a file manager).** The flat view
is already a byte buffer and three int32 arrays — exactly what maps well. Memory-map the cache
file instead of decoding a gob into the heap: resident memory becomes elastic, the kernel
evicts it under pressure and pages it back for a query, and start-up stops costing a 285 MB
decode. Cost: the watcher can no longer edit in place, so changes need an in-heap overlay plus
a periodic rewrite of the file, and it is new platform code (`mmap` / `MapViewOfFile`).

**D. Do not index by default.** `search_roots = ["*"]` is why an ordinary user pays a gigabyte
for a feature they may never press `Ctrl+F` for. Alternatives: default to the home directory,
or build the index on first use of the search dialog and say so. Supersedes
`decisions/search-roots-all-fixed-drives.md`, so it needs its own decision file.

Smaller follow-ups, independent of the above:

- `flat.dirLow` is 846 k lowercased directory strings (~96 MB) used only by path terms; it
  could be a byte buffer with offsets, or computed per query.
- `flat.dirPos` (~40 MB) could be a binary search over the sorted `dirs[:built]` plus a small
  map for directories added after the build.
- Intern directory strings once (the map keys and `flat.dirs` duplicate the headers, not the
  bytes).
- The 846 k `[]string` slices carry `append`'s capacity slack; building them to exact length
  from the cache would recover tens of megabytes.

## Acceptance
The slack half is done: 2.9 GB → 1.2 GB, figures above. What remains is a choice between A–D,
which is a product decision as much as a technical one and should be put to the user rather
than picked silently. Whichever is taken gets its own task file and decision record.
