//go:build windows

package main

import (
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// ShellExecuteW must report a missing file, with a non-ASCII (fullwidth)
// name passed through intact rather than mangled by an ANSI entry point.
// osOpenDirect is what the "-open" helper runs; osOpen checks the path
// itself before handing it over, so both report the missing file.
func TestOsOpenMissingUnicodePath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "2 Engel ＆ Charlie； FiNCH", "cover.jpg")
	if err := osOpenDirect(p); err == nil {
		t.Fatal("ShellExecuteW should report a missing file")
	}
	if err := osOpen(p); err == nil {
		t.Fatal("osOpen should report a missing file without launching the helper")
	}
}

// A file type with no associated program is reported as such, which osOpen
// can only tell from the helper's exit status.
func TestOpenExitCodeDistinguishesNoAssociation(t *testing.T) {
	if got := openExitCode(windows.ERROR_NO_ASSOCIATION); got != exitNoAssociation {
		t.Fatalf("no-association exit code = %d, want %d", got, exitNoAssociation)
	}
	if got := openExitCode(windows.ERROR_FILE_NOT_FOUND); got != 1 {
		t.Fatalf("other failures should exit 1, got %d", got)
	}
}
