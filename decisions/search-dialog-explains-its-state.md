# Index messages live in the search dialog, which states what it is doing

Date: 2026-10-25
Status: accepted

## Decision
Index state is shown only inside the Ctrl+F dialog, never in the status line (except a
failure when a build ends). While the list is empty the dialog says why: building (roots,
names so far, directory being read, refreshed by a progress event every 300 ms), failed
(error and what to do), empty, searching, or no matches; with an empty query it says how
many names it can search.

## Why
Index messages in the status row disturbed the console view; an empty dialog with "index
is empty" hid that an old binary was failing on the new `"*"` root.

## Where
`searchdialog.go` stateLines, `search.go` EventIndexProgress.
