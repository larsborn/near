package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The viewer wraps by default like FAR's; F2 switches wrapping off, which
// marks lines cut at the screen edge with » (and « once scrolled right);
// Shift+F2 chooses between word and character wrapping.
func TestViewerWrapAndContinuationMarks(t *testing.T) {
	a, data := testApp(t)
	a.screen.(interface{ SetSize(int, int) }).SetSize(40, 12)
	a.draw()
	long := strings.Repeat("word ", 7) + strings.Repeat("x", 50) + " end"
	os.WriteFile(filepath.Join(data, "long.txt"), []byte("short\n"+long+"\n"), 0o644)
	a.reloadAll()
	a.tab().SetCursorOn("long.txt")
	key(a, "F3")
	v, ok := a.top().(*Viewer)
	if !ok {
		t.Fatalf("expected the viewer, got %T", a.top())
	}
	if !v.Wrap || !v.WordWrap {
		t.Fatal("the viewer should wrap at words by default")
	}
	rowsOf := func() []string { return strings.Split(screenText(a), "\n") }
	// Word wrap: no row ends in the middle of a word, and no » marks.
	for _, r := range rowsOf()[1:11] {
		if strings.Contains(r, "»") || strings.HasSuffix(strings.TrimRight(r, " "), "wor") {
			t.Fatalf("word wrap broke a word or showed a mark: %q", r)
		}
	}
	if r := strings.TrimRight(rowsOf()[2], " "); !strings.HasSuffix(r, "word") {
		t.Fatalf("word wrap should break after the last word that fits: %q", r)
	}
	if !strings.Contains(screenText(a), "wrap: words") {
		t.Fatalf("footer should show the wrap mode:\n%s", screenText(a))
	}
	// Character wrap fills the whole width, cutting into the x's.
	key(a, "Shift+F2")
	if r := rowsOf()[2]; !strings.HasSuffix(r, "xxxxx") {
		t.Fatalf("character wrap should fill the row: %q", r)
	}
	// Wrap off: the long line is cut and marked with » at the right edge.
	key(a, "F2")
	s := screenText(a)
	rows := rowsOf()
	if !strings.HasSuffix(rows[2], "»") || strings.Contains(rows[1], "»") {
		t.Fatalf("expected » only on the long line:\n%s", s)
	}
	if !strings.Contains(rows[0], "Col 0") || !strings.Contains(rows[0], "UTF-8") {
		t.Fatalf("header should show column and encoding: %q", rows[0])
	}
	// Scrolled right: « at the left, Col in the header.
	key(a, "Right")
	rows = rowsOf()
	if !strings.HasPrefix(rows[2], "«") || !strings.HasSuffix(rows[2], "»") || !strings.Contains(rows[0], "Col 8") {
		t.Fatalf("expected « and » around the scrolled line:\n%s", screenText(a))
	}
	if strings.HasPrefix(rows[1], "«") {
		t.Fatalf("the short line has nothing to the left: %q", rows[1])
	}
}
