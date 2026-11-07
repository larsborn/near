# Folder shortcuts live on Ctrl+D and are stored in shortcuts.toml

Date: 2026-11-07
Status: accepted

## Decision
Ctrl+D opens a numbered list of directories: 1 to 9 or Enter go to one, Ins adds the
panel's directory under its base name, Del removes the one under the cursor. The list is
`cfg.Bookmarks`, the same entries the locations dialog shows, and edits are written to
`shortcuts.toml` next to the config, which `loadConfig` then prefers over the `[[bookmarks]]`
of config.toml. `unselect_all` moved from Ctrl+D to Ctrl+Shift+A, beside `select_all` on
Ctrl+A.

## Why
Requested as a quick way to reach a few working directories. Reusing the existing bookmarks
means one list rather than two that drift apart. Writing to a separate file follows the rule
set for menus: config.toml keeps the user's comments and layout and is never rewritten.
Adding happens without asking for a name, because the point is a single keystroke; the base
name labels it and the full path is shown beside it.

## Consequences
After the first edit, bookmarks added to config.toml have no effect until shortcuts.toml is
deleted. A shortcut cannot be renamed from inside NEAR, only removed and added again or
edited in the file. Users whose config.toml lists `unselect_all = ["Ctrl+D"]` keep that
binding and will not get the shortcuts list until the line is changed; the developer's live
config was updated.

## Where
`shortcuts.go`, `config.go` (loadConfig, the default `[keys]` and the bookmarks comment),
`app.go` (the shortcuts action), `shortcuts_test.go`.
