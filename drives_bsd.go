//go:build darwin || freebsd

package main

import (
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

// mntRemovable is macOS's MNT_REMOVABLE mount flag.
const mntRemovable = 0x200

// isNetworkPath reports whether a path lies on a file system the kernel
// does not call local, which is where listing is slow enough to cache.
func isNetworkPath(p string) bool {
	n, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil || n <= 0 {
		return false
	}
	buf := make([]unix.Statfs_t, n)
	if n, err = unix.Getfsstat(buf, unix.MNT_NOWAIT); err != nil {
		return false
	}
	p = filepath.Clean(p)
	best, network := 0, false
	for _, st := range buf[:n] {
		mp := unix.ByteSliceToString(st.Mntonname[:])
		if mp != "/" && p != mp && !strings.HasPrefix(p, withSep(mp)) {
			continue
		}
		if len(mp) >= best {
			best, network = len(mp), st.Flags&unix.MNT_LOCAL == 0
		}
	}
	return network
}

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
