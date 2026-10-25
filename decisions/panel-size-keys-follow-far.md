# Panel height and width keys follow FAR, including the direction of Ctrl+Up/Down

Date: 2026-10-23
Status: accepted

## Decision
Ctrl+Up makes both panels shorter and Ctrl+Down taller (checked against FAR's source after
they were first implemented the other way round); Ctrl+Shift+Up/Down resize the current
panel only; Ctrl+Left/Right move the divider. These work only while the command line is
empty. Ctrl+O hides both panels, Ctrl+F1/Ctrl+F2 the left/right one; a hidden panel keeps
its width. Sizes are kept across sessions. Rows taken off the panels show the console band.

## Why
The user asked to research FAR's defaults and to re-implement them exactly.

## Where
`app.go` resizePanels, panelSplit, consoleBand; `config.go` panels_* keys; README "Panel
control".
