# A menu command naming a file runs once per selected entry

Date: 2026-10-25
Status: accepted

## Decision
If a menu item's command uses a single-file symbol (`!`, `!.!`, `!~`, `` !` ``, `{file}`,
`{path}`, `{name}`, `{ext}`) and several entries are selected, the command lines are
expanded once per selected entry and joined into one shell run. Commands without such
symbols run once; `!&` and `{files}` give the whole selection. `!?Title?Default!` prompts are
asked before anything runs.

## Why
The user asked for the command to be "executed on the current selection". FAR itself runs
a user menu command once and offers `!&` for the selection; this is a deliberate deviation,
in line with NEAR's Apply command.

## Consequences
`pdflatex "!.tex"` on three selected files compiles all three. Very large selections make
long shell lines.

## Where
`usermenu.go` usesFileSymbols and menuCommandLine, README.
