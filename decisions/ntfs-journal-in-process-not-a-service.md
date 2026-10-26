# The index catches up from the NTFS change journal inside NEAR, not through a background service

Date: 2026-10-26
Status: accepted

## Decision
On Windows, NEAR keeps the search index current by reading the NTFS change journal (USN
journal) itself, unprivileged, in the NEAR process: at start it applies the records since
the position stored in the cache instead of rebuilding, and later tails the journal while
running. No Windows service is installed. The full walk remains the initial build and the
fallback whenever the journal cannot be used (journal recreated or wrapped, disabled,
non-NTFS volume, older Windows, other platforms).

## Why
The start-up rebuild exists only to catch changes made while NEAR was closed; on an index of
all fixed drives it costs a 96-second walk and about 3 GB of memory churn at every start.
The journal removes that cost. A service, as Everything uses, would additionally keep the
index warm while NEAR is closed and allow an instant first build from the file table, but
needs an elevated install step, a process running as the system account, a named-pipe
protocol and a second copy of the index in memory. NEAR's index is per user and only used
while NEAR runs, so the in-process form gives nearly all of the benefit for a fraction of
the complexity. A service can still be added on top of the same journal code later.

## Consequences
Windows-only code behind build tags. Enumerating the file table needs administrator rights
and is not used; the initial build stays the walk. Linux and macOS have no equivalent
journal and get an age-based start-up rebuild instead.

## Where
`usn_windows.go`, `search.go` CatchUp/syncDirs and the cache's journal positions,
`app.go` startIndex, `todo/ntfs-change-journal.md` (live tailing and the age-based
rebuild for other platforms are still open there).
