//go:build !windows

package main

import (
	"bufio"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func hasHiddenAttr(info fs.FileInfo) bool { return false }

func hideFile(path string) {}

// listLocations returns the root plus interesting mount points.
func listLocations() []Bookmark {
	out := []Bookmark{{Name: "/", Path: "/"}}
	f, err := os.Open("/proc/mounts")
	if err != nil {
		if runtime.GOOS == "darwin" {
			if des, err := os.ReadDir("/Volumes"); err == nil {
				for _, d := range des {
					out = append(out, Bookmark{Name: d.Name(), Path: "/Volumes/" + d.Name()})
				}
			}
		}
		return out
	}
	defer f.Close()
	seen := map[string]bool{"/": true}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		mp, fstype := fields[1], fields[2]
		switch fstype {
		case "proc", "sysfs", "devtmpfs", "devpts", "tmpfs", "cgroup", "cgroup2", "securityfs",
			"pstore", "debugfs", "tracefs", "configfs", "mqueue", "hugetlbfs", "fusectl",
			"binfmt_misc", "autofs", "efivarfs", "bpf", "squashfs", "overlay", "nsfs", "rpc_pipefs":
			continue
		}
		for _, skip := range []string{"/proc", "/sys", "/dev", "/run", "/snap", "/boot/efi", "/var/lib/docker"} {
			if mp == skip || strings.HasPrefix(mp, skip+"/") {
				mp = ""
				break
			}
		}
		if mp == "" || seen[mp] {
			continue
		}
		seen[mp] = true
		mp = strings.ReplaceAll(mp, `\040`, " ")
		out = append(out, Bookmark{Name: mp, Path: mp})
	}
	return out
}

func osOpenCmd(path string) *exec.Cmd {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", path)
	}
	return exec.Command("xdg-open", path)
}

func defaultShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "sh"
}

func defaultEditor() string { return "vi" }

func shellCommand(shell, cmdline string) *exec.Cmd {
	return exec.Command(shell, "-c", cmdline)
}

func pathsEqualFold() bool { return runtime.GOOS == "darwin" }
