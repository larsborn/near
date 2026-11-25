# The console fills the column of a hidden panel

Date: 2026-11-25
Status: accepted

## Decision
When exactly one panel is hidden (`Ctrl+P`, `Ctrl+F1`, `Ctrl+F2`) and the console has output,
the console is drawn in the column that panel left, for the full height of the panel area,
with one column of air next to the remaining panel's border. The remaining panel is then
drawn at the full height, and the band under the panels is not drawn — it would only repeat
the same tail in a second place.

`drawConsole` took `(top, height)` and assumed the full width; it now takes `(x, top, w,
height)`. Everything that follows the console rect — wrapping width, the scroll tag, the
mouse hit test, drag clamping, wheel scrolling and the selection highlight — works off
`App.conArea` rather than `App.w`.

Nothing changes with both panels visible (the band as before) or with both hidden (`Ctrl+O`,
the full-screen console). With no output the freed column stays blank.

## Why
Reported with two screenshots: hiding both panels showed the whole scrollback, hiding one
left half the screen blank while the output stayed crammed into the six-row band at the
bottom. The freed column is the obvious place for it, and it is what FAR does — hiding a
panel there reveals the console screen behind it.

Making the lone panel full height follows from the console no longer needing room underneath.
It does mean `Ctrl+P` changes the remaining panel's height; `heights` is untouched, so showing
the panel again restores the band exactly as it was.

The column console is drawn with `full = false`, so it gets no "PgUp/PgDn scroll" tag:
`PgUp`/`PgDn` still page the panel in this view, and offering a scroll hint that does not
work would be worse than no hint.

## Consequences
- After running a command, `Ctrl+P` is now a quick way to read the whole output without
  losing the panel — roughly 35 rows beside the panel instead of 6 under it.
- Output lines wrap at half the width, so long lines wrap where they did not before.
- Mouse selection works in the column, clamped to it rather than to the screen width.

## Where
`draw.go` (`drawAll`), `console.go` (`drawConsole`), `console_select.go` (`consoleMouse`,
`drawConsoleSelection`), `consolecol_test.go`, README "Panel control" and "Console view".
