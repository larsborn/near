# FAR's regular-expression file masks are not supported

## What
FAR's file masks (help topic `FileMasks`) allow a mask surrounded by slashes to be a Perl
regular expression with optional flags, for example `/(eng|rus)/i` for names containing
"eng" or "rus". NEAR's masks are globs only: `*`, `?` and `[c-f]` through
`filepath.Match`, split on commas and semicolons by `splitMasks` (`findfile.go`).

## Why it matters
It is the documented way to express "either of these two words anywhere in the name", which
a glob cannot do in one mask, and FAR users will try it. Entering `/eng/` today is treated
as a literal name and matches nothing, silently.

## Where
`findfile.go`: `newFindMatcher` builds `findMatcher.masks` from `splitMasks`, and
`matchName` runs `filepath.Match` against the base name, or against the relative path when
the mask contains a path separator.

## Options
1. Recognise a mask that starts and ends with `/` (with optional trailing flags) and compile
   it with `regexp`, keeping globs for everything else. `i` is the only flag worth
   supporting at first; NEAR already matches case-insensitively on Windows and macOS.
2. Leave it out and say so in the dialog, so nobody expects it.

## Acceptance
Either `/(eng|rus)/i` finds what FAR finds, with a test covering a regex mask, a glob mask
and a list mixing both; or the dialog and README state that masks are globs only. Whichever
is chosen gets a decision file.
