package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
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

// Searching in the viewer marks every hit, says what it found, and offers
// the same text again, selected, the next time.
func TestViewerSearchMarksHits(t *testing.T) {
	a, data := testApp(t)
	a.screen.(interface{ SetSize(int, int) }).SetSize(60, 14)
	a.draw()
	doc := "first line\nserver { listen 80; }\nnothing here\nnginx again nginx\n"
	os.WriteFile(filepath.Join(data, "conf.txt"), []byte(doc), 0o644)
	a.reloadAll()
	a.tab().SetCursorOn("conf.txt")
	key(a, "F3")
	v, ok := a.top().(*Viewer)
	if !ok {
		t.Fatalf("expected the viewer, got %T", a.top())
	}
	key(a, "F7")
	typeText(a, "nginx")
	key(a, "Enter")
	if v.query != "nginx" || v.cursor != 3 {
		t.Fatalf("search put the cursor on line %d with query %q", v.cursor, v.query)
	}
	a.draw()
	if s := screenText(a); !strings.Contains(s, `"nginx" on line 4 of 1 lines with it`) {
		t.Fatalf("the footer should report the hit:\n%s", s)
	}
	// Both occurrences on the line are drawn reversed: 2 x len("nginx").
	marked := 0
	cells, w, _ := a.screen.(tcell.SimulationScreen).GetContents()
	for i, c := range cells {
		if i/w != 1 {
			continue // the found line is the first one shown
		}
		if _, _, attr := c.Style.Decompose(); attr&tcell.AttrReverse != 0 {
			marked++
		}
	}
	if marked != 10 {
		t.Fatalf("both occurrences should be marked (10 cells), found %d", marked)
	}
	// Searching again offers the same text, selected, so typing replaces it.
	key(a, "F7")
	d, ok := a.top().(*InputDialog)
	if !ok {
		t.Fatalf("expected the search dialog, got %T", a.top())
	}
	if d.Edit.String() != "nginx" {
		t.Fatalf("the search text should be offered again, got %q", d.Edit.String())
	}
	if d.Edit.SelectedText() != "nginx" {
		t.Fatalf("it should be selected, selection is %q", d.Edit.SelectedText())
	}
	typeText(a, "server")
	key(a, "Enter")
	if v.query != "server" || v.cursor != 1 {
		t.Fatalf("typing should have replaced it: query %q line %d", v.query, v.cursor)
	}
	// Something that is not there says so instead of moving.
	key(a, "F7")
	typeText(a, "zzz")
	key(a, "Enter")
	a.draw()
	if s := screenText(a); !strings.Contains(s, `"zzz" is not in this file`) {
		t.Fatalf("a miss should be reported in the viewer:\n%s", s)
	}
}

// A file too big to read in full is shown from the beginning instead of
// being refused, and the viewer says how much it read.
func TestViewerShowsTheStartOfAHugeFile(t *testing.T) {
	a, data := testApp(t)
	a.screen.(interface{ SetSize(int, int) }).SetSize(100, 14)
	oldText, oldHex := viewerMaxBytes, viewerHexMaxBytes
	viewerMaxBytes, viewerHexMaxBytes = 2048, 1024
	t.Cleanup(func() { viewerMaxBytes, viewerHexMaxBytes = oldText, oldHex })

	big := strings.Repeat("line of text\n", 1000) // ~13 KB
	os.WriteFile(filepath.Join(data, "big.log"), []byte(big), 0o644)
	a.reloadAll()
	a.tab().SetCursorOn("big.log")
	key(a, "F3")
	v, ok := a.top().(*Viewer)
	if !ok {
		t.Fatalf("F3 should open the viewer, got %T", a.top())
	}
	if !v.cut {
		t.Fatal("the viewer should know the file was cut short")
	}
	if int64(len(v.raw)) != viewerMaxBytes {
		t.Fatalf("read %d bytes, want %d", len(v.raw), viewerMaxBytes)
	}
	if v.size != int64(len(big)) {
		t.Fatalf("the whole size should be known: %d, want %d", v.size, len(big))
	}
	a.draw()
	s := screenText(a)
	if !strings.Contains(s, "only the first 2.0 K was read") {
		t.Fatalf("the footer should say what was read:\n%s", s)
	}
	if !strings.Contains(s, "2.0 K of 12.7 K") {
		t.Fatalf("the header should show both sizes:\n%s", s)
	}
	key(a, "Esc")

	// A binary file is dumped up to the smaller hex limit.
	bin := append([]byte{0, 1, 2, 3}, make([]byte, 8000)...)
	os.WriteFile(filepath.Join(data, "big.bin"), bin, 0o644)
	a.reloadAll()
	a.tab().SetCursorOn("big.bin")
	key(a, "F3")
	v, ok = a.top().(*Viewer)
	if !ok || !v.hex {
		t.Fatalf("expected a hex dump, got %T hex=%v", a.top(), v.hex)
	}
	if int64(len(v.raw)) != viewerHexMaxBytes || !v.cut {
		t.Fatalf("hex read %d bytes (cut=%v), want %d", len(v.raw), v.cut, viewerHexMaxBytes)
	}
	key(a, "Esc")

	// The editor still refuses: it would have to write the whole file back.
	a.tab().SetCursorOn("big.log")
	key(a, "F4")
	if len(a.dialogs) > 0 {
		t.Fatalf("the editor must not open a file it cannot hold, got %T", a.top())
	}
	if !strings.Contains(a.status, "cannot be edited") {
		t.Fatalf("the editor should say why: %q", a.status)
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
