# Ctrl+V pastes the system clipboard, NEAR's own clipboard is the fallback

Date: 2026-10-25
Status: accepted

## Decision
Ctrl+V and Shift+Insert read the system clipboard first (natively on Windows, through
pbpaste, wl-paste, xclip or xsel elsewhere) and insert its first line into the command
line, also when the line is empty and in the console view. Dialog inputs and the editor
paste from the same source. The text copied inside NEAR is used only when the system
clipboard cannot be read. A terminal OSC 52 reply lands on the command line when no dialog
is open.

## Why
The user wanted text copied in other programs to arrive in the console. NEAR's copy
actions also write the system clipboard, so reading it first is consistent.

## Consequences
Tests must switch the reader off (`a.readClip = nil` in testApp) so they never see the
developer's clipboard.

## Where
`clipboard.go` pasteText, `clipboard_windows.go`, `clipboard_other.go`, `app.go` handleKey.
