//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const trashName = "Recycle Bin"

var (
	shell32         = windows.NewLazySystemDLL("shell32.dll")
	shFileOperation = shell32.NewProc("SHFileOperationW")
)

// shFileOpStruct mirrors SHFILEOPSTRUCTW (8-byte packing on 64-bit Windows).
type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

const (
	foDelete           = 3
	fofSilent          = 0x0004
	fofNoConfirmation  = 0x0010
	fofAllowUndo       = 0x0040
	fofNoErrorUI       = 0x0400
	fofNoConfirmMkdir  = 0x0200
	fofWantNukeWarning = 0x4000
)

// trashPaths moves the given absolute paths to the Recycle Bin.
func trashPaths(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	// pFrom is a list of NUL-terminated strings ended by a second NUL.
	var buf []uint16
	for _, p := range paths {
		u, err := windows.UTF16FromString(p)
		if err != nil {
			return err
		}
		buf = append(buf, u...) // includes the terminating NUL
	}
	buf = append(buf, 0)
	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  &buf[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI,
	}
	r, _, _ := shFileOperation.Call(uintptr(unsafe.Pointer(&op)))
	if r != 0 {
		return fmt.Errorf("recycle bin: error 0x%x", r)
	}
	if op.fAnyOperationsAborted != 0 {
		return errCancelled
	}
	return nil
}
