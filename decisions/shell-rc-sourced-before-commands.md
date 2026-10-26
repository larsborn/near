# Aliases come from an opt-in `shell_rc` file, not from an interactive shell

Date: 2026-10-26
Status: accepted

## Decision
Commands keep running through `$SHELL -c`. A new config key `shell_rc` (default empty)
names a file that NEAR sources on its own line before the command, with
`shopt -s expand_aliases` when the shell has `shopt`. `~` is expanded; a missing file is
reported in the status line and the command still runs. The working directory is restored
after sourcing, because rc files written for a terminal may `cd`. Ignored on Windows.

## Why
`psg` (a bash alias) did not work at NEAR's command line. `bash -c` is non-interactive:
Ubuntu's `~/.bashrc` returns at its `case $- in *i*)` guard before any alias is defined,
and non-interactive bash does not expand aliases at all. The alternatives were rejected:
- `bash -ic` reads `~/.bashrc` but, without a controlling terminal, prints "cannot set
  terminal process group" and "no job control in this shell" before every command, and an
  interactive bash ignores SIGTERM, which weakens Ctrl+C (`killTree`).
- `BASH_ENV` works only for bash; zsh and fish have their own always-read files, so one
  shell-agnostic "source this file" option is simpler to explain.
- Sourcing `~/.bashrc` by default is pointless because of the guard; the user has to pick a
  guard-free file (`~/.bash_aliases` on Ubuntu) anyway.

## Consequences
Users must keep aliases in a file that is safe to source non-interactively. Output from
that file lands in the console. The console-PTY task (`todo/`) would make `bash -i` viable
later; this option stays useful as the shell-agnostic form.

## Where
`config.go` (`ShellRC`, default text), `commands.go` `shellScript` (used by `runShell` and
`runInConsole`), `console_test.go` `TestConsoleShellRC`, README "Configuration".
