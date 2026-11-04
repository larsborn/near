//go:build windows

package main

import (
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	openClipboard    = user32.NewProc("OpenClipboard")
	closeClipboard   = user32.NewProc("CloseClipboard")
	emptyClipboard   = user32.NewProc("EmptyClipboard")
	getClipboardData = user32.NewProc("GetClipboardData")
	setClipboardData = user32.NewProc("SetClipboardData")
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	globalAlloc      = kernel32.NewProc("GlobalAlloc")
	globalFree       = kernel32.NewProc("GlobalFree")
	globalLock       = kernel32.NewProc("GlobalLock")
	globalUnlock     = kernel32.NewProc("GlobalUnlock")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// writeClipboard puts text on the system clipboard as UTF-16, and reports
// whether it got there. The helper programs are no use here: clip.exe
// decodes its input in the console's OEM code page, which turns "ö" into
// two characters of rubbish.
func writeClipboard(text string) bool {
	u, err := windows.UTF16FromString(text) // includes the terminating NUL
	if err != nil {
		return false
	}
	if !openClipboardWaiting() {
		return false
	}
	defer closeClipboard.Call()
	emptyClipboard.Call()
	h, _, _ := globalAlloc.Call(gmemMoveable, uintptr(len(u)*2))
	if h == 0 {
		return false
	}
	p, _, _ := globalLock.Call(h)
	if p == 0 {
		globalFree.Call(h)
		return false
	}
	// The lock returns a plain address; go through &p so vet does not
	// mistake it for a Go pointer stored in a uintptr.
	copy(unsafe.Slice((*uint16)(*(*unsafe.Pointer)(unsafe.Pointer(&p))), len(u)), u)
	globalUnlock.Call(h)
	if r, _, _ := setClipboardData.Call(cfUnicodeText, h); r == 0 {
		globalFree.Call(h) // still ours when SetClipboardData failed
		return false
	}
	return true // the system owns the memory now
}

// openClipboardWaiting opens the clipboard, retrying briefly: only one
// process may hold it at a time and clipboard managers take it often.
func openClipboardWaiting() bool {
	for i := 0; i < 10; i++ {
		if r, _, _ := openClipboard.Call(0); r != 0 {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// readClipboard returns the system clipboard's text, if it holds any.
func readClipboard() (string, bool) {
	if !openClipboardWaiting() {
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
