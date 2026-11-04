package main

import (
	"fmt"
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

// Blank lines take a row of their own, the screen is filled to the bottom
// even when lines wrap, and End reaches the last line.
func TestViewerKeepsBlankLinesAndFillsTheScreen(t *testing.T) {
	a, data := testApp(t)
	a.screen.(interface{ SetSize(int, int) }).SetSize(40, 12) // 10 viewer rows
	a.draw()
	var doc []string
	for i := 0; i < 60; i++ {
		doc = append(doc, fmt.Sprintf("L%02d", i), "")
	}
	doc = append(doc, "THE-END")
	os.WriteFile(filepath.Join(data, "blanks.txt"), []byte(strings.Join(doc, "\n")), 0o644)
	a.reloadAll()
	a.tab().SetCursorOn("blanks.txt")
	key(a, "F3")
	v, ok := a.top().(*Viewer)
	if !ok {
		t.Fatalf("expected the viewer, got %T", a.top())
	}
	// Rows 1..10 should alternate L00, blank, L01, blank, ...
	rows := strings.Split(screenText(a), "\n")
	for i := 0; i < 10; i++ {
		got := strings.TrimRight(rows[1+i], " ")
		want := ""
		if i%2 == 0 {
			want = fmt.Sprintf("L%02d", i/2)
		}
		if got != want {
			t.Fatalf("viewer row %d = %q, want %q (blank lines must keep their row)\n%s", i, got, want, screenText(a))
		}
	}
	// End shows the last line; before the fix it stayed out of reach.
	key(a, "End")
	if !strings.Contains(screenText(a), "THE-END") {
		t.Fatalf("End should reach the last line:\n%s", screenText(a))
	}
	// A long wrapped document still fills every row.
	v.Lines = []string{strings.Repeat("word ", 200)}
	v.Top = 0
	a.draw()
	rows = strings.Split(screenText(a), "\n")
	for i := 1; i <= 10; i++ {
		if strings.TrimSpace(rows[i]) == "" {
			t.Fatalf("wrapped text should fill row %d:\n%s", i, screenText(a))
		}
	}
}
