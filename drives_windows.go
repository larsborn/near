//go:build windows

package main

import "golang.org/x/sys/windows"

// fixedDrives returns the roots of all fixed (internal, non-removable)
// drives, e.g. ["C:\\", "D:\\"]. Network, removable and optical drives are
// left out, as are drives made with subst, which only mirror a directory
// of another drive.
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
		letter := string(rune('A' + i))
		root := letter + ":\\"
		p, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		if windows.GetDriveType(p) == windows.DRIVE_FIXED && !substDrive(letter) {
			out = append(out, root)
		}
	}
	return out
}

// substDrive reports whether a drive letter was created with subst (or
// DefineDosDevice): its DOS device then points at a path, "\??\C:\dir",
// rather than at a volume device.
func substDrive(letter string) bool {
	name, err := windows.UTF16PtrFromString(letter + ":")
	if err != nil {
		return false
	}
	buf := make([]uint16, 1024)
	n, err := windows.QueryDosDevice(name, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 {
		return false
	}
	target := windows.UTF16ToString(buf[:n])
	return len(target) > 4 && target[:4] == `\??\`
}
