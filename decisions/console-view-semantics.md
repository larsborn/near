# Commands run in a console view: output stays, prompt only after termination, panels come back

Date: 2026-10-23
Status: accepted

## Decision
A command typed on the command line runs with its output captured into NEAR's console
view (SGR colours kept), which replaces the panels while it runs. The prompt returns only
when the command has terminated. If the panels were visible before, they come back
afterwards and the tail of the output stays visible in a band under them (`console_rows`,
default 6); the band takes the status row when there is no message so no blank row sits
above the prompt. With the panels hidden (Ctrl+O) the view behaves like a shell: Up/Down
recall history, PgUp/PgDn scroll, Tab completes paths, Enter on an empty line prints a new
prompt, and nothing reaches the hidden panels. No hint lines such as "Ctrl+C stops it" are
shown. The buffer keeps `console_lines` = 5000 lines. Index messages never appear here.

## Why
The user expected `git status` output to stay on screen and to be "dropped back into a
shell", not into a pager that disappears on Enter; later requests removed the hints, the
blank row and a blue artefact, and asked for the panels to return by themselves.

## Consequences
Programs needing a real terminal must be listed in `interactive_commands` or run with
Shift+Enter (see `interactive-commands-run-outside.md`). No pty, so curses programs cannot
run inside the view (`todo/console-pty-for-interactive-commands.md`).

## Where
`console.go`, `cmdline.go`, `app.go` handleKey console-view block, `draw.go`, README
"Console view".
