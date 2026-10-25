# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`near` is a FAR Manager–style dual pane file manager for the terminal, written in Go on
`github.com/gdamore/tcell/v2`. Single `package main`, no subpackages. It targets Linux first
but builds and is developed on Windows.

## Git workflow

**Always commit and push when a piece of work is done** — do not leave changes sitting in the
working tree. Run `gofmt -l .`, `go vet ./...` and `go test ./...` first, commit with a message
that says what changed for the user, then `git push`. Releases are separate: they
are cut only when asked, by tagging `vX.Y.Z` and pushing the tag (see below).

## Open tasks — `todo/`

`todo/` is the durable memory of open tasks and unresolved questions — one Markdown file
per task (kebab-case filename). Treat it as the work queue; check it when picking up work.

- **Write tasks here autonomously.** Whenever an open question, deferred fix, or follow-up
  arises that isn't being done right now, create `todo/<slug>.md` for it — don't rely on it
  living only in the conversation.
- **Each file must be self-contained.** Assume the reader has *none* of the originating
  conversation's context: state what it is, why it matters, current status/findings, the
  relevant files/commands, options already considered, and clear acceptance criteria.
- **Delete a task file when the task is resolved** — don't leave stale "done" markers; git
  history preserves it. Keep files updated as understanding changes.
- **Remind the user of open tasks at the start of a new conversation** — briefly list
  what's in `todo/` so nothing is silently forgotten.
- **These are pre-backlog notes**, not a formal backlog. Resolving a task means either
  *doing* it or *promoting* it — e.g. moving it into a real backlog once one exists (issue
  tracker, project board). If a backlog is introduced, migrating a `todo/` file there (and
  deleting it here) is a valid resolution.

## Commands

```sh
go build -o near.exe .                               # Windows build (dev box)
GOOS=linux  GOARCH=amd64 go build -o near-linux-amd64 .
GOOS=darwin GOARCH=arm64 go build -o /tmp/near-mac . # keep the macOS build compiling
go vet ./... && gofmt -l .                           # both expected to be clean
go test ./...                                        # whole suite (~7 s, includes live watcher tests)
go test -run 'TestEditorBasics|TestWatcherNative' -v .   # single tests
./near.exe -defaults > config.toml                   # print the current default config
./near.exe -config ./some.toml                       # run against an alternative config
```

CI (`.github/workflows/ci.yml`) runs gofmt/vet/tests on Linux, Windows and macOS and
cross-compiles every release target. A `v*` tag triggers `release.yml`, which builds archives
for linux/darwin/windows/freebsd with `-X main.version=<tag>` and publishes them with
`gh release create`. Releases are cut by tagging: `git tag -a v0.0.2 -m ... && git push --tags`.

The developer's live config is `%APPDATA%\near\config.toml`. Whenever a default key binding
changes it has been regenerated with `near -defaults` (after checking it was still the
untouched generated file); `config.toml.bak` next to it is the first version.

## Architecture

**Event loop and key routing** (`app.go`). `App.run` draws, polls one tcell event, handles it.
`handleKey` has a strict precedence that every new feature must respect:

1. If any dialog is open, the top of `App.dialogs` gets the key. Nothing else runs.
2. Active quick search (`qsActive`) consumes printable keys, Alt+char, Backspace, Up/Down, Esc, Enter.
3. An unbound Alt+char starts a quick search (FAR behaviour). Bound Alt combinations win, which is
   why default bindings avoid Alt+letter.
4. The command line is always focused: with text in it, Enter/Esc/editing keys go to it; printable
   characters go to it unless the line is empty **and** the character is bound to an action
   (`+ - * Space`).
5. Otherwise `keymap[keyName(ev)]` → `doAction(name)`.

**Dialogs** are anything drawn over the panels: `Dialog` interface = `HandleKey(a, ev)` +
`Draw(a)`. Input, confirm, list (menus/histories), text (notes), mass rename, search, viewer,
diff viewer and the editor are all dialogs on one stack. Callbacks run after `a.pop()`, so a
callback may push the next dialog (menus → submenus, replace loop, save prompts). The
`ListDialog.OnKey` hook is how histories implement Delete/Ctrl+T/Shift+Enter.

**Actions** are strings. Adding one means touching three places: the default `[keys]` table inside
`defaultConfigTextFor` (`config.go`), the `switch` in `doAction` (`app.go`), and `actionHelp`
(`keys.go`, feeds the F1 screen). Menus from the config are bound as `menu:<name>`.
Key names are canonicalised by `keyName` (tcell event → "Ctrl+Shift+Up", "F5", "a") and
`normKey` (config string → same form); tests build events with `eventFor` in `app_test.go`, which
needs a case for each special key it is asked for (it silently sends a rune otherwise).

**Config** (`config.go`): the defaults *are* the TOML text in `defaultConfigTextFor` — it is parsed
to produce the default `Config`, then the user's file is decoded over it. BurntSushi/toml merges
maps (`[keys]`: unlisted actions keep defaults) but replaces slices (`[[menus]]`,
`search_exclude`). `TestDefaultConfigParsesOnEveryPlatform` guards the embedded text.

**Panels and drawing** (`model.go`, `draw.go`). `Panel` holds `Tab`s; a `Tab` is a directory
listing with cursor, selection map, sort, per-directory notes and a cursor-position memory.
`App.navigate` is the only way to change a tab's directory (it records folder history).
A tab with `Find != nil` (`findfile.go`, Alt+F7) lists search results instead: entry names are
paths relative to `Tab.Path`, so `filepath.Join(t.Path, e.Name)` stays valid everywhere; `loadTab`
re-checks the results instead of reading the directory, `navigate`/`goUp` leave the mode.
Directory reads go through `listDirWait` (`slowdir.go`): the listing runs in a goroutine, and
after `slowDirDelay` a "Reading directory…" box polls for Esc, which returns `errDirCancelled`
so `navigate`/`loadTab` keep the previous state. `listDirFn` is the hook tests use to slow it.
`draw.go` lays out: two panels (`panelChrome` = 7 fixed rows per panel, `bottomRows` = 3 for
status, command line, key bar), hidden/resized panels (`hidden`, `hideAll`, `heights`, `split`),
then the dialog stack. The FAR-like column layout (centred titles, `│` separators, sort letter,
`Up`/`Folder`, centred footer) is matched by `TestPanelLooksLikeFar`.

**Copies and moves** (`transfers.go`) are `transfer`s on a single `transferQueue` whose one
worker goroutine runs them strictly in order; progress lives in the transfer under its mutex,
the worker posts `EventTransfer`/`EventTransferDone`, and `TransferDialog` is just a modal view
of that state (Esc cancels via an atomic flag, B pops the dialog and the transfer continues in
the background). The copy/move dialog (`copydialog.go`) returns `copyOptions` incl. `Background`.
Files are copied in `copyChunk` blocks so cancellation is checked mid-file.
**Other long operations** (delete, content compare) still run synchronously in the event loop
and call `a.progress(title, item, -1, -1)`, which redraws and returns false if Esc is queued.

**Shell, console and editor** (`commands.go`, `console.go`): commands typed at the command
line go through `runCommandLine` → `runInConsole`: started with stdout/stderr on a pipe, no
stdin, output fed to `Console` (SGR-aware line buffer) by a goroutine that posts
`EventConsole`/`EventCommandDone`; `hideAll` is switched on so `draw` shows the console instead
of the panels, and it stays on. In that view Tab is path completion (`complete.go`: unique
match filled in, second Tab on the same text lists candidates in the console). Ctrl+C → `killTree` (platform: taskkill /T or SIGTERM to the
process group). Programs in `interactive_commands`, Shift+Enter, menu items and the external
editor use `runShell` instead, which suspends tcell, runs via `shellCommand` (platform file;
`cmd /S /C` with raw `CmdLine` on Windows, `sh -c` elsewhere), optionally waits for Enter, resumes. `editFileThen(path, external, done)` is the single entry for editing — the
built-in `Editor` dialog calls `done` from its close callback, the external editor blocks, so
callers (mass rename's F4 flow) must not assume synchronous completion.

**Search index** (`search.go`, `watch*.go`). `Index.dirs` is `map[directory][]names` (every
directory is a key), chosen so watcher events are cheap to apply. Build walks `search_roots`
in a goroutine (`expandRoots`: `"*"` = every fixed drive from the platform `fixedDrives` in
`drives_*.go`, nested roots dropped); results are announced by posting `EventIndexDone`/`EventIndexChanged` to the
tcell screen (the loop only redraws). Watch backends implement `fsWatcher`: `watch_windows.go`
is a native recursive `ReadDirectoryChangesW` (one handle per root); `watch.go` wraps fsnotify
with one watch per directory, capped by `search_watch_max_dirs`. Events are batched, a create is
applied as remove+add (idempotent), overflow triggers a rebuild. On Windows the tests exercise
both backends (`TestWatcherNative`, `TestWatcherPortable`); the fsnotify path is what Linux runs.
Cache is a gob of the map in the config dir; an old-format cache is rejected and rebuilt.
Searching goes through `flatIndex` (`flatindex.go`): all names lowercased in one byte buffer
scanned with `bytes.Index`, plus parallel `off/dirIx/nameIx` arrays; the map stays the source of
truth (results are verified against it, so stale entries are skipped). The view is built after
`Build`/`LoadCache`, appended to in `apply` (map removals swap the last name into the hole and
re-append it), and compacted by a rebuild `flatCompactAfter` after the last change. Counting
stops at `searchCountCap`. On the developer's 5M-name index every query is < 50 ms.

**Editor** (`editor.go`): lines as `[][]rune`; every mutation goes through `edit(r1,c1,r2,c2,ins)`
which returns an `editRec`; undo/redo replay those records, consecutive typing is merged.

**Platform files** use build tags: `platform_windows.go`/`platform_other.go` (hidden attribute,
drives/mounts, open-with-OS, shell), `watch_windows.go`/`watch_other.go`.

## Testing approach

All UI tests are headless via `tcell.NewSimulationScreen`: `testApp(t)` builds a temp tree and an
`App`, `key(a, "Ctrl+F6")` / `typeText` feed `handleEvent` directly (no run loop), `screenText(a)`
dumps the rendered screen for assertions. To look at a layout change, write a throwaway test
that prints `screenText` and run it with `-run ... -v`. Watcher tests poll with `waitFor`.
A one-off test that loads the developer's real 5-million-entry cache was used for timing;
name searches are ~0.3 s there, which is why the search dialog only searches when no key is
queued (`SearchDialog.dirty` + `HasPendingEvent`).

## Gotchas

- Do not patch Go source through a bash heredoc/`sed`/`printf` when the text contains `\n` or
  `\"`: the escapes become real newlines (this broke `draw.go` once). Use the Edit/Write tools, or
  write the patch script to a file first.
- `tcell.Style.Decompose()` returns three values; take `_, bg, _ :=` before calling `.Background(bg)`.
- `keyName` reports Ctrl+H/I/M as Backspace/Tab/Enter; they cannot be bound separately.
- Ctrl+Enter only arrives as a distinct key on terminals with modifyOtherKeys/kitty/win32-input
  support (tcell enables them); Ctrl+J is the bound fallback.
- Build artifacts `near.exe`, `near-linux-*` live in the repo root and are git-ignored.
