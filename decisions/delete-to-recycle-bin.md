# Delete and F8 move to the Recycle Bin / trash; Shift+Delete and Shift+F8 delete for good

Date: 2026-10-24
Status: accepted

## Decision
`delete_to_trash = true` by default: Delete and F8 send entries to the Windows Recycle Bin
(SHFileOperationW) or the FreeDesktop trash on Linux (including `.Trash-<uid>` on other
file systems) and the macOS trash. Shift+Delete and Shift+F8 delete permanently, with a
progress box and Esc to stop. `confirm_delete` asks first in both cases.

## Why
"DEL should move a file to the Recycling Bin under Windows and Shift+DEL should really
delete it."

## Where
`trash_windows.go`, `trash_unix.go`, `actions.go` deleteSelected, `config.go`.
