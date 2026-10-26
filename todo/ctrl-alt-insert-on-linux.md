# Ctrl+Alt+Insert (copy_path) does not work on Linux

## What
The default binding `copy_path = ["Ctrl+Alt+Insert"]` (`config.go`, FAR's key for copying
full names) is reported not to work on the developer's Linux box (Ubuntu, gnome-terminal,
X11, xclip installed). It works on Windows.

## Findings so far (2026-10-26)
Everything inside NEAR and tcell is correct for the xterm-style sequence `ESC [ 2 ; 7 ~`:
- tcell v2.13.10 `input.go`: `{M:'~', P:2}` is KeyInsert and `calcModifier(7)` yields
  Ctrl|Alt, so the event is KeyInsert with ModCtrl|ModAlt.
- `keys.go` keyName renders that as `Ctrl+Alt+Insert`; `app.go` handleKey has no branch
  that swallows it (the Alt quick-search path only fires for runes); `doAction("copy_path")`
  calls `setClipboard`, which writes OSC 52 plus `xclip -selection clipboard`, and sets the
  status line to `Copied <path>`.
- VTE's keymap appends the modifier parameter for Insert and suppresses the meta-ESC
  prefix when it does, so gnome-terminal should send `ESC [ 2 ; 7 ~`.

So the failure is outside the process: the desktop or terminal swallows the chord, the
terminal drops the modifiers (sends plain `ESC [ 2 ~`, which is `select`), or tmux/screen
in between rewrites it. Not yet verified which.

## How to verify
In the same terminal (not inside NEAR) run `cat -v`, press Ctrl+Alt+Insert, press Enter:
- `^[[2;7~` → terminal is fine; then check whether NEAR shows `Copied …` in the status
  line and whether `xclip -o -selection clipboard` returns the path.
- `^[[2~` → modifiers dropped by the terminal/multiplexer.
- nothing → the chord is grabbed before the terminal (desktop keybinding, VM/remote
  desktop client, keyboard without a real Insert key).

## Options
1. Keep the FAR key and add a second default binding for `copy_path` that every terminal
   delivers. Candidates: `Alt+Shift+Insert` (FAR's "copy full names" in some versions),
   `Ctrl+Shift+Insert`, or a Ctrl+letter that is still free.
2. Document the terminal requirement in README and leave the default.
3. Both.

## Acceptance
`copy_path` has a default key that works in gnome-terminal on Linux; README's key list and
`actionHelp` (F1) mention it; `TestDefaultConfigParsesOnEveryPlatform` still passes;
a decision file records the choice if the default changes.
