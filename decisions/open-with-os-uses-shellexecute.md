# Opening a file with its program goes through ShellExecuteW on Windows, xdg-open/open elsewhere

Date: 2026-10-24
Status: accepted

## Decision
Enter on a file (and "open" from the context menu) calls ShellExecuteW with a UTF-16 path
on Windows. The earlier `rundll32 url.dll,FileProtocolHandler` route is not used because
it is ANSI-only. Linux uses xdg-open, macOS open.

## Why
A path with full-width punctuation (`S:\2 Engel ＆ Charlie； FiNCH\…\cover.jpg`) did not
open the default program.

## Where
`platform_windows.go` osOpen, `platform_other.go` osOpen.
