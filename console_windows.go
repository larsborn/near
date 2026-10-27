//go:build windows

package main

import "syscall"

var (
	procGetConsoleWindow = kernel32.NewProc("GetConsoleWindow")
	procAllocConsole     = kernel32.NewProc("AllocConsole")
	procSetConsoleMode   = kernel32.NewProc("SetConsoleMode")
)

// Console input mode flags as tcell's Windows backend sets them.
const (
	conModeWindowInput  = 0x0008
	conModeExtendedFlag = 0x0080
	conModeVTInput      = 0x0200
)

// restoreConsoleInput puts the console's input mode back to what tcell
// expects. A program that shared the console (anything run from the
// command line) may have switched it to line or processed input, after
// which keys only arrive on Enter and Ctrl+C becomes a control event.
// tcell runs the console in virtual-terminal input mode (0x288, measured
// on a live instance); the mouse comes through VT sequences, so the mouse
// flag is not part of it.
func restoreConsoleInput() {
	h, err := syscall.Open("CONIN$", syscall.O_RDWR, 0)
	if err != nil {
		return
	}
	defer syscall.Close(h)
	_, _, _ = procSetConsoleMode.Call(uintptr(h), uintptr(conModeVTInput|conModeWindowInput|conModeExtendedFlag))
}

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
