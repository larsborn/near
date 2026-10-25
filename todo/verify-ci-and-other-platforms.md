# Verify CI runs and behaviour on Linux and macOS

## What
No GitHub Actions run has been looked at since the workflows were added. Everything was
developed and tested on Windows only; Linux and macOS code paths are exercised solely by
cross-compiling and by the fsnotify watcher test (which runs on Windows through the portable
backend).

## Why it matters
Linux is the primary target. Several subsystems have platform branches that have never run on
their platform: the fsnotify/inotify watcher (`watch.go`), the FreeDesktop trash
(`trash_unix.go`), the desktop-file context menu (`shellmenu_other.go`), process-group kill
(`platform_other.go`), the macOS `~/.Trash` and Finder actions.

## Status / findings
- `.github/workflows/ci.yml` runs gofmt, vet, tests on ubuntu/windows/macos and cross-compiles.
- Suspected weak spots: macOS kqueue watcher tests (one watch per directory, low default
  `ulimit -n`), `TestTrashPaths` on macOS (moves into the real `~/.Trash`), `TestWatcherNative`
  is Windows-only by name but the file has no build tag check for `newWatcher` semantics.

## How to check
```
gh run list --repo larsborn/near --limit 10
gh run view <id> --log-failed
```
Then run `go test ./...` on a real Linux box (or WSL) and try: Alt+F7 search with the watcher,
F8 to the trash, Shift+F10 context menu, Ctrl+C on a running console command.

## Acceptance
CI green on all three OSes; a manual session on Linux confirms watcher, trash, context menu
and console Ctrl+C; any platform-specific failures fixed or filed as their own task.
