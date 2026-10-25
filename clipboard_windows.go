//go:build windows

package main

import (
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	openClipboard    = user32.NewProc("OpenClipboard")
	closeClipboard   = user32.NewProc("CloseClipboard")
	getClipboardData = user32.NewProc("GetClipboardData")
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	globalLock       = kernel32.NewProc("GlobalLock")
	globalUnlock     = kernel32.NewProc("GlobalUnlock")
)

const cfUnicodeText = 13

// readClipboard returns the system clipboard's text, if it holds any.
func readClipboard() (string, bool) {
	if r, _, _ := openClipboard.Call(0); r == 0 {
		return "", false
	}
	defer closeClipboard.Call()
	h, _, _ := getClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return "", false
	}
	p, _, _ := globalLock.Call(h)
	if p == 0 {
		return "", false
	}
	defer globalUnlock.Call(h)
	// The lock returns a plain address; go through &p so vet does not
	// mistake it for a Go pointer stored in a uintptr.
	base := *(*unsafe.Pointer)(unsafe.Pointer(&p))
	var u []uint16
	for i := 0; ; i++ {
		c := *(*uint16)(unsafe.Add(base, i*2))
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	return string(utf16.Decode(u)), true
}
