package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// Text selection in the console with the mouse, like in a terminal: drag
// with the left button over the console rows (the full console view or the
// band under the panels) to mark text in reading order; releasing the
// button copies it to the clipboard. The marking stays visible until the
// next key, click or new output.
type conSelection struct {
	active   bool
	dragging bool
	ax, ay   int // anchor (screen cells)
	cx, cy   int // current end
}

// ordered returns the selection's start and end in reading order.
func (s conSelection) ordered() (x1, y1, x2, y2 int) {
	if s.ay < s.cy || s.ay == s.cy && s.ax <= s.cx {
		return s.ax, s.ay, s.cx, s.cy
	}
	return s.cx, s.cy, s.ax, s.ay
}

// contains reports whether the cell at x, y is inside the selection.
func (s conSelection) contains(x, y int) bool {
	if !s.active {
		return false
	}
	x1, y1, x2, y2 := s.ordered()
	switch {
	case y < y1 || y > y2:
		return false
	case y1 == y2:
		return x >= x1 && x <= x2
	case y == y1:
		return x >= x1
	case y == y2:
		return x <= x2
	}
	return true
}

// inConsole reports whether a screen position lies on the console rows
// drawn last.
func (a *App) inConsole(x, y int) bool {
	r := a.conArea
	return r.h > 0 && x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

// consoleMouse handles mouse events over the console rows: wheel scrolling
// in the full view, and drag selection. It reports whether it used the event.
func (a *App) consoleMouse(ev *tcell.EventMouse, pressed, buttons tcell.ButtonMask) bool {
	x, y := ev.Position()
	switch {
	case a.sel.dragging:
		if buttons&tcell.ButtonPrimary != 0 {
			a.sel.cx, a.sel.cy = clampInt(x, 0, a.w-1), clampInt(y, a.conArea.y, a.conArea.y+a.conArea.h-1)
			return true
		}
		a.sel.dragging = false
		if text := a.selectedConsoleText(); text != "" {
			a.setClipboard(text)
			a.setStatus(fmt.Sprintf("Copied %d characters", len([]rune(text))))
		} else {
			a.sel.active = false
		}
		return true
	case !a.inConsole(x, y):
		return false
	case buttons&tcell.WheelUp != 0 && a.hideAll:
		a.console.scrollBy(3, a.w, a.conArea.h)
		return true
	case buttons&tcell.WheelDown != 0 && a.hideAll:
		a.console.scrollBy(-3, a.w, a.conArea.h)
		return true
	case pressed&tcell.ButtonPrimary != 0:
		a.sel = conSelection{active: true, dragging: true, ax: x, ay: y, cx: x, cy: y}
		return true
	}
	return a.hideAll
}

// selectedConsoleText returns the marked text: whole rows between the ends,
// trailing blanks dropped, rows joined with newlines.
func (a *App) selectedConsoleText() string {
	if !a.sel.active {
		return ""
	}
	x1, y1, x2, y2 := a.sel.ordered()
	var lines []string
	for y := y1; y <= y2; y++ {
		row := y - a.conRowTop
		if row < 0 || row >= len(a.conRows) {
			continue
		}
		text := []rune(plainRow(a.conRows[row]))
		from, to := 0, len(text)
		if y == y1 {
			from = clampInt(x1, 0, len(text))
		}
		if y == y2 {
			to = clampInt(x2+1, 0, len(text))
		}
		if from > to {
			from = to
		}
		lines = append(lines, strings.TrimRight(string(text[from:to]), " "))
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// plainRow joins a styled console row into plain text.
func plainRow(l conLine) string {
	var b strings.Builder
	for _, s := range l {
		b.WriteString(s.text)
	}
	return b.String()
}

// drawConsoleSelection inverts the marked cells after the console rows
// were drawn.
func (a *App) drawConsoleSelection() {
	if !a.sel.active {
		return
	}
	for y := a.conArea.y; y < a.conArea.y+a.conArea.h; y++ {
		for x := 0; x < a.w; x++ {
			if !a.sel.contains(x, y) {
				continue
			}
			r, comb, st, _ := a.screen.GetContent(x, y)
			a.screen.SetContent(x, y, r, comb, st.Reverse(true))
		}
	}
}
