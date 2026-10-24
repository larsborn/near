//go:build !windows

package main

import (
	"bufio"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
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

// osOpen opens path with its associated program (open / xdg-open).
func osOpen(path string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", path)
	} else {
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
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

// setProcessGroup puts the command in its own process group so that
// killTree can end it together with its children.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killTree sends SIGTERM to the command's process group, then SIGKILL if
// it is still around a moment later.
func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pgid := -cmd.Process.Pid
	_ = syscall.Kill(pgid, syscall.SIGTERM)
	go func() {
		time.Sleep(2 * time.Second)
		_ = syscall.Kill(pgid, syscall.SIGKILL)
	}()
}
