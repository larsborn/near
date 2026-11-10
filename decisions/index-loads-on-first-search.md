# The search index is loaded on the first search, not at start-up

Date: 2026-11-10
Status: accepted

## Decision
`search_index_on_start` now defaults to `false`, and it means something different: with it
off, NEAR does not read the index cache at all at start-up. `startIndex` only calls
`Index.Configure` (which reads nothing), and the index is brought up by `App.ensureIndex` the
first time `showSearch` runs — read the cache, start the watcher, catch up from the change
journals, or build from scratch when there is no usable cache. `ensureIndex` is idempotent via
`App.indexStarted`, and the work happens in a goroutine so the key that opened the dialog does
not block; `Index.Loading` drives a "Loading the index of …" line in the dialog meanwhile.

`search_index_on_start = true` restores the old behaviour: the whole sequence runs at
start-up, so the first search answers instantly.

`-reindex` no longer works by forcing `search_index_on_start`; it sets `Config.reindex` and
goes straight to `RebuildAsync`, which is what its help text has always claimed.

The roots themselves are unchanged — `search_roots = ["*"]`, every fixed drive
(`search-roots-all-fixed-drives.md`). What changed is when that costs anything.

## Why
Measured on 2026-11-10: started with the index switched off, NEAR is **17 MB working set /
24 MB private**. With the developer's index it is **1.2 GB**. So the entire footprint of the
program is the name index, and under the previous default every user paid for it at start-up
whether or not they ever pressed `Ctrl+F` — a file manager quietly holding a gigabyte for a
feature that may go unused all session.

Loading lazily was preferred over the alternatives. Shrinking the default roots to the home
directory would have cut only 40 % on the developer's box (`C:/Users/born/Projects` alone is
3.4 M of the 7.8 M names) while making the feature worse for everyone. Defaulting
`search_roots` to `[]` would have hidden a headline feature. Lazy loading keeps the feature
exactly as capable and moves the cost to the moment the user asks for it, which is also the
moment a progress message is welcome rather than surprising.

It helps Linux and macOS twice over: with no usable change journal there, the index was
rebuilt from scratch at *every* start (`todo/ntfs-change-journal.md`). That 96-second walk now
happens on the first search instead, and only in sessions that search.

## Consequences
- A session that does not search costs **16-17 MB instead of 1.2 GB**, measured with the real
  285 MB cache sitting unused in the config directory and `search_roots = ["*"]`.
- `search_index_on_start = true` still measures 1348 MB, so the knob really is the trade.
- The first `Ctrl+F` of a session waits while the cache is read — about two seconds for a
  285 MB cache — and the dialog says so. Typing during it is not lost: the dialog is already
  open and searches as soon as the names are there.
- The watcher also starts on first use, so between start-up and the first search nothing is
  being watched. Correctness is unaffected because the change-journal catch-up runs as part
  of coming up, and it covers exactly that gap.
- Users who want the old instant search set `search_index_on_start = true` and pay the memory
  at start-up as before. The developer's live config already has that line set to `true`, so
  the new default does not change their running instance (`live-config-not-regenerated.md`).
- `todo/index-memory-and-search-speed.md` keeps the remaining structural work: every name is
  still stored twice, which is worth another ~300-400 MB for users who do search.

## Where
`app.go` (`startIndex`, `ensureIndex`, `App.indexStarted`), `searchdialog.go` (`showSearch`,
`stateLines`), `search.go` (`Index.loading`, `Index.Loading`), `config.go` (`Config.reindex`,
the `search_index_on_start` default and comment), `main.go` (`-reindex`),
`lazyindex_test.go`, README "Search".
