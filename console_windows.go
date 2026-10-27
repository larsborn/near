//go:build windows

package main

var (
	procGetConsoleWindow = kernel32.NewProc("GetConsoleWindow")
	procAllocConsole     = kernel32.NewProc("AllocConsole")
)

// ensureConsole gives NEAR a console when it was started without one.
// GUI launchers (Keykoff, Explorer shortcuts with "detached" flags, task
// schedulers) create console programs without a console; tcell then cannot
// open CONIN$/CONOUT$ and NEAR would exit before showing anything.
func ensureConsole() {
	if r, _, _ := procGetConsoleWindow.Call(); r != 0 {
		return
	}
	_, _, _ = procAllocConsole.Call()
}
