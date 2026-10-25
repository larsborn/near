# Interactive programs run outside the console view, by name list or Shift+Enter

Date: 2026-10-23
Status: accepted

## Decision
Commands whose program is in `interactive_commands` (editors, pagers, shells, ssh, python,
node, cmd, powershell …), anything sent with Shift+Enter, and menu items with `nowait` run
with the terminal handed over: tcell is suspended, the program gets the real console, and
NEAR resumes afterwards. `command_wait = true` asks for Enter before returning so the output
can be read. Everything else runs in the console view with pipes and no stdin.

## Why
The console view cannot host programs that read the keyboard or draw screens. A script
reading the console directly (msvcrt.getwch) while running in the console view fights NEAR
for keystrokes; such wrappers belong in the list.

## Consequences
A batch file wrapping an interactive program is not recognised by name
(`todo/console-pty-for-interactive-commands.md`, vidmerge case).

## Where
`console.go` isInteractive, `commands.go` runShell, `config.go` interactive_commands and
command_wait.
