# Notes on files live in a hidden .near-notes JSON file per directory

Date: 2026-10-23
Status: accepted

## Decision
Ctrl+N edits a note for the current entry. Notes are stored per directory in a hidden file
named by `notes_file` (default `.near-notes`), a JSON object from entry name to text, with
the hidden attribute set on Windows. The note is shown below the listing and flagged by
`*` in the name column; renames, copies and moves made within NEAR carry the note along.
The notes file itself is never listed.

## Why
Requested as "notes on files via hidden file in directory".

## Consequences
Notes do not follow files changed by other programs. Search result panels show no notes.

## Where
`notes.go` (loadNotes, deleteNote), `model.go` listDir, `fsops.go` renameMany, `config.go`
notes_file.
