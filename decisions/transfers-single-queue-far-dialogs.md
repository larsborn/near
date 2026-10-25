# Copies and moves use FAR's dialogs and one strictly sequential background queue

Date: 2026-10-23
Status: accepted

## Decision
F5/F6 open FAR's Copy / Rename-Move dialog (destination, existing-files policy Ask /
Overwrite / Skip / Only newer, preserve timestamps, copy contents of symbolic links,
Background button). The progress box is FAR's with a bar, totals, elapsed, remaining and
speed. Files are copied in 4 MB chunks so Esc cancels at any time and removes the partial
target. All transfers go through one queue with one worker and run strictly one after
another, never in parallel; B sends a running transfer to the background; a waiting
transfer's box lists the queue with finished, running and queued entries; Ctrl+B lists all
transfers with their outcome and Delete cancels one. Quitting with transfers running asks.

## Why
Requested step by step: FAR's look (screenshots), an ETA, "send to background", "all
background transfers consecutive, not in parallel", "show the previous transfers and their
status instead of Waiting…".

## Consequences
A second copy waits for the first even when the disks differ. Tests must slow copies
(`copyPace`) to observe queue states on fast machines.

## Where
`transfers.go`, `copydialog.go`, `fsops.go` copyFile, README "Copy and move" and
"Background transfers".
