# Shift+Up/Down/Home/End/PgUp/PgDn select entries; Ctrl+Shift+Left/Right select words in inputs

Date: 2026-10-23
Status: accepted

## Decision
In the panels, Shift with a movement key toggles the selection of the entries passed over
(FAR's selection keys), Insert and Space toggle the current entry, `+`/`-`/`*` select by
pattern. In every input field Shift+Left/Right/Home/End extend a selection and
Ctrl+Shift+Left/Right select by word; Ctrl+C/Ctrl+X/Ctrl+V and Ctrl+Insert/Shift+Insert
copy, cut and paste; typing replaces a selection.

## Why
Requested ("Shift+Up/Down should select files", "POS1 and END should work with Shift",
"Ctrl+Shift+Left/Right in input boxes doesn't select the current word").

## Where
`app.go` select_* actions, `dialogs.go` LineEdit, `config.go` select_to_* keys.
