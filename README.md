# near

A keyboard-driven dual pane file manager for the terminal in the spirit of
FAR Manager. One static binary; runs on Linux, macOS and Windows.

```
╔═ 1:src ═╤ 2:home ═════════════════════════════╗╔═ 1:build ═══════════════════════════════════╗
║ ~/src                                          ║║ ~/build                                      ║
║n            Name            │ Size  │   Date   │║n           Name           │ Size  │   Date   │║
║..                           │     Up│          │║..                         │     Up│          │║
║near                         │ Folder│2026-10-23│║out                        │ Folder│2026-10-23│║
║notes.txt                   *│  1.2 K│2026-10-23│║near-linux-amd64           │  5.9 M│2026-10-23│║
╟─────────────────────────────┴───────┴──────────╢╟───────────────────────────┴───────┴──────────╢
║ notes.txt  1,234 bytes  2026-10-23 09:30       ║║                                              ║
║ ✎ dual pane file manager, Go + tcell           ║║                                              ║
╚════════ Bytes: 1.2 K, files: 1, folders: 1 ════╝╚═══════ Bytes: 5.9 M, files: 1, folders: 1 ═══╝

~/src>git log --oneline near
 1Help  2Menu  3View  4Edit  5Copy  6Move  7MkDir 8Del   9Menus 10Quit
```

## Features

- **Two panels with tabs.** `Ctrl+T` new, `Ctrl+W` close, `Ctrl+PgUp`/`Ctrl+PgDn` or
  `Alt+Left`/`Alt+Right` switch, `Alt+1`..`Alt+9` jump. Panels look like FAR's:
  `Name │ Size │ Date │ Time` with centred titles, the sort mode as a letter at
  the left of the header, `Up` and `Folder` in the size column, a centred
  `Bytes: …, files: …, folders: …` footer.
- **Command line** at the bottom that always has the focus. Printable keys type
  into it; `Tab` still switches panels and the cursor keys still move the
  panel. With text in it, `Enter` runs it in the active panel's directory,
  `Esc` clears it. `Ctrl+Enter` (or `Ctrl+J`) inserts the current entry's name,
  `Alt+Enter` its full path. `cd <dir>` moves the panel instead of spawning a
  shell. `Ctrl+E`/`Ctrl+X` step through the history.
- **Esc goes to the parent directory.** Every key is configurable.
- **Everything-like search** (`Alt+F7` / `Ctrl+F`): an in-memory index of every
  file name below `search_roots`, built in the background, cached between runs
  and kept current by a filesystem watcher. Words match the name, a term with
  `/` matches the path, `*` and `?` are wildcards. `Enter` jumps to the file,
  `Ctrl+T` opens it in a new tab.
- **Quick search by holding Alt**: `Alt+p`, `Alt+h`, `Alt+o` moves the cursor to
  the first name starting with "pho"; keep typing with or without Alt,
  `Up`/`Down` step through matches, `Enter` opens the match. `Ctrl+S` starts it too.
- **Built-in editor** (`F4`, `Shift+F4` for a new file) modelled on FAR's: `F2`
  save, `Shift+F2` save as, `F7` find, `Shift+F7`/`Alt+F7` next/previous,
  `Ctrl+F7` replace, `Alt+F8` go to line, `Ctrl+Z`/`Ctrl+Shift+Z` undo/redo,
  `Ctrl+Y` delete line, `Ctrl+K` delete to end of line, `Shift`+movement
  selects, `Ctrl+C`/`Ctrl+X`/`Ctrl+V`, `Insert` toggles overwrite, `F1` lists
  the keys. CRLF and a UTF-8 BOM are preserved. `Ctrl+F4` uses the external
  editor; `internal_editor = false` makes that the default.
- **Mass rename** (`Ctrl+F6`): regex find/replace with `$1` groups, a `{n}`
  counter with start/step/padding, `{name}` `{ext}` `{date}` `{time}` `{parent}`
  tokens, case conversion and a live preview that flags conflicts. `F4` edits
  the name list in the editor instead. Swaps and chains are handled safely.
- **Notes on files** (`Ctrl+N`): stored in a hidden `.near-notes` JSON file per
  directory, shown below the listing, flagged by `*` in the name column, and
  following renames, copies and moves made within near.
- **Quick launch menu** (`F2`) and any number of **custom menus** (`F9` lists
  them, each may have its own key, menus nest). Items run shell commands with
  placeholders for the current file, the selection and both directories.
- **Histories**, FAR's keys: `Alt+F8` commands (Enter runs, Shift+Enter edits,
  Delete removes), `Alt+F12` folders (Enter goes, Ctrl+T new tab, Shift+Enter
  other panel), `Alt+F11` viewed and edited files (Enter views, F4 edits,
  Ctrl+Enter jumps to it). Searchable by typing, kept across sessions.
- **Compare** (`Shift+F3`): mark the entries that differ between the panels
  (missing, different size, newer; optionally by content) or show the current
  file of each panel side by side with changed, added and removed lines
  coloured and `n`/`p` to jump between differences.
- **Panel control**, FAR's keys: `Ctrl+O` hides both panels, `Ctrl+P` the
  inactive one, `Ctrl+F1`/`Ctrl+F2` left/right; `Ctrl+Up`/`Ctrl+Down` make
  both panels shorter/taller, `Ctrl+Shift+Up`/`Ctrl+Shift+Down` the current
  one, `Ctrl+Left`/`Ctrl+Right` move the divider (while the command line is
  empty). `Ctrl+U` swaps panels. Sizes are kept across sessions.
- **Copy path** (`Ctrl+Alt+Insert`): the current entry's full path, or the
  directory when `..` is selected, to the clipboard via the terminal (OSC 52)
  and a native helper where available (`clip.exe`, `pbcopy`, `wl-copy`,
  `xclip`, `xsel`).
- Also: selection by glob or regex (`+` `-` `*`), sort menu (`Ctrl+F12`),
  hidden files (`Ctrl+F10`), locations dialog with drives, mounts, bookmarks
  and open tabs (`Alt+F1`), go to path (`Ctrl+G`), viewer with hex mode (`F3`),
  copy/move/mkdir/delete with progress and cancel (`F5`–`F8`), session
  restore, and `F1` with the full key reference generated from your config.

## Install

Download the archive for your platform from the
[releases page](../../releases), unpack it and put `near` (or `near.exe`)
on your `PATH`. Or build from source:

```sh
go build -o near .
```

## Run

```
near                      # restores the last session, or cwd left and $HOME right
near /srv/data ~/projects # explicit left and right directories
near -config ./my.toml    # alternative config file
near -reindex             # force a rebuild of the search index at start
near -defaults            # print the default config to stdout
near -version
```

## Configuration

Written on first start with every default and a comment for each setting:

| Platform | Path |
| --- | --- |
| Linux | `~/.config/near/config.toml` |
| macOS | `~/Library/Application Support/near/config.toml` |
| Windows | `%APPDATA%\near\config.toml` |

The same directory holds `state.json` (session, histories, panel sizes) and
`index.gob` (search cache). `near -defaults` prints the current defaults,
useful after an upgrade changed the default keys.

### Keys

```toml
[keys]
up_dir = ["Esc", "Backspace"]     # several keys per action are fine
mass_rename = ["Ctrl+F6", "Alt+F6"]
quit = ["F10", "Ctrl+Q"]
```

Key names: `F1`..`F12`, `Shift+F6`, `Ctrl+X`, `Alt+X`, `Ctrl+Alt+Insert`,
`Esc`, `Enter`, `Tab`, `Shift+Tab`, `Space`, `Insert`, `Delete`, `Backspace`,
`Home`, `End`, `PgUp`, `PgDn`, arrows, or a plain character. Only the actions
you list are changed; the rest keep their defaults.

Characters bound to actions (`+`, `-`, `*`, `Space`) trigger the action only
while the command line is empty; otherwise they are typed. `Alt+<character>`
starts a quick search unless that combination is bound, so the defaults keep
Alt+letter free. `Ctrl+Enter` needs a terminal that reports modified Enter
(xterm modifyOtherKeys, kitty keyboard protocol, Windows Terminal); `Ctrl+J`
works everywhere. `command_wait = false` returns to the panels right after a
command finishes instead of waiting for Enter.

### Menus

```toml
[[menus]]
name = "Quick launch"      # the first menu is opened by the quick_launch action (F2)
key = "F2"                 # optional direct key

  [[menus.items]]
  title = "Git status"
  hotkey = "g"
  command = "git status"   # runs in the current directory, waits for Enter afterwards

  [[menus.items]]
  title = "Open in file browser"
  hotkey = "o"
  command = "xdg-open ."
  detach = true            # run in the background, stay in near

  [[menus.items]]
  title = "Tools..."
  hotkey = "t"
  submenu = "Tools"        # opens another [[menus]] entry by name
```

Placeholders in `command` and `cwd` are quoted for the shell:

| Placeholder | Value |
| --- | --- |
| `{file}` / `{path}` | current entry name / full path |
| `{files}` / `{paths}` | selected names / full paths (current entry if none selected) |
| `{dir}` / `{other}` | this panel's / the other panel's directory |
| `{name}` / `{ext}` | current name without extension / extension without dot |

Item flags: `nowait` (no "press Enter" pause), `detach` (background, for GUI
apps), `confirm` (ask first), `cwd` (working directory). The same placeholders
work on the command line.

### Search

```toml
search_roots = ["~", "/srv/data"]
search_exclude = [".git", "node_modules", "/proc", "/sys"]  # name anywhere, or an exact path
search_max_results = 500
search_index_on_start = true
search_watch = true            # keep the index current while near runs
search_watch_max_dirs = 100000 # cap for per-directory watches (Linux, macOS)
```

The index maps each directory to its names, so a change is a small update
rather than a rescan. Changes arrive through a watcher:

| Platform | Mechanism | Scope |
| --- | --- | --- |
| Windows | `ReadDirectoryChangesW` with subtree watching | one handle per root, whole tree |
| Linux | inotify via fsnotify | one watch per directory, capped |
| macOS, BSD | kqueue via fsnotify | one watch per directory, capped (needs a high `ulimit -n`) |

Creates, deletes and renames show up within about a third of a second. If the
system reports lost events the index is rebuilt. On Linux, raise
`fs.inotify.max_user_watches` (about 1 KB of kernel memory per directory) and
`search_watch_max_dirs` to cover large trees; beyond the cap, changes in
unwatched directories only show up after the next rebuild (`Ctrl+R` in the
search dialog or the next start). The search dialog's status line shows what
is being watched.

### Notes file

`.near-notes` in each directory, a JSON object from entry name to note text:

```json
{
  "report.pdf": "sent to accounting 2026-10-20",
  "src": "main project\nbuild with make"
}
```

Set `notes_file` in the config to use a different name.

## Development

```sh
go test ./...                           # headless UI tests via tcell's simulation screen
go vet ./... && gofmt -l .
GOOS=linux  GOARCH=amd64 go build -o near-linux-amd64 .
GOOS=darwin GOARCH=arm64 go build -o near-darwin-arm64 .
```

CI runs vet and the test suite on Linux, Windows and macOS for every push.
Pushing a tag such as `v0.0.1` builds the release archives for all platforms
and publishes them on the GitHub release for that tag.
