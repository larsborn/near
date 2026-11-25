package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// clash writes a file of a known size and time, so a copy runs into it.
func clash(t *testing.T, dir, name, text string, when time.Time) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, when, when); err != nil {
		t.Fatal(err)
	}
}

var (
	conflictOld = time.Date(2026, 11, 20, 14, 3, 0, 0, time.Local)
	conflictNew = time.Date(2026, 11, 25, 9, 12, 0, 0, time.Local)
)

// Overwriting is a decision, so the box says what is there and what would
// replace it rather than only how many entries clash.
func TestConflictDialogComparesTheTwoFiles(t *testing.T) {
	a, data := testApp(t)
	other := a.other().Path
	clash(t, data, "report.txt", "a much longer replacement text", conflictNew) // 30 bytes
	clash(t, other, "report.txt", "short", conflictOld)                         // 5 bytes
	a.reloadAll()
	a.tab().SetCursorOn("report.txt")
	key(a, "F5")
	key(a, "Enter")

	if _, ok := a.top().(*ConfirmDialog); !ok {
		t.Fatalf("expected the overwrite confirmation, got %T", a.top())
	}
	screen := screenText(a)
	want := []string{
		// Which file is about to go. A long path keeps its tail, so only
		// the last components are on screen.
		filepath.Join("001", "report.txt"),
		"existing   5  2026-11-20 14:03",
		"new       30  2026-11-25 09:12   newer, larger",
	}
	for _, w := range want {
		if !strings.Contains(screen, w) {
			t.Errorf("the box does not show %q:\n%s", w, screen)
		}
	}
}

// With several clashes each gets a row, sizes side by side, and the note
// carries the rest so the box stays narrow.
func TestConflictDialogListsEveryClash(t *testing.T) {
	a, data := testApp(t)
	other := a.other().Path
	clash(t, data, "report.txt", "a much longer replacement text here indeed", conflictNew)
	clash(t, other, "report.txt", "short", conflictOld)
	clash(t, data, "notes.md", "tiny", conflictOld)
	clash(t, other, "notes.md", "a far bigger existing file", conflictNew)
	clash(t, data, "same.bin", "identical", conflictNew)
	clash(t, other, "same.bin", "identical", conflictNew)
	if err := os.MkdirAll(filepath.Join(data, "adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	clash(t, other, "adir", "a file where a directory wants to go", conflictOld)
	a.reloadAll()
	for _, n := range []string{"report.txt", "notes.md", "same.bin", "adir"} {
		a.tab().SetCursorOn(n)
		key(a, "Insert")
	}
	key(a, "F5")
	key(a, "Enter")

	screen := screenText(a)
	for _, want := range []string{
		"4 entries already exist in the destination (existing -> new):",
		"adir           36 -> <DIR>   a directory would replace a file",
		"notes.md       26 ->     4   older, smaller",
		"report.txt      5 ->    42   newer, larger",
		"same.bin        9 ->     9   same size and time",
	} {
		if !strings.Contains(screen, want) {
			t.Errorf("the box does not show %q:\n%s", want, screen)
		}
	}
}

// Long lists are cut off so the box still fits on the screen.
func TestConflictLinesStopCounting(t *testing.T) {
	a, _ := testApp(t)
	var cs []conflict
	for i := 0; i < conflictsShown+5; i++ {
		name := fmt.Sprintf("f%02d.txt", i)
		cs = append(cs, conflict{name: name, path: "/x/" + name, dst: statOf(t, a, "file1.txt"), src: statOf(t, a, "file10.txt")})
	}
	lines := a.conflictLines(cs)
	last := lines[len(lines)-1]
	if last != "  … and 5 more" {
		t.Fatalf("last line is %q", last)
	}
	if got := len(lines); got != conflictsShown+3 { // heading, blank, rows, the "more" line
		t.Fatalf("%d lines for %d conflicts", got, len(cs))
	}
}

func statOf(t *testing.T, a *App, name string) os.FileInfo {
	t.Helper()
	fi, err := os.Lstat(filepath.Join(a.tab().Path, name))
	if err != nil {
		t.Fatal(err)
	}
	return fi
}

func TestCompareNote(t *testing.T) {
	a, data := testApp(t)
	clash(t, data, "a.txt", "12345", conflictOld)
	clash(t, data, "b.txt", "1234567890", conflictNew)
	small, big := statOf(t, a, "a.txt"), statOf(t, a, "b.txt")
	dir, err := os.Lstat(filepath.Join(data, "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	// Same size, different time: the case that matters most, since it
	// usually means the same file downloaded again.
	clash(t, data, "again.txt", "12345", conflictNew)
	sameSizeNewer := statOf(t, a, "again.txt")
	// Same time, different size.
	clash(t, data, "wide.txt", "1234567890", conflictOld)
	sameTimeBigger := statOf(t, a, "wide.txt")

	cases := []struct {
		name     string
		dst, src os.FileInfo
		want     string
	}{
		{"bigger and newer", small, big, "newer, larger"},
		{"smaller and older", big, small, "older, smaller"},
		{"the same file", small, small, "same size and time"},
		{"same size, newer", small, sameSizeNewer, "newer, same size"},
		{"same size, older", sameSizeNewer, small, "older, same size"},
		{"same time, bigger", small, sameTimeBigger, "same time, larger"},
		{"a directory over a file", small, dir, "a directory would replace a file"},
		{"a file over a directory", dir, small, "a file would replace a directory"},
		{"nothing to compare", small, nil, ""},
	}
	for _, c := range cases {
		if got := compareNote(c.dst, c.src); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
