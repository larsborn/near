# Home and End always edit the text box; a list beside it takes them with Ctrl

Date: 2026-11-12
Status: accepted

## Decision
Wherever something can be typed into, `Home` and `End` move the caret to the start and end of
that text — in every input dialog, in the command line, and in the two multi-line editors
(where they go to the start and end of the *current line*, not of the whole text).

When a dialog shows a list next to its text box, the list gets first/last on `Ctrl+Home` and
`Ctrl+End` instead. That applies to the search dialog (`Ctrl+F`), which was the one place
where `Home` and `End` moved the result cursor and so never reached the "Find:" box.

A list with no text box — menus, histories — keeps plain `Home`/`End` for first/last item, and
the viewer keeps them for scrolling. The panel's own `home`/`end` actions are unchanged; the
command line already took the editing keys off them while it holds text.

## Why
Reported as a bug: "in boxes like 'Find: ' the buttons Home and End on the keyboard should
work". They did work in every box but the search dialog, whose `HandleKey` matched `Home` and
`End` ahead of handing the key to its `LineEdit`.

Which half should win is a real choice, because that dialog is both a text field and a list.
The text box wins because it is where the caret is and where every printable character already
goes, which is the same rule the command line follows (`command-line-always-focused.md`); a
caret that ignores Home is wrong in a way that a list needing a modifier for first/last is
not. The list keeps `Up`/`Down`/`PgUp`/`PgDn`, which is how results are actually walked.

`Ctrl+Home`/`Ctrl+End` was chosen for the list because the viewer already uses those names, so
`keyName` is known to produce them. On a terminal that does not report the modifier with those
keys, the list loses only first/last.

## Consequences
- `Home`/`End` in the search dialog no longer jump to the first/last result. Anyone used to
  that needs `Ctrl+Home`/`Ctrl+End`; the help screen and the README now say so.
- `TestHomeAndEndWorkInEveryTextBox` drives all seven single-line boxes (command line, input
  dialog, search, find file, copy, apply command, mass rename) and
  `TestHomeAndEndInMultiLineEditors` the editor and the notes text area, so a new dialog that
  swallows these keys ahead of its field is caught. A new text box should be added to that
  table.
- `Shift+Home`/`Shift+End` keep selecting to the start and end of the text, since those reach
  `LineEdit` through the same default branch.

## Where
`searchdialog.go` (`SearchDialog.HandleKey`), `dialogs.go` (`LineEdit.HandleKey`,
`TextDialog.HandleKey`), `editor.go`, `app.go` (the command line's editing keys and the help
screen), `homeend_test.go`, README "Everything-like search".
