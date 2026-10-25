# FAR's ! symbols are inserted unquoted; NEAR's {file} placeholders stay quoted

Date: 2026-10-25
Status: accepted

## Decision
In menu commands and Apply command, `!.!`, `!`, `!~`, `` !` ``, `!\`, `!/`, `!:`, `!@!` are
replaced by the bare value, as FAR does. `!&` quotes each selected name (FAR's default `Q`
modifier). NEAR's own `{file} {path} {files} {paths} {dir} {other} {name} {ext}` keep
quoting for the shell.

## Why
`pdflatex "!.tex"` must work; quoting `!` produced `""name".tex"`. FAR's help: "no
additional characters (for example, quotes) are added, and you should add them yourself".

## Consequences
Apply command's earlier behaviour (quoted `!.!`) changed; the Apply dialog's help line says
so. Users write `"!.!"` where names may contain spaces.

## Where
`applycmd.go` expandApply and applyHelpLines, README "Quick launch menu".
