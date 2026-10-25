# Tab in the console view completes paths; an ambiguous Tab does nothing until pressed twice

Date: 2026-10-25
Status: accepted

## Decision
With the panels hidden, Tab completes the path at the end of the command line. A unique
match is filled in without a trailing separator (cmd.exe style). With several matches the
first Tab leaves the line alone and a second Tab on the same text lists the candidates in
the console. With the panels visible Tab still switches panels.

## Why
Requested as "complete to C:\Windows; if not unique nothing should happen, hitting tab
twice should list all options". No common-prefix completion, by the user's wording.

## Consequences
Completing a directory and continuing into it needs the separator typed by hand.

## Where
`complete.go`, `app.go` handleKey console-view block, README "Console view".
