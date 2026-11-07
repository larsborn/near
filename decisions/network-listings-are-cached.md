# Directory listings of network drives are cached; every deliberate reload bypasses them

Date: 2026-11-07
Status: accepted

## Decision
Listings of directories on network drives (mapped drives and UNC paths on Windows, NFS, SMB,
sshfs and similar elsewhere) are kept for `network_cache_minutes`, default 60, and reused
while navigating. Local directories are never cached. Ctrl+R forgets the whole cache and
re-reads; so does every operation that changes something, because they all end in
`reloadAll`, which reads from the disk. The cache stores unfiltered listings so that showing
or hiding hidden files needs no second read, and hands out copies so callers can sort in
place. A panel showing a reused listing says how old it is, `cached 12m`, next to the hidden
flag.

## Why
Switching to the user's `S:` drive took seconds every time, for a long listing over the
network that rarely changes. Caching only the navigation path keeps the stale window where
it cannot cause harm: a copy, rename or delete is always followed by a real read, so NEAR
never shows a listing that contradicts what it just did.

## Consequences
A file created by another machine, or outside NEAR, can stay invisible for up to an hour on
a network drive. The age in the path row says so, and Ctrl+R is the cure. The cache holds at
most 300 listings, dropping the oldest.

## Where
`dircache.go`, `slowdir.go` (`listDirWait`, `listDirFresh`), `app.go` (`readTab`,
`reloadAll`, the refresh action), `drives_*.go` (`isNetworkPath`), `dircache_test.go`.
