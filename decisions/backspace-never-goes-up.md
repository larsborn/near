# Backspace never goes to the parent directory; Esc alone does

Date: 2026-10-31
Status: accepted
Supersedes: esc-goes-to-parent-directory.md

## Decision
`up_dir` is bound to `Esc` only. Backspace is not bound to any panel action: it edits the
command line, and when the line is empty it does nothing. Esc keeps its two jobs as before,
clearing a command line that has text and otherwise going up.

## Why
With the panels hidden and the command line emptied with Backspace, one keystroke too many
moved the hidden panel to its parent directory, invisibly. The user asked for Backspace never
to go up, anywhere. Deleting the last character and leaving a directory are the same gesture
otherwise, which cannot be made safe by context alone.

## Consequences
FAR's Backspace-goes-up is not matched. The generated default config says so on the `up_dir`
line, so a regenerated config does not quietly bring it back; existing user configs that list
`up_dir = ["Esc", "Backspace"]` keep the old behaviour until that line is edited (the
developer's live config was updated).

## Where
`config.go` default `[keys]`, `app_test.go` `TestBackspaceNeverGoesUp`,
`decisions/esc-goes-to-parent-directory.md` (superseded).
