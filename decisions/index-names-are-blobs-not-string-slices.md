# A directory's names are one length-prefixed blob, not a []string

Date: 2026-11-10
Status: accepted

## Decision
`Index.dirs` is `map[string]nameBlob`, where `nameBlob` (`nameblob.go`) holds a directory's
entry names as a single run of bytes, each name preceded by its length as a varint. Names are
addressed by ordinal — the same position the flat view records in `nameIx` — and `removeAt`
keeps the old swap-the-last-name-into-the-hole behaviour so the other ordinals stay valid.
`Build` compacts every blob before the map goes live, since the walk grew them with `append`.

Two structures in the flat view went the same way:

- `flatIndex.dirLow`, 846,000 lowercased directory strings, is now one `lowBuf` byte buffer
  with a `lowOff` offset array. Each directory is stored with a trailing `/`, which is what a
  path-prefix test wants anyway (`dirLow`, `dirLowSlash`).
- `flatIndex.dirPos`, a map from every directory to its index, is gone. `dirs[:dirsBuilt]` is
  sorted, so `dirIndex` binary-searches it and only directories discovered after the build need
  the small `dirAdded` map.

`matcher` also carries its term as `[]byte` so the scan stops converting it per candidate.

The index cache format changes with this, so caches written by older versions are refused by
`LoadCache` and the index is rebuilt (`TestOldFormatCacheIsRejected`).

## Why
A `[]string` costs a 16-byte header per name plus whatever `append` left spare in the slice.
On the developer's 7.8-million-name index that was about 160 MB of overhead on top of the
179 MB the names themselves need — measured afterwards, a blob's overhead is **1 byte per
name**. The directory map and the lowercased directory strings cost comparably for 846,000
directories.

Measured resident memory on a normal start that loads the real cache, all fixed drives indexed:

| | working set |
| --- | --- |
| before any of this work | 2886 MB |
| returning the peak, exact flat buffers | 1348 MB |
| + `dirLow` buffer, `dirPos` binary search | 1026 MB |
| + name blobs | **755 MB** |

Live heap went 938 → 720 MB and the cache file 285 → 273 MB. Per name, resident cost fell from
167 to 102 bytes. The flat scan was checked against the slow map scan on the real 7.8-million
name index, and `go test ./...` — which includes both watcher backends and the USN catch-up —
passes, which is what exercises the ordinal semantics.

A length prefix was chosen over a `'\n'` separator because on Linux a file name may contain
any byte but `/` and NUL, newlines included. (The flat view's own buffer still separates
lowercased names with `'\n'`; that is a pre-existing assumption, not one worth adding to the
authoritative store.)

## Consequences
- Everyone's index cache is rebuilt once after upgrading: a few minutes of background walking
  on a whole-machine index, or on Windows no wait at all if a journal catch-up suffices.
- Reading a name costs a short scan of its directory's blob instead of a slice index.
  Directories hold about nine names on average, so this is nanoseconds, and the places that do
  it per result are bounded by `search_max_results`.
- `nameBlob` aliases its own storage: `at` and `each` hand out slices into the blob, so callers
  must copy before keeping them. Both the index and the search path do.
- The names are still held twice — once in the blobs and once lowercased in the flat view's
  buffer, 186 MB each. That is now the largest single item and it needs its own design; see
  `todo/index-memory-and-search-speed.md`.

## Where
`nameblob.go` and `nameblob_test.go`, `flatindex.go` (`lowBuf`/`lowOff`, `dirIndex`, `addDir`,
`buildFlatIndex`, `appendLower`, `equalLower`, `search`), `search.go` (`Index.dirs`,
`indexCache.Dirs`, `Build`, `syncDirs`, `Search`, `apply`, `removeNameLocked`,
`addNameLocked`), `app_test.go` (`TestOldFormatCacheIsRejected`), `memory_test.go`.
