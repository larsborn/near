# Ctrl+G is FAR's Apply command; go-to-path moved to Alt+F10

Date: 2026-10-25
Status: accepted

## Decision
Ctrl+G opens Apply command: a command template run once for every selected entry, in the
console view, one after another, with FAR's `!` symbols and `!?Title?Default!` prompts.
The former Ctrl+G action, go to a directory by path, is bound to Alt+F10.

## Why
"Go to directory seems to be Ctrl+G, it should be a different one and Ctrl+G should be
'batch shell command' instead."

## Where
`applycmd.go`, `config.go` apply_command and goto_path.
