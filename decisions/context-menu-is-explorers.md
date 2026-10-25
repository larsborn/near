# The context menu is Windows Explorer's shell menu; right-click selects the entry first

Date: 2026-10-25
Status: accepted

## Decision
Shift+F10, the keyboard Menu key and a right click open the entry's context menu. On
Windows it is populated exactly like Explorer's through the shell's IContextMenu
(including submenus and third-party entries such as 7-Zip); the shell verb is invoked
before the menu object is released. On Linux it is built from the desktop applications
that can open the file, on macOS from Finder's basics. NEAR's own actions follow after a
separator. A right click moves the cursor to the entry before the menu opens. The menu
takes mouse clicks. Because tcell's Windows console backend never reports the Menu key, a
poller watches VK_APPS while NEAR's terminal window, or a host such as Windows Terminal
that is an ancestor process, is in the foreground. Mouse support (`mouse = true`) came
with this: click moves the cursor, double-click opens, wheel scrolls.

## Why
Requested with a FAR screenshot; "populated exactly like the one from the Windows
explorer"; then "the file receives selection only after the menu closes", "the context
menu button doesn't work at all", "works in cmd.exe but not in Windows Terminal", "creating
a zip via 7z crashes".

## Consequences
Windows-only COM code with vtable calls; the Menu key poller is a workaround to revisit if
tcell starts reporting VK_APPS (`todo/shell-context-menu-follow-ups.md`).

## Where
`contextmenu.go`, `shellmenu_windows.go`, `shellmenu_other.go`, `menukey_windows.go`,
`mouse.go`, README "Context menu".
