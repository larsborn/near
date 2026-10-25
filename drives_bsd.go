//go:build darwin || freebsd

package main

import (
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

// mntRemovable is macOS's MNT_REMOVABLE mount flag.
const mntRemovable = 0x200

// fixedDrives returns the mount points of local, non-removable file
// systems. On macOS the system volume's firmlinks already lead into the Data
// volume, so the volumes under /System/Volumes are left out.
func fixedDrives() []string {
	n, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil || n <= 0 {
		return []string{"/"}
	}
	buf := make([]unix.Statfs_t, n)
	n, err = unix.Getfsstat(buf, unix.MNT_NOWAIT)
	if err != nil {
		return []string{"/"}
	}
	var out []string
	for _, st := range buf[:n] {
		if st.Flags&unix.MNT_LOCAL == 0 {
			continue
		}
		fstype := unix.ByteSliceToString(st.Fstypename[:])
		mp := unix.ByteSliceToString(st.Mntonname[:])
		if !localFsType(fstype) {
			continue
		}
		if runtime.GOOS == "darwin" {
			if st.Flags&mntRemovable != 0 || strings.HasPrefix(mp, "/System/Volumes/") || strings.HasPrefix(mp, "/private/var/vm") {
				continue
			}
		}
		out = append(out, mp)
	}
	if len(out) == 0 {
		return []string{"/"}
	}
	return out
}
