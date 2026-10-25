# The command line always has the focus; Tab still switches panels

Date: 2026-10-23
Status: accepted

## Decision
Printable keys type into the command line at the bottom; Tab and the cursor keys still
drive the panels. With text in the line, Enter runs it in the active panel's directory and
Esc clears it. Keys bound to actions that are printable (`+ - * Space`) act only while the
line is empty. Ctrl+Enter inserts the current entry's name (Ctrl+J is the fallback for
terminals that cannot report Ctrl+Enter), Alt+Enter its full path. `cd <dir>` moves the
panel instead of spawning a shell. The working directory of every command is the active
panel's directory.

## Why
FAR's shell line; requested as "shell at the bottom which has focus by default, Tab should
still switch between the panes, Ctrl+Return should insert the selected file name".

## Consequences
Single-letter hot keys cannot be bound at the panel level, which is why NEAR uses
function and Ctrl keys. The key routing order in `handleKey` is fixed and documented in
CLAUDE.md.

## Where
`cmdline.go`, `app.go` handleKey, CLAUDE.md "Event loop and key routing".
