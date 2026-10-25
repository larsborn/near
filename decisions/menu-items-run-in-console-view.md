# Menu items run in the console view; nowait means "interactive program"

Date: 2026-10-26
Status: accepted

## Decision
A menu item without flags runs like a command typed on the command line: output captured
into the console view, panels back when it ends, output kept in the band under them, no
"press Enter" pause. `nowait` hands the terminal to the command (shells, editors) and
returns without a pause. `detach` starts it in the background. Programs listed in
`interactive_commands` run outside and honour `command_wait`.

## Why
Running pdflatex from a menu paused on "Press Enter to return" and its output vanished when
the panels came back. The console path already had the wanted behaviour.

## Consequences
`nowait` changed meaning from "skip the pause" to "run outside, no pause"; the default
items (cmd, powershell) keep working.

## Where
`usermenu.go` runMenuItem, config comment on item flags, README.
