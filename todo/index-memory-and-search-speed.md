# Search index: memory footprint on very large trees

## What
The Everything-like index (`search.go`) is `map[directory][]names` plus a flat search view
(`flatindex.go`: one lowercased byte buffer and three int32 arrays). On the developer's
machine it holds ~5 million names in ~475,000 directories.

## Status (2026-10-25)
Latency is solved: every query on that index takes 1.5–45 ms (flat buffer scan with
`bytes.Index`, glob literals and directory narrowing for path terms, counting capped at
10,000). Remaining concern is memory: roughly 600 MB for the map plus ~200 MB for the flat
view (124 MB names buffer, 60 MB of int32 arrays, the directory position map).

Since 2026-10-25 the default `search_roots` is `["*"]`, every fixed drive (`drives_*.go`,
`expandRoots` in `search.go`), so a typical Windows install indexes the whole of C: rather
than the home directory. That makes the footprint the default experience, not a power-user
choice: measure it with all of C: indexed, not just the home directory.

Measured on 2026-10-25 20:29 with the new default on the developer's box (C: and D:):
7,697,088 names in 832,109 directories, built in 96 s, cache 283 MB, and the process sat at
3.2 GB private memory / 3.9 GB working set after the build. That is the number to bring down.

Since 2026-10-26 a start on Windows no longer rebuilds: the index catches up from the NTFS
change journal (`todo/ntfs-change-journal.md`), so the start-up churn is gone there; the
footprint of the loaded index itself is unchanged.

## Ideas
- Drop the per-directory `[]string` slices in favour of the flat view as the primary store
  (the map is now only needed for watcher updates and result verification).
- Intern directory strings once (the map keys and `flat.dirs` duplicate them).
- Suggest `AppData` in the generated Windows `search_exclude`; most of the 5M names are there.
- Measure RSS at startup with the real cache and record it in README.

## Acceptance
Memory for a 5M-name index measured and documented; either reduced substantially or the
defaults changed so typical users never index that much.
