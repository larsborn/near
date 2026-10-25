# Console view: run commands on a pseudo-terminal instead of pipes

## What
Commands typed at NEAR's command line run with stdout/stderr on a pipe and no stdin
(`console.go` `runInConsole`), so their output can be captured into the console view. Programs
that need a terminal are therefore special-cased: anything in `interactive_commands` (vi, less,
ssh, shells, python, …) and anything sent with Shift+Enter runs outside NEAR via `runShell`
(tcell suspended, real terminal, optional "press Enter").

## Why it matters
The list is a heuristic. An unlisted interactive program (e.g. `git add -p`, `npm init`,
`fzf`, a sudo prompt) hangs or misbehaves on the pipe until Ctrl+C. Colours also depend on
tools honouring `CLICOLOR_FORCE`/`FORCE_COLOR`/git's `color.ui=always` env injected by
`consoleEnv`.

## Option considered
Run console commands on a pty (Unix: `github.com/creack/pty`; Windows: ConPTY, e.g.
`github.com/UserExistsError/conpty`) and render the output with a small VT parser. That gives
every program a real terminal, keeps capture, and makes the interactive list unnecessary, at
the cost of a terminal emulator inside NEAR (cursor movement, clears, alt-screen) and input
forwarding while a command runs. Rejected for now as too large; keep in mind if the heuristic
keeps biting.

## Acceptance (if taken up)
`vim`, `less`, `ssh host` and `sudo` work from the command line inside the console view
without being listed; existing console tests still pass; Ctrl+C still kills the process tree.
