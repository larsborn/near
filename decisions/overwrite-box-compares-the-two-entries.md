# The "Destination exists" box compares the two entries

Date: 2026-11-25
Status: accepted

## Decision
When a copy or move runs into an existing destination under the "Ask" policy, the
confirmation lists what is actually at stake instead of only how many entries clash
(`conflicts.go`, `App.conflictLines`).

One clash gets the destination's full path, then a row each for the existing entry and the
new one, with size and timestamp aligned:

```
…\Temp\near\001\report.txt

  existing   5  2026-11-20 14:03
  new       30  2026-11-25 09:12   newer, larger
```

Several get one row each — name, existing size, new size, and the note — under a heading that
says which column is which, so the box stays narrow enough for an 80-column terminal:

```
4 entries already exist in the destination (existing -> new):

  adir           36 -> <DIR>   a directory would replace a file
  notes.md       26 ->     4   older, smaller
  report.txt      5 ->    42   newer, larger
  same.bin        9 ->     9   same size and time
```

Beyond `conflictsShown` (8) the rest become "… and N more". Sizes are exact and grouped
(`groupDigits`) rather than `humanSize`, because two different files round to the same "1.2 M"
and the point is to tell them apart. Directories show `<DIR>`. Timestamps use the configured
`date_format` and `time_format`.

`compareNote` compares modification times with the same strict `After`/`Before` that
`newerThan` uses, so the word "newer" in the box always means what the "Only newer" policy
would act on. A directory about to replace a file, or the reverse, is called out instead of
being described by size.

## Why
Asked for directly, against a screenshot of the old box, which said only "1 entries already
exist in the destination." — a question with no information to answer it. Overwriting is
destructive and irreversible, and the two facts that decide it are the size and the date.

The note ("newer, larger") is there because the raw numbers still need reading: when the box
appears mid-copy, a glance should be enough. It also keeps the multi-entry rows narrow, since
the note carries the time comparison and only the sizes need columns.

Tolerating a small clock difference when calling two files "the same time" was considered and
rejected: `newerThan` has no tolerance, so a box that smoothed over a one-second difference
would contradict the policy sitting next to it in the copy dialog.

## Consequences
- The box is wider and taller than before. It is sized from its longest line and clamped to
  the screen, and long paths keep their tail (`fitRight`), so a deep path shows the part that
  identifies the file.
- `doCopyOrMove` collects `[]conflict` with both `os.FileInfo`s rather than a count. The extra
  `Lstat` per target is on a path that was already stat-ing the destination.
- A source that cannot be read leaves `src` nil; the box then shows the existing side and
  drops the note rather than failing.

## Where
`conflicts.go` (`conflict`, `conflictLines`, `compareNote`, `conflictsShown`), `actions.go`
(`doCopyOrMove` builds the list), `conflicts_test.go`, README "Overwriting shows what is at
stake".
