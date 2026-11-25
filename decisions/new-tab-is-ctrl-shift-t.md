# A new tab is Ctrl+Shift+T, and everything that opens one follows that binding

Date: 2026-11-25
Status: accepted

## Decision
`new_tab` defaults to `Ctrl+Shift+T` instead of `Ctrl+T`, and it is the only place the key is
written down. The three dialogs that can open something in a new tab now ask for the bound
key rather than hardcoding one:

- the search dialog's key handler, its context-menu entry and its footer hint,
- the folders history's key handler and its hint,
- the F1 help text.

They use `a.keyFor("new_tab")`, which Go allows as a non-constant `case` expression, so
rebinding `new_tab` in the config moves every one of them together.

`Ctrl+T` is left unbound rather than kept as a second binding: the request was that `Ctrl+T`
should *not* open a tab, so keeping it would defeat the point.

`actionHelp`'s one-line description for `folder_history` no longer names the key ("the
new-tab key opens a tab"), because that table is static and would otherwise go stale.

## Why
Asked for directly. Hardcoding the key in four more places was the existing state and made
this a four-file change for one binding; routing them all through `keyFor` makes the next
rebinding a one-line edit, and the hints can no longer disagree with what the keys do.

## Consequences
- **`Ctrl+Shift+<letter>` needs a terminal that reports the modifier.** tcell asks for
  modifyOtherKeys / kitty / win32 input, and `Ctrl+Shift+A` has been a default binding
  (`unselect_all`) for a while, so the pipeline is proven. On a terminal that collapses
  `Ctrl+Shift+T` to `Ctrl+T`, new-tab is unreachable until `new_tab` is rebound — there is no
  fallback binding by design, unlike `Ctrl+Enter`/`Ctrl+J`.
- **Some terminal emulators keep `Ctrl+Shift+T` for themselves** (GNOME Terminal and konsole
  reopen a closed tab with it); there NEAR never sees the key and `new_tab` has to be rebound.
  conhost and Windows Terminal pass it through.
- The developer's live config bound `new_tab` explicitly, so that one line was changed too
  (`live-config-not-regenerated.md`); anyone else's config keeps whatever it names, since
  BurntSushi merges `[keys]` per action.
- Tests that pressed `Ctrl+T` now press `Ctrl+Shift+T`; `eventFor` already built it correctly
  (`Ctrl`+`Shift` stripped to `T`, sent as `KeyCtrlT` with `ModShift`).

## Where
`config.go` (the `new_tab` default), `searchdialog.go` (`HandleKey`, `showMenu`, the footer),
`history.go` (`OnKey`, the hint), `app.go` (the F1 text), `keys.go` (`actionHelp` wording),
`app_test.go`, `searchkeys_test.go`, README "Two panels with tabs".
