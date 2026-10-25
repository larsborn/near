# Histories and compare sit on FAR's keys

Date: 2026-10-23
Status: accepted

## Decision
Alt+F8 lists the command history (Enter runs, Shift+Enter edits, Delete removes), Alt+F12
the folder history (Enter goes, Ctrl+T new tab, Shift+Enter other panel), Alt+F11 the
viewed and edited files (Enter views, F4 edits, Ctrl+Enter jumps to it). Lists are
searchable by typing and kept across sessions in state.json. Ctrl+E/Ctrl+X step through
the command history on the command line. Shift+F3 compares the panels (missing, size,
newer, optionally content) or the current files side by side with changed lines coloured.

## Why
"Let's add new features next: alt history, command history, visual compare", on FAR's
bindings.

## Where
`history.go`, `compare.go`, `state.go`, `config.go` history* and compare keys.
