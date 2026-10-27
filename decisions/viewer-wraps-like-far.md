# The viewer wraps at words by default and marks cut lines with « » like FAR's

Date: 2026-10-27
Status: accepted

## Decision
F3's viewer starts in word-wrap mode for text, as FAR's viewer does; F2 toggles wrapping
and Shift+F2 switches between word and character wrapping. With wrapping off, a line that
continues beyond the right edge ends in `»` and, once scrolled sideways, starts with `«`.
The header carries FAR's right-hand fields: mode letter, encoding, size, `Col N`, position
in percent. Hex dumps never wrap.

## Why
The user showed FAR's viewer with its `»` marks and asked for them and for a word-wrap
switch; FAR's defaults were adopted with them.

## Where
`viewer.go` (wrapText, Draw), `viewer_test.go`, README "Viewer".
