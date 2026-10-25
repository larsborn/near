# The product name is written NEAR in prose, lowercase as a command and in file names

Date: 2026-10-26
Status: accepted

## Decision
Prose everywhere (README, help screens, generated config comments, dialog entries,
messages, release titles, CLAUDE.md, todo and decision files) spells the name NEAR. Things
that are typed or exist on disk stay lowercase: the `near` command and its usage line,
`near.exe`, the release archive names, `.near-notes`, the `near` config directory, the Go
module.

## Why
The user asked for "allcaps everywhere"; a capitalised command name would be wrong to type.

## Where
`CLAUDE.md`, README, `config.go` default text, `contextmenu.go`, `main.go`, `release.yml`.
