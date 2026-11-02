# Context menu: verify shell verbs that open UI, desktop menu on Linux/macOS

## What
Shift+F10, the keyboard Menu key and right-click open a context menu (`contextmenu.go`).
On Windows it is Explorer's own menu via `IContextMenu` (`shellmenu_windows.go`); elsewhere
`shellmenu_other.go` builds "Open with <app>" entries from `.desktop` files (Linux/BSD) or
Finder actions (macOS). NEAR's own actions are appended after a separator.

## Open points
- **Windows, verbs that show windows**: `InvokeCommand` is called with `hwnd = 0`.
  "Properties", "Open with…", "Send to → Mail recipient", "Share" and the 7-Zip dialogs
  were not all tried. Some verbs may require a window handle or a message loop; NEAR has no
  HWND (console) — `GetConsoleWindow()` could be passed as `hwnd` in `cmInvokeCommandInfoEx`.
- **Windows, Menu key**: the console never reports VK_APPS, so `menukey_windows.go` polls
  `GetAsyncKeyState` every 40 ms while NEAR's console window is in the foreground. Verify it
  fires exactly once per press under Windows Terminal and conhost and does not trigger when
  another window has the focus.
- **Linux**: `desktopAppsFor` parses `~/.local/share/applications`, `$XDG_DATA_DIRS` and
  `/usr/share/applications`, uses `xdg-mime query filetype` for the MIME type, and runs the
  `Exec=` line with `%f/%F/%u/%U` substituted. Not run on a real desktop yet; `mimeapps.list`
  default/associations ordering is ignored (plain alphabetical).
- **macOS**: only Open, Reveal in Finder, Quick Look, Get Info. No "Open with" list
  (would need LaunchServices).
- **Windows, a launched program can write over the screen**: `InvokeCommand` runs in NEAR's
  process, so a GUI program started from a context-menu verb can call
  `AttachConsole(ATTACH_PARENT_PROCESS)`, land on NEAR's console and scribble over the
  panels (measured with a probe; Electron editors do this). Enter on a file no longer has
  the problem — it goes through the detached `near -open` helper
  (`decisions/open-via-detached-helper.md`) — but the context menu still does. Options: run
  the whole invocation in a detached helper process (it would have to rebuild the menu
  there, so probably only for the plain "open" verb), or accept it and rely on Ctrl+R, which
  now repaints every cell.

## Acceptance
Each Windows verb above either works or fails with a visible status message; the Menu key
behaves as described; the Linux list is checked against a desktop with a few associations;
decisions about an "Open with" list on macOS recorded.
