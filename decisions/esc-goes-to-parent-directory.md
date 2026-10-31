# Esc (and Backspace) go to the parent directory

Date: 2026-10-23
Status: superseded by backspace-never-goes-up.md

## Decision
With an empty command line, Esc and Backspace move the panel to the parent directory
(`up_dir`). At a drive or file system root they open the locations list. With text in the
command line Esc clears it instead. Every key is configurable in `[keys]`.

## Why
Requested from the start ("ESC = dir up, configurable").

## Consequences
Esc cannot be a cancel key at the panel level; dialogs still close on Esc because dialogs
get the key first.

## Where
`config.go` `up_dir = ["Esc", "Backspace"]`, `app.go` goUp and handleKey.
