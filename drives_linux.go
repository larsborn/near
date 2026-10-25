//go:build linux

package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// fixedDrives returns the mount points of local disk file systems that are
// not on removable media, falling back to "/" when /proc is unavailable.
func fixedDrives() []string {
	f, err := os.Open("/proc/self/mounts")
	if err != nil {
		return []string{"/"}
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		dev, mp, fstype := fields[0], fields[1], fields[2]
		if !localFsType(fstype) || strings.HasPrefix(dev, "/dev/loop") || removableDevice(dev) {
			continue
		}
		out = append(out, strings.ReplaceAll(mp, `\040`, " "))
	}
	if len(out) == 0 {
		return []string{"/"}
	}
	return out
}

// removableDevice reports whether a block device sits on removable media,
// according to sysfs (a partition inherits the flag from its disk).
func removableDevice(dev string) bool {
	if !strings.HasPrefix(dev, "/dev/") {
		return false
	}
	p, err := filepath.EvalSymlinks("/sys/class/block/" + filepath.Base(dev))
	if err != nil {
		return false
	}
	for _, f := range []string{filepath.Join(p, "removable"), filepath.Join(p, "..", "removable")} {
		if b, err := os.ReadFile(f); err == nil {
			return strings.TrimSpace(string(b)) == "1"
		}
	}
	return false
}
