# Optional right-aligned extension column like the user's FAR setup

## What
In the user's FAR configuration (screenshot compared on 2026-10-23) the Name column shows the
base name on the left and the extension right-aligned in a sub-column just before Size
(`args    c`, `README   md`). near shows `args.c` as one string.

## Why it matters
It was the last visible difference when matching the FAR panel look; the user did not ask for
it explicitly, it was offered ("say so if you want that too") and is still open.

## Where
`draw.go` `drawPanel`: the Name cell is printed with `fitLeft(name, nameW)`. An option
`align_extensions = true` in `config.go` would split `Entry.Name` via `Entry.Ext()` and print
the extension right-aligned inside the name column (FAR keeps a fixed-width ext area).

## Acceptance
Config option, off by default; with it on, the panel renders base name left and extension
right within the Name column, directories unaffected; `TestPanelLooksLikeFar` extended.
