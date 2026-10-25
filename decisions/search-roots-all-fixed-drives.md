# The search index covers all fixed drives by default

Date: 2026-10-25
Status: accepted

## Decision
`search_roots = ["*"]` is the default. `"*"` expands to every fixed drive: DRIVE_FIXED
letters on Windows minus subst drives, local disk file systems on non-removable media on
Linux (from /proc/self/mounts), getfsstat local volumes on macOS and FreeBSD without the
macOS system volumes under /System/Volumes. Network, removable and optical drives are left
out. Other entries remain directories. A root inside another root is dropped so nothing is
indexed or watched twice.

## Why
The user compared NEAR with Everything, which indexes whole drives, and wanted the same
coverage out of the box. Subst drives only mirror a directory of another drive and would
list everything under it twice.

## Consequences
A fresh install indexes the whole of C: and the other internal drives. On the developer's
box that is 7.7 million names, 283 MB of cache and about 3 GB of memory
(`todo/index-memory-and-search-speed.md`).

## Where
`drives_*.go` fixedDrives, `search.go` expandRoots and outerRoots, config comment, README.
