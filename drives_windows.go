//go:build windows

package main

import "golang.org/x/sys/windows"

// fixedDrives returns the roots of all fixed (internal, non-removable)
// drives, e.g. ["C:\\", "D:\\"]. Network, removable and optical drives are
// left out.
func fixedDrives() []string {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	var out []string
	for i := 0; i < 26; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		root := string(rune('A'+i)) + ":\\"
		p, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		if windows.GetDriveType(p) == windows.DRIVE_FIXED {
			out = append(out, root)
		}
	}
	return out
}
