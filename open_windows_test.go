//go:build windows

package main

import (
	"path/filepath"
	"testing"
)

// ShellExecuteW must report a missing file, with a non-ASCII (fullwidth)
// name passed through intact rather than mangled by an ANSI entry point.
func TestOsOpenMissingUnicodePath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "2 Engel ＆ Charlie； FiNCH", "cover.jpg")
	err := osOpen(p)
	if err == nil {
		t.Fatal("expected an error for a missing file")
	}
}
