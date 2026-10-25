# Holding Alt while typing is the quick search; Enter opens the match at once

Date: 2026-10-23
Status: accepted

## Decision
An Alt+character that is not bound starts a quick search in the active panel; further
characters with or without Alt extend it, Up/Down step through matches, Esc leaves, and
Enter ends the search and opens the match immediately (no confirmation). Ctrl+S starts a
quick search too. Because bound Alt combinations win, default bindings avoid Alt+letter.

## Why
FAR behaviour, requested as "holding ALT while typing should move the cursor to a matching
name". The confirmation step that first appeared on Enter was removed on request.

## Consequences
Alt+1..Alt+9 switch tabs, so a quick search cannot start with a digit (open point in
`todo/small-open-points.md`).

## Where
`app.go` handleQuickSearch, `config.go` quick_search, README "Quick search by holding Alt".
