# Verify behaviour on Linux and macOS by hand

## What
Everything was developed and tested on Windows. CI (`.github/workflows/ci.yml`) now runs the
suite on ubuntu, windows and macos and cross-compiles every target; the `v0.0.2` release was
built and published by `release.yml` on 2026-10-25.

## Status / findings (2026-10-25)
- ubuntu and macos jobs pass, including the fsnotify watcher, FreeDesktop trash and
  process-group tests. The windows job failed on three consecutive pushes in
  `TestWatcherNative` ("timed out waiting for fresh.log indexed"): the native watcher armed
  its first `ReadDirectoryChangesW` only inside the reader goroutine, so a change made right
  after `Add` could be lost. Fixed by arming with overlapped I/O before `Add` returns
  (`watch_windows.go`). Confirm the next CI runs stay green.
- Still never exercised on a real Linux/macOS desktop: the desktop-file context menu
  (`shellmenu_other.go`), `xdg-open`, the kqueue watcher under a real `ulimit -n`, Ctrl+C
  process-group kill of a console command, the FreeDesktop trash on a second filesystem
  (`.Trash-<uid>`).

## How to check
```
gh run list --repo larsborn/near --limit 5
gh run view <id> --log-failed
```
Then a manual session on Linux (or WSL) and macOS: Alt+F7 search with the watcher, F8 to the
trash, Shift+F10 context menu, Ctrl+C on a running console command, mouse clicks.

## Acceptance
CI green on all three OSes for several pushes; a manual session on Linux and macOS confirms
the items above; platform-specific failures fixed or filed as their own task.
