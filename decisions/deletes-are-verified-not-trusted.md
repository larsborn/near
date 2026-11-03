# A delete is verified on disk, not trusted to the shell's return value

Date: 2026-11-03
Status: accepted

## Decision
After `SHFileOperationW` reports success, `trashPaths` checks that the paths are really
gone and returns an error naming the ones that survived. NEAR only says "Moved N entries to
the Recycle Bin" when the entries have actually left.

## Why
The user deleted an empty folder on a network share, NEAR reported success and the folder
was still there. Microsoft documents the return value of SHFileOperationW as unreliable,
and with `FOF_SILENT | FOF_NOERRORUI` the shell can give up quietly. Claiming a delete that
did not happen is worse than reporting the failure, especially as the panel listing is
re-read immediately afterwards and then contradicts the status line.

## Consequences
One `Lstat` per entry after a delete. The exact failure above was never reproduced:
deleting an empty folder on that share, in that directory, with those flags, works both
from a test process and from the running app. Whatever the cause, NEAR now reports it
instead of claiming success.

## Where
`trash_windows.go` (`trashPaths`, `stillThere`), `trash_result_windows_test.go`.
