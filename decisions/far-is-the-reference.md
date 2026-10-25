# FAR Manager is the reference for keys, dialogs and looks

Date: 2026-10-23
Status: accepted

## Decision
Where NEAR has a FAR counterpart, it matches FAR: default key bindings come from FAR's
help, dialogs copy FAR's wording and layout (double frame, centred titles, `{ Default }`
button in braces, yellow hotkey letters), panels use FAR's columns (`Name │ Size │ Date │
Time`, sort letter at the left of the header, `Up`/`Folder` in the size column, the
`Bytes: …, files: …, folders: …` footer). Dialogs are not full width; the FAR screenshots
only looked that way. Screens are drawn in one pass (`drawAll` then a single `Show`) so
boxes do not flicker. The user supplies FAR screenshots and FAR's help source
(FarEng.hlf.m4 on GitHub) is consulted when behaviour is unclear. Deliberate deviations get
their own decision file.

## Why
The user knows FAR and wants NEAR to feel the same, on Linux first.

## Consequences
New features start from "how does FAR do it". `TestPanelLooksLikeFar` guards the panel
layout; `TestCopyDialogLooksLikeFar` the copy dialog.

## Where
`draw.go`, `dialogs.go` drawBox, `copydialog.go`, `findfile.go`, `usermenu.go`, README.
