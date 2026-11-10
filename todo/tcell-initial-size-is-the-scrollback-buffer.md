# tcell on Windows sizes its first cell buffer to the scrollback, not the window

## What
On Windows, `tcell.NewDevTty` (`tty_win.go`, v2.13.10) takes the terminal's initial size from
`GetConsoleScreenBufferInfo`'s `dwSize` — the **screen buffer**, i.e. the scrollback — rather
than from `srWindow`, the visible window:

```go
_, _, _ = procGetConsoleScreenBufferInfo.Call(uintptr(w.out), uintptr(unsafe.Pointer(&w.oscreen)))
w.rows = uint16(w.oscreen.size.y)   // dwSize.Y: 9001 with cmd's default scrollback
w.cols = uint16(w.oscreen.size.x)
```

`w.rows` is only corrected later, when a `resizeEvent` (WINDOW_BUFFER_SIZE_EVENT) arrives. By
then `tScreen.Init` has already called `resize` → `CellBuffer.Resize(120, 9001)`.

## Why it matters
That is **1.08 million cells, about 200 MB**, allocated and immediately thrown away at every
start-up. Measured on 2026-11-10 with a heap profile taken right after `App.setup`:

```
screen 120x30  heapAlloc=1 MB  sys=214 MB
alloc_space:  199.38MB  97.07%  tcell.(*CellBuffer).Resize
                                 <- tcell.(*tScreen).resize
                                 <- tcell.(*tScreen).Init
                                 <- main.main
gc 1 @0.003s  199->200->198 MB
```

With a 50-line console buffer (`mode con: lines=50`) the same start-up is **1 MB live, 4 MB
total**, which confirms the cause: the cost is proportional to the scrollback height.

The heap is garbage straight away, but an idle file manager never allocates enough to trigger
a second collection, so the pages stayed charged to the process — NEAR idled at 225–312 MB
with no index at all.

## Status
Worked around on our side (2026-11-10): `main.go` calls `releaseMemory()` once after
`app.setup`, which collects and returns the pages (`memory.go`,
`decisions/index-memory-is-returned-to-the-system.md`). Idle with no index is now ~17 MB.

The workaround is cheap and correct, but it means NEAR still *touches* 200 MB of pages during
start-up, which costs a little time and page-table churn.

## What is left
- **Report it upstream.** This looks like a plain tcell bug: `NewDevTty` should use `srWindow`
  (`right-left+1`, `bottom-top+1`) like `WindowSize` consumers expect, matching what the
  resize event later reports. Not filed yet — ask before opening an issue on
  `github.com/gdamore/tcell`. We are on the latest release (v2.13.10), so there is no version
  to upgrade to.
- **Do not "fix" it by shrinking the console buffer.** Calling `SetConsoleScreenBufferSize` to
  match the window before `screen.Init()` would avoid the allocation, but NEAR hands the real
  console to external programs (`runShell`, `interactive_commands`, Shift+Enter), and those
  would lose their scrollback. Rejected.
- If upstream fixes it or we vendor a patch, the `releaseMemory()` call in `main.go` can stay
  anyway — it is harmless and covers other start-up transients.

## Acceptance
Either an upstream fix released and picked up, or a decision that the workaround is permanent
and this file is deleted.
