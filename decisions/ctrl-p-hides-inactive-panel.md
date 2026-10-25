# Ctrl+P hides the inactive panel, not the right one

Date: 2026-10-23
Status: accepted

## Decision
Ctrl+P toggles the panel that does not have the focus. Ctrl+F1 and Ctrl+F2 toggle the
left and right panel by position. A hidden panel keeps its width; the remaining one does
not grow to full width.

## Why
This is FAR's definition of Ctrl+P. The user first asked for "the right one" and then
chose to stick with FAR.

## Consequences
Hiding the inactive panel while the right panel is active hides the left one.

## Where
`app.go` doAction `toggle_inactive_panel`, README "Panel control".
