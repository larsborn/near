package main

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// TextArea is a small multi-line edit control for dialogs (the Commands
// box of the user menu editor). Lines are rune slices; there is no undo.
type TextArea struct {
	lines    [][]rune
	row, col int
	top      int
}

func newTextArea(text string) *TextArea {
	t := &TextArea{}
	t.Set(text)
	return t
}

func (t *TextArea) Set(text string) {
	t.lines = nil
	for _, l := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		t.lines = append(t.lines, []rune(l))
	}
	t.row, t.col, t.top = 0, 0, 0
}

// String returns the text with "\n" between lines and no trailing newline.
func (t *TextArea) String() string {
	parts := make([]string, len(t.lines))
	for i, l := range t.lines {
		parts[i] = string(l)
	}
	return strings.Join(parts, "\n")
}

func (t *TextArea) clampCol() {
	if t.col > len(t.lines[t.row]) {
		t.col = len(t.lines[t.row])
	}
	if t.col < 0 {
		t.col = 0
	}
}

func (t *TextArea) insert(s string) {
	for _, r := range s {
		if r == '\r' {
			continue
		}
		if r == '\n' {
			t.newline()
			continue
		}
		l := t.lines[t.row]
		l = append(l[:t.col], append([]rune{r}, l[t.col:]...)...)
		t.lines[t.row] = l
		t.col++
	}
}

func (t *TextArea) newline() {
	l := t.lines[t.row]
	rest := append([]rune{}, l[t.col:]...)
	t.lines[t.row] = l[:t.col]
	t.lines = append(t.lines[:t.row+1], append([][]rune{rest}, t.lines[t.row+1:]...)...)
	t.row++
	t.col = 0
}

// HandleKey edits the text; it returns false for keys it does not use.
func (t *TextArea) HandleKey(ev *tcell.EventKey, a *App) bool {
	switch keyName(ev) {
	case "Left":
		if t.col > 0 {
			t.col--
		} else if t.row > 0 {
			t.row--
			t.col = len(t.lines[t.row])
		}
	case "Right":
		if t.col < len(t.lines[t.row]) {
			t.col++
		} else if t.row < len(t.lines)-1 {
			t.row++
			t.col = 0
		}
	case "Up":
		if t.row > 0 {
			t.row--
			t.clampCol()
		}
	case "Down":
		if t.row < len(t.lines)-1 {
			t.row++
			t.clampCol()
		}
	case "Home":
		t.col = 0
	case "End":
		t.col = len(t.lines[t.row])
	case "Enter":
		t.newline()
	case "Backspace":
		if t.col > 0 {
			l := t.lines[t.row]
			t.lines[t.row] = append(l[:t.col-1], l[t.col:]...)
			t.col--
		} else if t.row > 0 {
			prev := t.lines[t.row-1]
			t.col = len(prev)
			t.lines[t.row-1] = append(prev, t.lines[t.row]...)
			t.lines = append(t.lines[:t.row], t.lines[t.row+1:]...)
			t.row--
		}
	case "Delete":
		l := t.lines[t.row]
		if t.col < len(l) {
			t.lines[t.row] = append(l[:t.col], l[t.col+1:]...)
		} else if t.row < len(t.lines)-1 {
			t.lines[t.row] = append(l, t.lines[t.row+1]...)
			t.lines = append(t.lines[:t.row+1], t.lines[t.row+2:]...)
		}
	case "Ctrl+Y":
		if len(t.lines) > 1 {
			t.lines = append(t.lines[:t.row], t.lines[t.row+1:]...)
			if t.row >= len(t.lines) {
				t.row = len(t.lines) - 1
			}
		} else {
			t.lines[0] = nil
		}
		t.clampCol()
	case "Ctrl+V", "Shift+Insert":
		if a != nil {
			t.insert(a.pasteText())
		}
	default:
		if ev.Key() == tcell.KeyRune && ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt|tcell.ModMeta) == 0 {
			t.insert(string(ev.Rune()))
			return true
		}
		return false
	}
	return true
}

// Draw paints the text in the given box and places the cursor when focused.
func (t *TextArea) Draw(a *App, x, y, w, h int, st tcell.Style, focused bool) {
	if h < 1 {
		return
	}
	if t.row < t.top {
		t.top = t.row
	}
	if t.row >= t.top+h {
		t.top = t.row - h + 1
	}
	a.fill(x, y, w, h, st)
	for i := 0; i < h; i++ {
		li := t.top + i
		if li >= len(t.lines) {
			break
		}
		a.print(x, y+i, w, st, fitLeft(string(t.lines[li]), w))
	}
	if focused {
		cx := runewidth.StringWidth(string(t.lines[t.row][:t.col]))
		if cx < w {
			a.screen.ShowCursor(x+cx, y+t.row-t.top)
		}
	}
}
