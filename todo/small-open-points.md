# Small open points and pending decisions

One file for the minor items; promote any of them to its own file if it grows.

## Decisions awaiting the user
- **Release `v0.0.2`**: everything since `v0.0.1` is on `main`; tag and push when wanted
  (`git tag -a v0.0.2 -m "near 0.0.2" && git push --tags`, the release workflow does the rest).
- **Alt+digit**: `Alt+1`..`Alt+9` switch tabs, so a quick search cannot start with a digit.
  Either accept, or drop the digit tab keys (then Alt+digit searches).
- **Editor: typing over a selection replaces it** (modern behaviour). FAR by default keeps the
  block text and only drops the selection. Keep or match FAR?
- **Ctrl+P hides the inactive panel** (FAR's definition, chosen over "right panel").

## Cosmetic / minor
- Panel path row shows a ` hidden ` flag whenever hidden files are listed; since that is now
  the default it is always on. Show it only when hidden files are *not* shown, or drop it.
- `TestTrashPaths` on Windows sends two small temp items to the real Recycle Bin on every
  test run; they accumulate. Empty them in the test (SHEmptyRecycleBin is too drastic) or
  mark the test for manual runs.
- Panel heights set by `ensureConsoleBand` (console output band) are persisted in
  `state.json`; after a restart with an empty console the panels stay shorter with a blank
  band until Ctrl+Down. Could reset heights at startup when the console is empty.
- With `mouse = true` the terminal's own text selection needs Shift held (or the terminal's
  override); mention in README.
- Search results within the `search_max_results` cap are whatever map iteration yields;
  sorted alphabetically afterwards. Relevance ordering (prefix matches first) would be nicer.
- The `menukey_windows.go` poller runs for the whole process even if the Menu key is unbound.

## Acceptance
Each line either implemented (and removed here), promoted to its own `todo/` file, or
explicitly decided and recorded in README/CLAUDE.md.
