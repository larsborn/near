# Shift+F5 copies the cursor entry to another name beside it

Date: 2026-11-14
Status: accepted

## Decision
`copy_as`, bound to `Shift+F5`, asks for a name with the current one pre-filled and copies the
entry under the cursor to it, in the entry's own directory. It is the counterpart of
`Shift+F6` (`rename`) and is built to behave the same way:

- **The cursor entry, whatever is selected.** Only one name is asked for, so only one source
  makes sense. Because the selection is ignored, it is also left alone afterwards — hence
  `copyOptions.KeepSelection`, which stops `doCopyOrMove` from clearing it the way `F5`/`F6`
  rightly do.
- **A path is allowed, not just a name.** Relative paths resolve against the entry's own
  directory, so `alpha/copy.txt` works and an absolute path goes wherever it says. This comes
  free from reusing `doCopyOrMove` and matches what the `F5` destination field accepts.
- **An existing name is confirmed** by `doCopyOrMove`'s usual "Destination exists" box, and an
  existing *directory* means "copy into it", again as `F5` does.
- **Copying an entry onto itself is refused** rather than silently truncating it, the same
  guard `renameCurrent` has for a name that is already taken.
- **Directories are copied whole**, with the dialog titled "Copy directory" so it is obvious
  what is about to happen.
- **Refused inside a zip archive**, like every other action that writes
  (`zip-archives-are-read-only.md`): the destination would be a path inside the archive, which
  is read-only. `F5` remains the way to get something out of an archive.
- Timestamps are preserved (`KeepTimes`), matching the `F5` dialog's default.

## Why
Asked for directly: "Shift+F5 should ask for a file name and copy the currently selected file
to that name". It is FAR's binding for exactly this, and NEAR already had `Shift+F6` for the
rename half, so the pair was half-missing. The common use is keeping a quick backup of a file
before editing it, which otherwise needed `F5` to the other panel and a rename.

Reusing `doCopyOrMove` rather than writing a direct file copy means it gets the transfer queue,
the progress box, `Esc` mid-file, the overwrite policy and directory trees for nothing. The one
thing it had to be taught is not to clear the selection, since it never consulted it.

## Consequences
- `Shift+F5` was previously unbound, so nothing is taken away.
- A new option on `copyOptions` that only this action sets. Anything else calling
  `doCopyOrMove` keeps clearing the selection.
- The copy goes through the normal transfer queue, so for a large directory it shows the
  progress box and can be sent to the background with `B`.

## Where
`actions.go` (`copyAsCurrent`, and the `KeepSelection` check in `doCopyOrMove`),
`copydialog.go` (`copyOptions.KeepSelection`), `app.go` (`doAction`), `config.go` (the
`copy_as` default binding), `keys.go` (`actionHelp`), `copyas_test.go`, `zip_test.go`
(`TestZipIsReadOnly` covers the archive refusal), README "Copy to another name".
