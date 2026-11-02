# Files are opened through a detached helper process, not from NEAR itself

Date: 2026-11-02
Status: accepted

## Decision
On Windows, Enter on a file (and anything else calling `osOpen`) spawns `near -open <path>`
with DETACHED_PROCESS and its own process group; that helper performs the ShellExecuteW and
exits. The `-open` entry point returns before `ensureConsole`, so the helper never has a
console. If the helper cannot be spawned, NEAR falls back to calling ShellExecuteW itself.
`refresh` (Ctrl+R) additionally calls `Screen.Sync`, repainting every cell, so a screen that
something else wrote on can be repaired by hand.

## Why
A GUI program may call `AttachConsole(ATTACH_PARENT_PROCESS)` to make its diagnostics
visible when started from a terminal; Electron does. Started directly by NEAR, it attached
to NEAR's console and wrote over the drawn screen — reported after opening a `.md` file in
VS Code. Measured with a GUI-subsystem probe that does the same attach: launched directly its
output appears across NEAR's panels and key bar, launched through the detached helper the
console stays clean. A new console for the child was not wanted (a window would flash) and
`SEE_MASK_NO_CONSOLE` does the opposite of what its name suggests (it *grants* the parent's
console).

## Consequences
Opening a file costs one short-lived extra process. `near -open <path>` is a documented flag
and works on its own. On Linux and macOS no detour is needed: the child's standard streams
are left unset, so os/exec gives it /dev/null. Shell verbs invoked from the context menu
(`IContextMenu::InvokeCommand`) still run in-process and can still write to the console; see
`todo/shell-context-menu-follow-ups.md`.

## Where
`platform_windows.go` (`osOpen`, `openHelperCmd`, `osOpenDirect`), `main.go` `-open`,
`open_helper_windows_test.go`, `app.go` refresh.
