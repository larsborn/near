//go:build windows

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

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
	// A missing file is the common failure and is worth an exact message;
	// the helper below could only report it through its exit status.
	if _, err := os.Stat(path); err != nil {
		return err
	}
	// Launch through a detached helper (near -open <path>) that has no
	// console of its own. Started directly, a GUI program is free to call
	// AttachConsole(ATTACH_PARENT_PROCESS), land on NEAR's console and write
	// its diagnostics over the screen; Electron editors do exactly that.
	cmd, err := openHelperCmd(path)
	if err != nil {
		return osOpenDirect(path) // no helper available: launch from here
	}
	if err := cmd.Start(); err != nil {
		return osOpenDirect(path)
	}
	// The helper only calls ShellExecuteW and exits, so waiting is brief.
	// It is bounded all the same: a shell that takes its time must not
	// freeze the panels (the old in-process call could block for as long as
	// it liked).
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		var ee *exec.ExitError
		switch {
		case err == nil:
		case errors.As(err, &ee) && ee.ExitCode() == exitNoAssociation:
			return fmt.Errorf("no program is associated with %s", filepath.Base(path))
		default:
			return fmt.Errorf("could not open %s", filepath.Base(path))
		}
	case <-time.After(3 * time.Second):
	}
	return nil
}

// exitNoAssociation is the helper's exit status when the file type has no
// associated program; every other failure exits 1.
const exitNoAssociation = 2

// openExitCode turns a ShellExecuteW failure into the helper's exit status,
// which is all osOpen gets to see of it.
func openExitCode(err error) int {
	if errors.Is(err, windows.ERROR_NO_ASSOCIATION) {
		return exitNoAssociation
	}
	return 1
}

// Creation flags for the helper: no console, and its own process group so
// a Ctrl+C in NEAR's console does not reach it.
const (
	detachedProcess       = 0x00000008
	createNewProcessGroup = 0x00000200
)

// openHelperCmd builds the detached "near -open <path>" helper command.
func openHelperCmd(path string) (*exec.Cmd, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(self, "-open", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: detachedProcess | createNewProcessGroup,
		HideWindow:    true,
	}
	return cmd, nil
}

// osOpenDirect performs the ShellExecuteW itself. It runs in the helper,
// where there is no console to disturb.
func osOpenDirect(path string) error {
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
