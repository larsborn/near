# Alt+F7 is FAR's Find file; the index search moved to Ctrl+F

Date: 2026-10-25
Status: accepted

## Decision
Alt+F7 opens FAR's "Find file" dialog: file masks, text or hex content with case sensitive,
whole words and not containing, folders as results, symbolic links, and the search area
(current folder, current folder only, selected folders, drive root). The Everything-like
index search is bound to Ctrl+F only. Code pages, archives, alternate streams, Drive,
Filter and Advanced from FAR's dialog are left out because NEAR has nothing behind them.

The results replace the current panel: entry names are paths relative to the panel's
directory, so every operation that joins `Tab.Path` and `Entry.Name` works on them. Esc or
`..` returns to the directory; changing directory leaves the results. Copies of results
land under the bare file name.

## Why
The user wanted FAR's dialog on FAR's key and the results in the pane rather than a list.
Relative-path names make the results usable everywhere with one representation.

## Consequences
The user's live config was rebound (`find_file = ["Alt+F7"]`, `search = ["Ctrl+F"]`).
Notes are not shown in a results panel.

## Where
`findfile.go`, `model.go` Tab.Find, `app.go` loadTab/goUp/navigate, `actions.go` copy and
rename, README "Find file".
