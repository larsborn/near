# A slow directory read shows a box after 300 ms; Esc abandons it and stays where it was

Date: 2026-10-24
Status: accepted

## Decision
Directory listings are read in a goroutine. If one takes longer than 300 ms a "Reading
directory…" box appears with Esc to cancel; cancelling keeps the previous listing and
directory. The same applies to re-reads of the current directory.

## Why
Switching to a slow network share (S:) left the cursor blinking with no indication and no
way out.

## Where
`slowdir.go` listDirWait, `app.go` navigate and loadTab.
