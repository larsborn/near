//go:build windows

package main

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"golang.org/x/sys/windows"
)

// The Windows console never reports the keyboard's Menu (Apps) key, so a
// small watcher polls its state while near's window is in the foreground
// and injects the key event tcell would have produced.

var (
	getAsyncKeyState    = user32.NewProc("GetAsyncKeyState")
	getForegroundWindow = user32.NewProc("GetForegroundWindow")
	getConsoleWindow    = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow")
)

const vkApps = 0x5D

func startMenuKeyWatcher(s tcell.Screen) {
	go func() {
		down := false
		for {
			time.Sleep(40 * time.Millisecond)
			st, _, _ := getAsyncKeyState.Call(vkApps)
			pressed := st&0x8000 != 0
			if pressed && !down {
				fg, _, _ := getForegroundWindow.Call()
				con, _, _ := getConsoleWindow.Call()
				if fg == con && fg != 0 {
					_ = s.PostEvent(tcell.NewEventKey(tcell.KeyMenu, 0, 0))
				}
			}
			down = pressed
		}
	}()
}
