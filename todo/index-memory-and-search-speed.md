# Search index: memory and per-keystroke latency on very large trees

## What
The Everything-like index (`search.go`) is `map[directory][]names`. On the developer's
machine it holds ~5 million names in ~475,000 directories.

## Measured (2026-10-23, real cache)
| | flat slice (old) | map (current) |
| --- | --- | --- |
| cache file | 570 MB | 169 MB |
| cache load | 475 ms | 294 ms |
| name search | 175 ms | 330 ms |
| path search | 574 ms | 829 ms |
| glob search | 876 ms | 917 ms |

Memory in-process is roughly 500–600 MB for that tree. The search dialog coalesces keystrokes
(`SearchDialog.dirty` + `HasPendingEvent`) so typing fast searches once, but each search still
blocks the UI for the duration.

## Ideas
- Run the search in a goroutine with a generation counter; show results when done (UI stays
  responsive, late results dropped).
- Store names of a directory as one string with separators to improve locality.
- Per-directory lowercase cache only for directories with uppercase names.
- Default `search_exclude` suggestions for Windows (`AppData`) in the generated config.
- Sort results by relevance (prefix match first) instead of map order — see
  `search-results-ordering.md`.

## Acceptance
Typing in the search dialog never freezes the UI noticeably on a 5M-name index; memory for
that index measured and documented; decisions on defaults recorded in README.
