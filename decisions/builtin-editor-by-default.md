# F4 opens a built-in FAR-like editor; the external editor is Ctrl+F4 or a config switch

Date: 2026-10-23
Status: accepted

## Decision
F4 edits in NEAR's own editor modelled on FAR's keys (F2 save, Shift+F2 save as, F7 find,
Shift+F7/Alt+F7 next/previous, Ctrl+F7 replace, Alt+F8 go to line, Ctrl+Z/Ctrl+Shift+Z
undo/redo, Ctrl+Y delete line, Ctrl+K delete to end of line, Shift+movement selects,
Insert toggles overwrite). CRLF and a UTF-8 BOM are preserved. Ctrl+F4 uses the external
editor; `internal_editor = false` makes that the default. Typing over a selection replaces
it (modern behaviour; FAR keeps the text, open point in `todo/small-open-points.md`).
Shift+F4 creates a new file.

## Why
"F4 opens in an external text editor, can we replicate the one from within FAR?"

## Where
`editor.go`, `commands.go` editFileThen, `config.go` internal_editor.
