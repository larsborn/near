//go:build windows

package main

import (
	"time"
	"unsafe"

	"github.com/gdamore/tcell/v2"
	"golang.org/x/sys/windows"
)

// The Windows console never reports the keyboard's Menu (Apps) key, so a
// small watcher polls its state while near's terminal window is in the
// foreground and injects the key event tcell would have produced. The
// terminal window may belong to conhost (cmd.exe window) or to a terminal
// such as Windows Terminal that hosts near through a pseudo-console; both
// are recognised by their process being an ancestor of near or the owner
// of near's console window.

var (
	getAsyncKeyState         = user32.NewProc("GetAsyncKeyState")
	getForegroundWindow      = user32.NewProc("GetForegroundWindow")
	getWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	getConsoleWindow         = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow")
)

const vkApps = 0x5D

func startMenuKeyWatcher(s tcell.Screen) {
	hosts := hostProcessIDs()
	go func() {
		down := false
		for {
			time.Sleep(40 * time.Millisecond)
			st, _, _ := getAsyncKeyState.Call(vkApps)
			pressed := st&0x8000 != 0
			if pressed && !down && foregroundIsOurs(hosts) {
				_ = s.PostEvent(tcell.NewEventKey(tcell.KeyMenu, 0, 0))
			}
			down = pressed
		}
	}()
}

// hostProcessIDs returns near's own pid, its ancestors (the shell and the
// terminal that started it) and the owner of its console window.
func hostProcessIDs() map[uint32]bool {
	ids := map[uint32]bool{windows.GetCurrentProcessId(): true}
	if con, _, _ := getConsoleWindow.Call(); con != 0 {
		var pid uint32
		getWindowThreadProcessId.Call(con, uintptr(unsafe.Pointer(&pid)))
		ids[pid] = true
	}
	parents := map[uint32]uint32{}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return ids
	}
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	for err = windows.Process32First(snap, &pe); err == nil; err = windows.Process32Next(snap, &pe) {
		parents[pe.ProcessID] = pe.ParentProcessID
	}
	pid := windows.GetCurrentProcessId()
	for i := 0; i < 8; i++ {
		p, ok := parents[pid]
		if !ok || p == 0 || p == pid {
			break
		}
		ids[p] = true
		pid = p
	}
	return ids
}

func foregroundIsOurs(hosts map[uint32]bool) bool {
	fg, _, _ := getForegroundWindow.Call()
	if fg == 0 {
		return false
	}
	if con, _, _ := getConsoleWindow.Call(); con != 0 && fg == con {
		return true
	}
	var pid uint32
	getWindowThreadProcessId.Call(fg, uintptr(unsafe.Pointer(&pid)))
	return hosts[pid]
}
