# The viewer shows the beginning of a file too big to read; the editor still refuses

Date: 2026-11-07
Status: accepted

## Decision
F3 reads at most `viewerMaxBytes` (64 MB) of a file, or `viewerHexMaxBytes` (4 MB) when the
content is binary and will be shown as a hex dump, and displays that. The header shows
`4.0 M of 370 M` and the footer `only the first 4.0 M was read`. F4, the editor, keeps
refusing a file larger than 64 MB, now saying so before reading it rather than after.

## Why
Pressing F3 on a 370 MB video said only "… is larger than 64 MB", which helps nobody: the
point of looking at such a file is usually the first few bytes, a header or a magic number.
Reading the beginning costs nothing; that file now opens in 250 ms over a network drive. The
editor cannot do the same, because saving writes the whole buffer back and would truncate
the file to the part that was read.

## Consequences
A truncated view can end in the middle of a line or a rune. The hex limit is a quarter of
the text one because a dump is about four times the size of what it shows, and 4 MB is
already 262,144 lines. Both limits are variables so tests can lower them instead of writing
files of tens of megabytes. The compare viewer keeps refusing big files: half a diff would
mislead.

## Where
`viewer.go` (`newFileViewer`, the header and footer), `editor.go` (`newEditor`),
`viewer_test.go`.
