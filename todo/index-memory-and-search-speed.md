# Search index: memory footprint on very large trees

## What
The Everything-like index (`search.go`) is `map[directory]nameBlob` plus a flat search view
(`flatindex.go`: one lowercased byte buffer and three int32 arrays). On the developer's
machine it holds ~7.8 million names in ~847,000 directories.

## Status (2026-11-10): 2886 MB → 755 MB when searching, 16 MB when not

Latency was never the problem — every query is 1.5–45 ms. Memory was. Measured resident
memory, always on a normal start that loads the real cache with `search_roots = ["*"]`:

| | working set |
| --- | --- |
| at the start of this work | 2886 MB |
| returning the peak, exact flat buffers | 1348 MB |
| + `dirLow` byte buffer, `dirPos` binary search | 1026 MB |
| + name blobs instead of `[]string` | **755 MB** |
| a session that never opens the search dialog | **16 MB** |

Live heap 938 → 720 MB; cache file 285 → 273 MB; 167 → 102 bytes resident per name. What was
done, each with its own decision file:

- `decisions/index-memory-is-returned-to-the-system.md` — the peaks were kept as collector
  headroom and never returned. `releaseMemory` now frees and caps the headroom at a fixed
  384 MB; `buildFlatIndex` sizes its buffers from the map instead of estimating.
- `decisions/index-loads-on-first-search.md` — nothing is read until the search dialog is
  opened, so a session that never searches pays 16 MB.
- `decisions/index-names-are-blobs-not-string-slices.md` — `[]string` cost a 16-byte header
  per name plus `append` slack; a blob costs 1 byte per name. Same for the 846,000 lowercased
  directory strings and the directory→index map.

Also worth knowing: about 200 MB of the old baseline was not NEAR's at all, see
`todo/tcell-initial-size-is-the-scrollback-buffer.md`.

## What is left

The 720 MB of live heap now breaks down roughly as:

```
186 MB  flat.names         every name, lowercased
186 MB  the blobs          every name, original case
 96 MB  map keys           846 k directory paths (83 MB of bytes + headers)
 93 MB  off/dirIx/nameIx   12 bytes per name
 86 MB  flat.lowBuf        the same paths again, lowercased
 42 MB  map bucket overhead
 13 MB  flat.dirs          headers sharing the map's key bytes
```

So the remaining duplication is **every name stored twice** (372 MB) and **every directory
path stored twice** (182 MB). Neither is removable without a design decision:

- **Names once (~186 MB).** The flat buffer must stay lowercased for the vectorised scan, so
  the original case has to come from somewhere. Two candidates:
  (a) a per-name case mask — for a 23-character ASCII name a 3-byte bitmask, or a varint list
      of the few uppercase positions, with a side table for names where Unicode lowering is
      not reversible; (b) resolve the real name from the filesystem for the ≤
      `search_max_results` results actually displayed, which would also replace today's
      verification against the map. (b) is simpler but breaks on case-sensitive filesystems
      where `README` and `readme` can coexist in one directory.
- **Paths once (~86 MB).** `flat.lowBuf` is the map's keys lowercased. It could be dropped and
  computed per query — path terms already scan every directory — at the cost of 846 k
  `ToLower` allocations per path query.
- `nameIx` (31 MB) exists only to find a name in its blob for verification. If names were
  stored once it would go too.
- Intern directory strings so the map key and `flat.dirs` share one allocation (they already
  share the bytes; this is only headers).
- Suggest `AppData` in the generated Windows `search_exclude`: 1.16 M of the 7.8 M names are
  there, and 3.4 M more are in one project tree full of build output.

## How to measure
`%LOCALAPPDATA%\Temp\nearrepro\` holds the harness used above (not in the repo):
`memprobe.ps1 -Near <exe> -Roots 'search_roots = ["*"]' -OnStart true -WithCache` copies the
real config and cache into a throwaway `-config` directory, starts NEAR in its own conhost
window, samples the working set and kills it. `memverify.ps1` does a rebuild, keeps the cache
and then measures the load path. For the live-heap breakdown, write a throwaway test that
`LoadCache`s a real `index.gob`, waits for `flatBusy` to clear and logs `runtime.MemStats`
plus the sizes above — and delete it again rather than making every `go test` run load 273 MB.

## Acceptance
Done for everything that does not need a new design decision. The two remaining items —
storing names once and paths once — should be split into their own task when picked up, with
the trade-offs above put to the user first.
