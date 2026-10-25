# The index watcher is native ReadDirectoryChangesW on Windows and fsnotify elsewhere

Date: 2026-10-23
Status: accepted

## Decision
The index is kept current by a cross-platform watcher. On Windows one recursive
ReadDirectoryChangesW handle per root covers the whole tree, armed with overlapped I/O
before Add returns so no early change is lost. Elsewhere fsnotify needs one watch per
directory, capped by `search_watch_max_dirs` (100,000); beyond the cap deeper changes are
only picked up by a rebuild. Events are batched, a create is applied as remove+add, an
overflow triggers a rebuild. The cache (a gob of the directory map) is rejected and rebuilt
when its format is old; indexes over 200,000 names are not written at quit because encoding
them would make quitting slow.

## Why
"Let's build a cross-platform filesystem watcher for the Everything-like feature."

## Where
`watch_windows.go`, `watch.go`, `search.go` apply and Close, `config.go` search_watch*.
