package main

import (
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// Dialog is a modal layer drawn over the panels that receives all keys.
type Dialog interface {
	HandleKey(a *App, ev *tcell.EventKey)
	Draw(a *App)
}

// ---------------------------------------------------------------- drawing

func (a *App) print(x, y, maxw int, st tcell.Style, s string) int {
	if maxw <= 0 {
		return 0
	}
	col := 0
	for _, r := range s {
		if r == '\t' {
			r = ' '
		}
		if r < ' ' {
			continue
		}
		rw := runewidth.RuneWidth(r)
		if rw == 0 {
			continue
		}
		if col+rw > maxw {
			break
		}
		a.screen.SetContent(x+col, y, r, nil, st)
		col += rw
	}
	return col
}

func (a *App) fill(x, y, w, h int, st tcell.Style) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			a.screen.SetContent(xx, yy, ' ', nil, st)
		}
	}
}

func (a *App) hline(x, y, w int, st tcell.Style, r rune) {
	for i := 0; i < w; i++ {
		a.screen.SetContent(x+i, y, r, nil, st)
	}
}

func fitLeft(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return runewidth.Truncate(s, w, "…")
}

func fitRight(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return "…" + runewidth.TruncateLeft(s, runewidth.StringWidth(s)-w+1, "")
}

func padRight(s string, w int) string {
	return runewidth.FillRight(fitLeft(s, w), w)
}

func padLeft(s string, w int) string {
	return runewidth.FillLeft(fitLeft(s, w), w)
}

// drawBox fills an area, draws a single-line frame and centres a title.
func (a *App) drawBox(x, y, w, h int, st tcell.Style, title string) {
	a.fill(x, y, w, h, st)
	for i := 1; i < w-1; i++ {
		a.screen.SetContent(x+i, y, '─', nil, st)
		a.screen.SetContent(x+i, y+h-1, '─', nil, st)
	}
	for i := 1; i < h-1; i++ {
		a.screen.SetContent(x, y+i, '│', nil, st)
		a.screen.SetContent(x+w-1, y+i, '│', nil, st)
	}
	a.screen.SetContent(x, y, '┌', nil, st)
	a.screen.SetContent(x+w-1, y, '┐', nil, st)
	a.screen.SetContent(x, y+h-1, '└', nil, st)
	a.screen.SetContent(x+w-1, y+h-1, '┘', nil, st)
	if title != "" {
		t := " " + fitLeft(title, w-4) + " "
		tx := x + (w-runewidth.StringWidth(t))/2
		a.print(tx, y, w-2, a.theme.DialogTitle, t)
	}
}

func (a *App) center(w, h int) (int, int) {
	x := (a.w - w) / 2
	y := (a.h - h) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y
}

// ---------------------------------------------------------------- LineEdit

// LineEdit is a single-line text editor shared by all input dialogs.
type LineEdit struct {
	Text []rune
	Cur  int
	off  int
}

func newLineEdit(s string) *LineEdit {
	e := &LineEdit{}
	e.Set(s)
	return e
}

func (e *LineEdit) String() string { return string(e.Text) }

func (e *LineEdit) Set(s string) {
	e.Text = []rune(s)
	e.Cur = len(e.Text)
}

func (e *LineEdit) wordLeft() int {
	i := e.Cur
	for i > 0 && !isWordRune(e.Text[i-1]) {
		i--
	}
	for i > 0 && isWordRune(e.Text[i-1]) {
		i--
	}
	return i
}

func (e *LineEdit) wordRight() int {
	i := e.Cur
	for i < len(e.Text) && isWordRune(e.Text[i]) {
		i++
	}
	for i < len(e.Text) && !isWordRune(e.Text[i]) {
		i++
	}
	return i
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// HandleKey returns true when the key was consumed.
func (e *LineEdit) HandleKey(ev *tcell.EventKey) bool {
	switch keyName(ev) {
	case "Left":
		if e.Cur > 0 {
			e.Cur--
		}
	case "Right":
		if e.Cur < len(e.Text) {
			e.Cur++
		}
	case "Ctrl+Left", "Alt+B":
		e.Cur = e.wordLeft()
	case "Ctrl+Right", "Alt+F":
		e.Cur = e.wordRight()
	case "Home", "Ctrl+A":
		e.Cur = 0
	case "End", "Ctrl+E":
		e.Cur = len(e.Text)
	case "Backspace":
		if e.Cur > 0 {
			e.Text = append(e.Text[:e.Cur-1], e.Text[e.Cur:]...)
			e.Cur--
		}
	case "Delete":
		if e.Cur < len(e.Text) {
			e.Text = append(e.Text[:e.Cur], e.Text[e.Cur+1:]...)
		}
	case "Ctrl+U":
		e.Text = e.Text[:0]
		e.Cur = 0
	case "Ctrl+K":
		e.Text = e.Text[:e.Cur]
	case "Ctrl+W", "Alt+Backspace", "Ctrl+Backspace":
		i := e.wordLeft()
		e.Text = append(e.Text[:i], e.Text[e.Cur:]...)
		e.Cur = i
	default:
		if ev.Key() == tcell.KeyRune && ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt|tcell.ModMeta) == 0 {
			r := ev.Rune()
			e.Text = append(e.Text[:e.Cur], append([]rune{r}, e.Text[e.Cur:]...)...)
			e.Cur++
			return true
		}
		return false
	}
	return true
}

func (e *LineEdit) Draw(a *App, x, y, w int, st tcell.Style, focused bool) {
	if w < 1 {
		return
	}
	if e.Cur < e.off {
		e.off = e.Cur
	}
	if e.Cur >= e.off+w {
		e.off = e.Cur - w + 1
	}
	if e.off > len(e.Text) {
		e.off = len(e.Text)
	}
	end := e.off + w
	if end > len(e.Text) {
		end = len(e.Text)
	}
	a.fill(x, y, w, 1, st)
	a.print(x, y, w, st, string(e.Text[e.off:end]))
	if focused {
		a.screen.ShowCursor(x+e.Cur-e.off, y)
	}
}

// ---------------------------------------------------------------- InputDialog

type InputDialog struct {
	Title  string
	Prompt string
	Edit   *LineEdit
	Width  int
	OnOK   func(text string)
}

func (a *App) input(title, prompt, initial string, onOK func(string)) *InputDialog {
	d := &InputDialog{Title: title, Prompt: prompt, Edit: newLineEdit(initial), OnOK: onOK}
	a.push(d)
	return d
}

func (d *InputDialog) HandleKey(a *App, ev *tcell.EventKey) {
	switch keyName(ev) {
	case "Enter":
		a.pop()
		if d.OnOK != nil {
			d.OnOK(d.Edit.String())
		}
	case "Esc":
		a.pop()
	default:
		d.Edit.HandleKey(ev)
	}
}

func (d *InputDialog) Draw(a *App) {
	w := d.Width
	if w == 0 {
		w = 64
	}
	w = clampInt(w, 24, a.w-2)
	h := 6
	x, y := a.center(w, h)
	th := a.theme
	a.drawBox(x, y, w, h, th.Dialog, d.Title)
	a.print(x+2, y+1, w-4, th.Dialog, d.Prompt)
	d.Edit.Draw(a, x+2, y+2, w-4, th.InputFocus, true)
	a.print(x+2, y+4, w-4, th.DialogHint, "Enter: OK   Esc: cancel")
}

// ---------------------------------------------------------------- ConfirmDialog

type ConfirmDialog struct {
	Title    string
	Lines    []string
	Buttons  []string
	Active   int
	OnResult func(button int) // -1 on Esc
}

func (a *App) confirm(title string, lines []string, buttons []string, onResult func(int)) {
	a.push(&ConfirmDialog{Title: title, Lines: lines, Buttons: buttons, OnResult: onResult})
}

func (a *App) message(title string, lines ...string) {
	a.confirm(title, lines, []string{"OK"}, nil)
}

func (d *ConfirmDialog) finish(a *App, result int) {
	a.pop()
	if d.OnResult != nil {
		d.OnResult(result)
	}
}

func (d *ConfirmDialog) HandleKey(a *App, ev *tcell.EventKey) {
	switch keyName(ev) {
	case "Enter", "Space":
		d.finish(a, d.Active)
	case "Esc":
		d.finish(a, -1)
	case "Left", "Shift+Tab", "Up":
		d.Active = (d.Active + len(d.Buttons) - 1) % len(d.Buttons)
	case "Right", "Tab", "Down":
		d.Active = (d.Active + 1) % len(d.Buttons)
	default:
		if ev.Key() == tcell.KeyRune {
			r := unicode.ToLower(ev.Rune())
			for i, b := range d.Buttons {
				if len(b) > 0 && unicode.ToLower([]rune(b)[0]) == r {
					d.finish(a, i)
					return
				}
			}
		}
	}
}

func (d *ConfirmDialog) Draw(a *App) {
	th := a.theme
	w := runewidth.StringWidth(d.Title) + 6
	for _, l := range d.Lines {
		if lw := runewidth.StringWidth(l) + 4; lw > w {
			w = lw
		}
	}
	bw := 0
	for _, b := range d.Buttons {
		bw += runewidth.StringWidth(b) + 6
	}
	if bw+2 > w {
		w = bw + 2
	}
	w = clampInt(w, 30, a.w-2)
	h := len(d.Lines) + 4
	x, y := a.center(w, h)
	a.drawBox(x, y, w, h, th.Dialog, d.Title)
	for i, l := range d.Lines {
		a.print(x+2, y+1+i, w-4, th.Dialog, fitLeft(l, w-4))
	}
	bx := x + (w-bw)/2
	by := y + h - 2
	for i, b := range d.Buttons {
		label := "[ " + b + " ]"
		st := th.Button
		if i == d.Active {
			st = th.ButtonActive
		}
		a.print(bx, by, w, st, label)
		bx += runewidth.StringWidth(b) + 6
	}
}

// ---------------------------------------------------------------- ListDialog

type ListItem struct {
	Label  string
	Hotkey rune
	Right  string
	Data   any
	Sep    bool
}

type ListDialog struct {
	Title      string
	Items      []ListItem
	Cursor     int
	Top        int
	Width      int
	Hint       string
	Searchable bool
	OnSelect   func(i int, it ListItem)
	OnKey      func(a *App, d *ListDialog, ev *tcell.EventKey) bool
	filter     []rune
}

func (a *App) list(title string, items []ListItem, onSelect func(int, ListItem)) *ListDialog {
	d := &ListDialog{Title: title, Items: items, OnSelect: onSelect}
	a.push(d)
	return d
}

func (d *ListDialog) visibleRows(a *App) int {
	rows := len(d.Items)
	if rows > a.h-6 {
		rows = a.h - 6
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (d *ListDialog) move(delta int, rows int) {
	if len(d.Items) == 0 {
		return
	}
	d.Cursor = clampInt(d.Cursor+delta, 0, len(d.Items)-1)
	for d.Items[d.Cursor].Sep {
		if delta < 0 && d.Cursor > 0 {
			d.Cursor--
		} else if d.Cursor < len(d.Items)-1 {
			d.Cursor++
		} else {
			break
		}
	}
	_ = rows
}

func (d *ListDialog) select_(a *App) {
	if d.Cursor < 0 || d.Cursor >= len(d.Items) || d.Items[d.Cursor].Sep {
		return
	}
	a.pop()
	if d.OnSelect != nil {
		d.OnSelect(d.Cursor, d.Items[d.Cursor])
	}
}

func (d *ListDialog) HandleKey(a *App, ev *tcell.EventKey) {
	if d.OnKey != nil && d.OnKey(a, d, ev) {
		return
	}
	rows := d.visibleRows(a)
	switch keyName(ev) {
	case "Up":
		d.move(-1, rows)
	case "Down":
		d.move(1, rows)
	case "PgUp":
		d.move(-rows, rows)
	case "PgDn":
		d.move(rows, rows)
	case "Home":
		d.Cursor = 0
		d.move(0, rows)
	case "End":
		d.Cursor = len(d.Items) - 1
		d.move(0, rows)
	case "Enter":
		d.select_(a)
	case "Esc":
		a.pop()
	case "Backspace":
		if len(d.filter) > 0 {
			d.filter = d.filter[:len(d.filter)-1]
		}
	default:
		if ev.Key() != tcell.KeyRune || ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) != 0 {
			return
		}
		r := unicode.ToLower(ev.Rune())
		for i, it := range d.Items {
			if it.Hotkey != 0 && unicode.ToLower(it.Hotkey) == r {
				d.Cursor = i
				d.select_(a)
				return
			}
		}
		if d.Searchable {
			d.filter = append(d.filter, ev.Rune())
			f := strings.ToLower(string(d.filter))
			for i, it := range d.Items {
				if strings.HasPrefix(strings.ToLower(it.Label), f) {
					d.Cursor = i
					return
				}
			}
			for i, it := range d.Items {
				if strings.Contains(strings.ToLower(it.Label), f) {
					d.Cursor = i
					return
				}
			}
		}
	}
}

func (d *ListDialog) Draw(a *App) {
	th := a.theme
	rows := d.visibleRows(a)
	if d.Cursor < d.Top {
		d.Top = d.Cursor
	}
	if d.Cursor >= d.Top+rows {
		d.Top = d.Cursor - rows + 1
	}
	w := d.Width
	if w == 0 {
		w = runewidth.StringWidth(d.Title) + 6
		for _, it := range d.Items {
			lw := runewidth.StringWidth(it.Label) + 6
			if it.Right != "" {
				lw += runewidth.StringWidth(it.Right) + 2
			}
			if lw > w {
				w = lw
			}
		}
	}
	w = clampInt(w, 24, a.w-2)
	h := rows + 2
	if d.Hint != "" || len(d.filter) > 0 {
		h++
	}
	x, y := a.center(w, h)
	a.drawBox(x, y, w, h, th.Dialog, d.Title)
	for i := 0; i < rows; i++ {
		idx := d.Top + i
		if idx >= len(d.Items) {
			break
		}
		it := d.Items[idx]
		ry := y + 1 + i
		if it.Sep {
			a.hline(x+1, ry, w-2, th.Dialog, '─')
			continue
		}
		st := th.Dialog
		hot := th.Hotkey
		if idx == d.Cursor {
			st = th.ListCursor
			hot = th.ListCursor.Bold(true)
		}
		a.fill(x+1, ry, w-2, 1, st)
		label := fitLeft(it.Label, w-4)
		if it.Right != "" {
			rw := runewidth.StringWidth(it.Right)
			a.print(x+w-2-rw, ry, rw, st, it.Right)
			label = fitLeft(it.Label, w-6-rw)
		}
		cx := x + 2
		drawn := false
		for _, r := range label {
			s := st
			if !drawn && it.Hotkey != 0 && unicode.ToLower(r) == unicode.ToLower(it.Hotkey) {
				s = hot
				drawn = true
			}
			cx += a.print(cx, ry, 1+runewidth.RuneWidth(r), s, string(r))
		}
	}
	if len(d.filter) > 0 {
		a.print(x+2, y+h-2, w-4, th.DialogHint, "Search: "+string(d.filter))
	} else if d.Hint != "" {
		a.print(x+2, y+h-2, w-4, th.DialogHint, fitLeft(d.Hint, w-4))
	}
}

// ---------------------------------------------------------------- TextDialog

// TextDialog is a small multi-line editor used for notes.
type TextDialog struct {
	Title    string
	Lines    [][]rune
	Row, Col int
	Top      int
	OnSave   func(text string)
	modified bool
}

func (a *App) textDialog(title, text string, onSave func(string)) *TextDialog {
	d := &TextDialog{Title: title, OnSave: onSave}
	for _, l := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		d.Lines = append(d.Lines, []rune(l))
	}
	if len(d.Lines) == 0 {
		d.Lines = [][]rune{{}}
	}
	d.Row = len(d.Lines) - 1
	d.Col = len(d.Lines[d.Row])
	a.push(d)
	return d
}

func (d *TextDialog) text() string {
	parts := make([]string, len(d.Lines))
	for i, l := range d.Lines {
		parts[i] = string(l)
	}
	return strings.TrimRight(strings.Join(parts, "\n"), "\n ")
}

func (d *TextDialog) size(a *App) (w, h int) {
	w = clampInt(70, 30, a.w-2)
	h = clampInt(14, 6, a.h-2)
	return
}

func (d *TextDialog) HandleKey(a *App, ev *tcell.EventKey) {
	_, h := d.size(a)
	rows := h - 3
	line := d.Lines[d.Row]
	switch keyName(ev) {
	case "Esc":
		if d.modified {
			a.confirm("Discard changes?", []string{"The note was changed."}, []string{"Discard", "Keep editing"}, func(r int) {
				if r == 0 {
					a.pop()
				}
			})
			return
		}
		a.pop()
	case "Ctrl+S", "F2":
		a.pop()
		if d.OnSave != nil {
			d.OnSave(d.text())
		}
	case "Up":
		if d.Row > 0 {
			d.Row--
		}
	case "Down":
		if d.Row < len(d.Lines)-1 {
			d.Row++
		}
	case "PgUp":
		d.Row = clampInt(d.Row-rows, 0, len(d.Lines)-1)
	case "PgDn":
		d.Row = clampInt(d.Row+rows, 0, len(d.Lines)-1)
	case "Left":
		if d.Col > 0 {
			d.Col--
		} else if d.Row > 0 {
			d.Row--
			d.Col = len(d.Lines[d.Row])
		}
	case "Right":
		if d.Col < len(line) {
			d.Col++
		} else if d.Row < len(d.Lines)-1 {
			d.Row++
			d.Col = 0
		}
	case "Home", "Ctrl+A":
		d.Col = 0
	case "End", "Ctrl+E":
		d.Col = len(line)
	case "Enter":
		rest := append([]rune{}, line[d.Col:]...)
		d.Lines[d.Row] = line[:d.Col]
		d.Lines = append(d.Lines[:d.Row+1], append([][]rune{rest}, d.Lines[d.Row+1:]...)...)
		d.Row++
		d.Col = 0
		d.modified = true
	case "Backspace":
		if d.Col > 0 {
			d.Lines[d.Row] = append(line[:d.Col-1], line[d.Col:]...)
			d.Col--
			d.modified = true
		} else if d.Row > 0 {
			prev := d.Lines[d.Row-1]
			d.Col = len(prev)
			d.Lines[d.Row-1] = append(prev, line...)
			d.Lines = append(d.Lines[:d.Row], d.Lines[d.Row+1:]...)
			d.Row--
			d.modified = true
		}
	case "Delete":
		if d.Col < len(line) {
			d.Lines[d.Row] = append(line[:d.Col], line[d.Col+1:]...)
			d.modified = true
		} else if d.Row < len(d.Lines)-1 {
			d.Lines[d.Row] = append(line, d.Lines[d.Row+1]...)
			d.Lines = append(d.Lines[:d.Row+1], d.Lines[d.Row+2:]...)
			d.modified = true
		}
	case "Ctrl+K":
		d.Lines[d.Row] = line[:d.Col]
		d.modified = true
	default:
		if ev.Key() == tcell.KeyRune && ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0 {
			r := ev.Rune()
			d.Lines[d.Row] = append(line[:d.Col], append([]rune{r}, line[d.Col:]...)...)
			d.Col++
			d.modified = true
		}
	}
	if d.Col > len(d.Lines[d.Row]) {
		d.Col = len(d.Lines[d.Row])
	}
}

func (d *TextDialog) Draw(a *App) {
	th := a.theme
	w, h := d.size(a)
	rows := h - 3
	x, y := a.center(w, h)
	if d.Row < d.Top {
		d.Top = d.Row
	}
	if d.Row >= d.Top+rows {
		d.Top = d.Row - rows + 1
	}
	a.drawBox(x, y, w, h, th.Dialog, d.Title)
	iw := w - 4
	for i := 0; i < rows; i++ {
		idx := d.Top + i
		if idx >= len(d.Lines) {
			break
		}
		a.fill(x+2, y+1+i, iw, 1, th.Input)
		a.print(x+2, y+1+i, iw, th.Input, string(d.Lines[idx]))
	}
	for i := len(d.Lines) - d.Top; i < rows; i++ {
		if i >= 0 {
			a.fill(x+2, y+1+i, iw, 1, th.Input)
		}
	}
	col := d.Col
	if col > iw-1 {
		col = iw - 1
	}
	a.screen.ShowCursor(x+2+col, y+1+d.Row-d.Top)
	a.print(x+2, y+h-2, w-4, th.DialogHint, "Ctrl+S / F2: save   Esc: cancel   Enter: new line")
}
