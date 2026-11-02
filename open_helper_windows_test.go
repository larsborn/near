//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Opening a file goes through a detached helper, so that a GUI program
// cannot attach to NEAR's console and write over the screen.
func TestOpenHelperIsDetached(t *testing.T) {
	const path = `C:\tmp\notes.md`
	cmd, err := openHelperCmd(path)
	if err != nil {
		t.Fatal(err)
	}
	if base := strings.ToLower(filepath.Base(cmd.Path)); base != "near.exe" && !strings.HasSuffix(base, ".test.exe") {
		t.Fatalf("the helper should be NEAR itself, got %q", cmd.Path)
	}
	if len(cmd.Args) != 3 || cmd.Args[1] != "-open" || cmd.Args[2] != path {
		t.Fatalf("helper args = %q", cmd.Args)
	}
	attr := cmd.SysProcAttr
	if attr == nil {
		t.Fatal("the helper needs creation flags")
	}
	if attr.CreationFlags&detachedProcess == 0 {
		t.Fatalf("the helper must be detached from the console, flags = %#x", attr.CreationFlags)
	}
	if attr.CreationFlags&createNewProcessGroup == 0 {
		t.Fatalf("the helper needs its own process group, flags = %#x", attr.CreationFlags)
	}
	if !attr.HideWindow {
		t.Fatal("the helper must not flash a window")
	}
	// The command is only built here, never started: starting it would open
	// the file on the developer's desktop.
}
