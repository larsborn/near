# ZIP archives can be browsed but never written to

Date: 2026-11-10
Status: accepted

## Decision
Enter on a `.zip` opens it in the panel: `Tab.Zip` holds the archive path, the directory
inside it and the whole listing, read once. Inside, Enter walks directories, `..` and Esc
go up and finally out to the directory the archive is in, F3 shows a file through a copy in
a temp directory, Enter on a file opens it with its program through the same kind of copy,
and F5 extracts the selection (directories with everything under them) to the other panel.
Every action that would change something — rename, delete, create, move, mass rename, notes,
apply command, the context menu, find — is refused with "… is not possible inside an
archive". Only `.zip` is browsed; other zip-based formats such as `.docx` and `.jar` keep
opening with the program that owns them.

## Why
Asked for as "allow to navigate into ZIP files". Reading an archive is simple and safe with
the standard library; writing one means rebuilding it, which risks the user's data for a
feature nobody asked for. Refusing clearly beats letting an action run against a path like
`C:\x.zip\inner.txt`, which would fail in a confusing way or, worse, act on the archive
itself. Extraction and viewing are included because browsing an archive you cannot get
anything out of is half a feature, and both share one extraction helper.

## Consequences
A temporary copy opened with Enter is not written back, and the status line says so.
Temp copies are removed when NEAR quits. An archive panel is not restored on restart, since
the saved path is a file. Names stored without the UTF-8 flag (old archives, code page 437)
may show as mojibake. Other formats (7z, tar, rar) are not browsed.

## Where
`zip.go`, `app.go` (`readTab`, `enterCurrent`, `goUp`, `navigate`), `actions.go` guards and
the extract branch of `copyOrMove`, `draw.go` path row, `zip_test.go`.
