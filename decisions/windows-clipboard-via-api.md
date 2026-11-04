# On Windows the clipboard is written through the API, never through clip.exe

Date: 2026-11-04
Status: accepted

## Decision
`setClipboard` writes the system clipboard with `SetClipboardData(CF_UNICODETEXT)` on
Windows and falls back to no helper program at all; `clip.exe` was removed from the
candidate list. Both reading and writing retry opening the clipboard for about 100 ms.
Other platforms keep `pbcopy`, `wl-copy`, `xclip` and `xsel`, which take UTF-8 correctly.

## Why
`clip.exe` decodes its standard input in the console's OEM code page, so copying
`…\Widerspruch und Löschanfrage_Wallenborn.docx` put `L├╢schanfrage` on the clipboard. Keeping
it as a fallback would mean silently mangling non-ASCII text whenever the API call failed,
which is worse than copying nothing. Only one process may hold the clipboard at a time and
clipboard managers take it often, which showed up as an intermittent "nothing there" when
reading right after writing; hence the retry on both sides.

## Consequences
One more piece of Windows-only API code. If the clipboard stays locked for 100 ms the text
is still in NEAR's own clipboard and was offered to the terminal over OSC 52.

## Where
`clipboard_windows.go` (`writeClipboard`, `openClipboardWaiting`), `clipboard.go`
(`setClipboard`, `clipboardWriter`), `clipboard_windows_test.go`.
