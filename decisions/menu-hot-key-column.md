# Menus show the hot key in a column of its own

Date: 2026-10-26
Status: accepted

## Decision
Lists drawn as menus (`ListDialog.Menu`) print the hot key in a four-cell column before the
label instead of highlighting a letter inside the label. Lists without any hot key, and
non-menu lists such as the sort menu, keep the plain layout.

## Why
That is how FAR's user menu looks ("L    Compile Latex").

## Where
`dialogs.go` ListDialog.Draw.
