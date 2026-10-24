//go:build windows

package main

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func hasHiddenAttr(info fs.FileInfo) bool {
	if d, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return d.FileAttributes&syscall.FILE_ATTRIBUTE_HIDDEN != 0
	}
	return false
}

func hideFile(path string) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	attrs, err := syscall.GetFileAttributes(p)
	if err != nil {
		return
	}
	_ = syscall.SetFileAttributes(p, attrs|syscall.FILE_ATTRIBUTE_HIDDEN)
}

// listLocations returns the drive letters that currently exist.
func listLocations() []Bookmark {
	var out []Bookmark
	for c := 'A'; c <= 'Z'; c++ {
		root := string(c) + ":\\"
		if _, err := os.Stat(root); err == nil {
			out = append(out, Bookmark{Name: string(c) + ":", Path: root})
		}
	}
	return out
}

// osOpen opens path with its associated program through ShellExecuteW, which
// takes the path as UTF-16 so names outside the system code page work
// (rundll32 url.dll,FileProtocolHandler is ANSI-only and mangles them).
func osOpen(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	verb, _ := windows.UTF16PtrFromString("open")
	dir, _ := windows.UTF16PtrFromString(filepath.Dir(path))
	return windows.ShellExecute(0, verb, p, nil, dir, windows.SW_SHOWNORMAL)
}

func defaultShell() string  { return "cmd" }
func defaultEditor() string { return "notepad" }

func shellCommand(shell, cmdline string) *exec.Cmd {
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(shell), ".exe"))
	if base == "powershell" || base == "pwsh" {
		return exec.Command(shell, "-NoLogo", "-Command", cmdline)
	}
	cmd := exec.Command(shell)
	// cmd.exe mangles quotes when Go re-quotes arguments; pass the line raw.
	// /S makes cmd strip exactly the outer pair of quotes.
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `"` + shell + `" /S /C "` + cmdline + `"`}
	return cmd
}

func pathsEqualFold() bool { return true }

// setProcessGroup is a no-op on Windows; killTree uses taskkill instead.
func setProcessGroup(cmd *exec.Cmd) {}

// killTree ends the command and everything it started.
func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	k := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	k.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := k.Run(); err != nil {
		_ = cmd.Process.Kill()
	}
}
