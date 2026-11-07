//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sys/windows"
)

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

// isNetworkPath reports whether a path lives on a mapped network drive or
// a UNC share, which is where listing a directory is slow enough to be
// worth caching.
func isNetworkPath(p string) bool {
	if strings.HasPrefix(p, `\\`) {
		return true
	}
	vol := strings.ToUpper(filepath.VolumeName(p))
	if len(vol) != 2 || vol[1] != ':' {
		return false
	}
	networkDriveMu.Lock()
	defer networkDriveMu.Unlock()
	if remote, ok := networkDrives[vol]; ok {
		return remote
	}
	remote := false
	if w, err := windows.UTF16PtrFromString(vol + `\`); err == nil {
		remote = windows.GetDriveType(w) == windows.DRIVE_REMOTE
	}
	networkDrives[vol] = remote
	return remote
}

// Drive letters do not change their kind while NEAR runs, so the answer is
// kept rather than asked of the system on every directory read.
var (
	networkDriveMu sync.Mutex
	networkDrives  = map[string]bool{}
)

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
