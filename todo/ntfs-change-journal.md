# Keep the index current from the NTFS change journal instead of rebuilding at start

## What
With `search_roots = ["*"]` the developer's index holds 7.7 million names. The start-up
rebuild (`search_index_on_start = true`) walks all of C: and D: for about 96 s at every
start and churns ~3 GB of memory, only to catch changes made while NEAR was closed. NTFS
keeps a per-volume change journal (USN journal) that records every create, delete and
rename with a running sequence number. Storing the last seen number per volume in the cache
lets the next start apply only the records since then: current within a second, no walk.

Decision: `decisions/ntfs-journal-in-process-not-a-service.md` (in-process, unprivileged,
no background service).

## Status (2026-10-26)
Step 1 is done (`usn_windows.go`, `Index.CatchUp`, `TestJournalCatchUp`). Findings that
changed the plan: the unprivileged read works on a volume handle opened with FILE_TRAVERSE,
input READ_USN_JOURNAL_DATA_V1 with record versions 2..2, start USN on a record boundary;
but it returns every record **without its file name**. So records are only used for their
parent reference number: the parent directory is resolved with OpenFileById and re-read,
and its listing reconciled with the index (`syncDirs`). Deleted directories cannot be
resolved; their own record names the grandparent, whose re-read drops the subtree.
The first start after this change still rebuilds once (old caches carry no positions).

## Plan
1. ~~Catch-up at start~~ done, see above.
2. **Live tailing** (second): replace ReadDirectoryChangesW on NTFS volumes by tailing the
   journal so the saved USN is exact and overflow cannot happen. Until then the directory
   watcher stays, and the USN stored at save time may miss a change in the sub-second gap
   between the watcher event and the save.
3. **Other platforms**: no journal; add an age-based start-up rebuild
   (`search_index_max_age`) so Linux/macOS do not rebuild at every start either.

## Technical notes
- Journal records name the file and its parent by file reference number; the parent is
  resolved to a path with OpenFileById + GetFinalPathNameByHandle on the volume, cached
  per batch. A parent that cannot be opened was deleted; its own delete record removes the
  subtree, so the child record is skipped.
- Renames arrive as RENAME_OLD_NAME (remove) and RENAME_NEW_NAME (add) records.
  ReturnOnlyOnClose must be 0 or the old-name record is never seen; intermediate records
  are harmless because a create is applied as remove+add.
- Records are filtered by the configured roots and `search_exclude`.
- Control codes: FSCTL_QUERY_USN_JOURNAL 0x900f4, FSCTL_READ_UNPRIVILEGED_USN_JOURNAL
  0x903ab. The unprivileged read exists since Windows 10 1709 (believed, unverified); on
  older systems the DeviceIoControl fails and the fallback is the rebuild.
- `fsutil usn queryjournal C:` shows whether a volume has a journal and its size; the
  default size on system volumes holds a few hundred thousand records, so a long absence
  wraps it and triggers a rebuild.

## Acceptance
- Starting NEAR with a fresh cache and journal position applies changes made while it was
  closed (create, delete, rename of files and directories) without a rebuild; the search
  dialog's state line says "caught up N changes from the change journal".
- Journal mismatch, disabled journal, non-NTFS roots and other platforms still rebuild.
- Tests: a unit test for record parsing with hand-built buffers; a live test on Windows
  that writes files under a temp dir on C:, saves a position, changes files, and verifies
  the catch-up (skipped when the journal cannot be read).
- `todo/index-memory-and-search-speed.md` updated: start-up churn gone on Windows.
