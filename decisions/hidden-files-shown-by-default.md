# Hidden files are shown by default

Date: 2026-10-23
Status: accepted

## Decision
`show_hidden = true` is the default; dot files and entries with the Windows hidden
attribute are listed. The toggle action stays available.

## Why
The user wants to see everything; hiding was getting in the way.

## Consequences
Directory listings are longer; the "hidden" flag at the right of the path row shows the
mode.

## Where
`config.go` default text, `model.go` listDir, `draw.go` path row.
