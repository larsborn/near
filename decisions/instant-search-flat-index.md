# The index search must feel instant: flat name buffer, background search, capped counts

Date: 2026-10-25
Status: accepted

## Decision
The search dialog never delays typing. Names are kept in one lowercased byte buffer with
offset arrays (`flatIndex`) scanned with bytes.Index; the per-directory map stays the
source of truth and results are verified against it. Each query runs in a goroutine with a
generation counter and the dialog shows the newest result only; counting stops at 10,000
("10,000+ matches"). Watcher changes are appended to the flat view at once and compacted
30 seconds after the last change.

## Why
On the developer's 5-million-name index a query took 330 ms and then still delayed keys
by about 100 ms; the user asked twice for it to be instant.

## Consequences
About 200 MB extra memory for the flat view on that index
(`todo/index-memory-and-search-speed.md`).

## Where
`flatindex.go`, `search.go`, `searchdialog.go`.
