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

// mouseDialog is a dialog that also reacts to the mouse.
type mouseDialog interface {
	HandleMouse(a *App, ev *tcell.EventMouse)
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

// drawBox draws a FAR dialog: the body extends two columns and one row
// beyond the double-line frame, a shadow falls to the right and below, and
// the title sits centred in the top border.
func (a *App) drawBox(x, y, w, h int, st tcell.Style, title string) {
	// shadow: darken what is underneath
	shadow := func(sx, sy, sw, sh int) {
		for yy := sy; yy < sy+sh; yy++ {
			for xx := sx; xx < sx+sw; xx++ {
				if xx < 0 || yy < 0 || xx >= a.w || yy >= a.h {
					continue
				}
				r, _, _, _ := a.screen.GetContent(xx, yy)
				a.screen.SetContent(xx, yy, r, nil, tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorGray))
			}
		}
	}
	shadow(x+w+2, y, 2, h+2)
	shadow(x-2+2, y+h+1, w+4, 1)
	// body
	a.fill(maxInt(0, x-2), maxInt(0, y-1), w+4, h+2, st)
	a.fill(x, y, w, h, st)
	for i := 1; i < w-1; i++ {
		a.screen.SetContent(x+i, y, '═', nil, st)
		a.screen.SetContent(x+i, y+h-1, '═', nil, st)
	}
	for i := 1; i < h-1; i++ {
		a.screen.SetContent(x, y+i, '║', nil, st)
		a.screen.SetContent(x+w-1, y+i, '║', nil, st)
	}
	a.screen.SetContent(x, y, '╔', nil, st)
	a.screen.SetContent(x+w-1, y, '╗', nil, st)
	a.screen.SetContent(x, y+h-1, '╚', nil, st)
	a.screen.SetContent(x+w-1, y+h-1, '╝', nil, st)
	if title != "" {
		t := " " + fitLeft(title, w-4) + " "
		tx := x + (w-runewidth.StringWidth(t))/2
		a.print(tx, y, w-2, a.theme.DialogTitle, t)
	}
}

// separator draws a horizontal rule across a box at row y.
func (a *App) separator(x, y, w int, st tcell.Style) {
	a.hline(x+1, y, w-2, st, '─')
	a.screen.SetContent(x, y, '╟', nil, st)
	a.screen.SetContent(x+w-1, y, '╢', nil, st)
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

// LineEdit is a single-line text editor shared by all input dialogs and the
// command line. Shift with the movement keys selects; Ctrl+Shift+Left/Right
// select by word; typing replaces the selection.
type LineEdit struct {
	Text   []rune
	Cur    int
	Anchor int // selection anchor, -1 when nothing is selected
	off    int
}

func newLineEdit(s string) *LineEdit {
	e := &LineEdit{Anchor: -1}
	e.Set(s)
	return e
}

func (e *LineEdit) String() string { return string(e.Text) }

func (e *LineEdit) Set(s string) {
	e.Text = []rune(s)
	e.Cur = len(e.Text)
	e.Anchor = -1
}

// Selection returns the selected range [from, to).
func (e *LineEdit) Selection() (from, to int, ok bool) {
	if e.Anchor < 0 || e.Anchor == e.Cur {
		return 0, 0, false
	}
	from, to = e.Anchor, e.Cur
	if from > to {
		from, to = to, from
	}
	return from, to, true
}

func (e *LineEdit) SelectedText() string {
	if from, to, ok := e.Selection(); ok {
		return string(e.Text[from:to])
	}
	return ""
}

// deleteSelection removes the selected text and reports whether there was one.
func (e *LineEdit) deleteSelection() bool {
	from, to, ok := e.Selection()
	if !ok {
		e.Anchor = -1
		return false
	}
	e.Text = append(e.Text[:from], e.Text[to:]...)
	e.Cur = from
	e.Anchor = -1
	return true
}

func (e *LineEdit) insert(s string) {
	e.deleteSelection()
	r := []rune(s)
	e.Text = append(e.Text[:e.Cur], append(r, e.Text[e.Cur:]...)...)
	e.Cur += len(r)
}

// move runs a cursor movement, extending the selection when shift is held
// and dropping it otherwise.
func (e *LineEdit) move(shift bool, to int) {
	if shift {
		if e.Anchor < 0 {
			e.Anchor = e.Cur
		}
	} else {
		e.Anchor = -1
	}
	e.Cur = clampInt(to, 0, len(e.Text))
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

// HandleKey returns true when the key was consumed. The app is used for
// the clipboard and may be nil.
func (e *LineEdit) HandleKey(ev *tcell.EventKey, a *App) bool {
	name := keyName(ev)
	shift := strings.Contains(name, "Shift+")
	switch strings.ReplaceAll(name, "Shift+", "") {
	case "Left":
		if _, _, ok := e.Selection(); ok && !shift {
			from, _, _ := e.Selection()
			e.move(false, from)
		} else {
			e.move(shift, e.Cur-1)
		}
	case "Right":
		if _, _, ok := e.Selection(); ok && !shift {
			_, to, _ := e.Selection()
			e.move(false, to)
		} else {
			e.move(shift, e.Cur+1)
		}
	case "Ctrl+Left", "Alt+B":
		e.move(shift, e.wordLeft())
	case "Ctrl+Right", "Alt+F":
		e.move(shift, e.wordRight())
	case "Home":
		e.move(shift, 0)
	case "End":
		e.move(shift, len(e.Text))
	case "Ctrl+A":
		e.Anchor = 0
		e.Cur = len(e.Text)
	case "Ctrl+E":
		e.move(shift, len(e.Text))
	case "Backspace":
		if !e.deleteSelection() && e.Cur > 0 {
			e.Text = append(e.Text[:e.Cur-1], e.Text[e.Cur:]...)
			e.Cur--
		}
	case "Delete":
		if !e.deleteSelection() && e.Cur < len(e.Text) {
			e.Text = append(e.Text[:e.Cur], e.Text[e.Cur+1:]...)
		}
	case "Ctrl+U":
		e.Text = e.Text[:0]
		e.Cur = 0
		e.Anchor = -1
	case "Ctrl+K":
		if !e.deleteSelection() {
			e.Text = e.Text[:e.Cur]
		}
	case "Ctrl+W", "Alt+Backspace", "Ctrl+Backspace":
		if !e.deleteSelection() {
			i := e.wordLeft()
			e.Text = append(e.Text[:i], e.Text[e.Cur:]...)
			e.Cur = i
		}
	case "Ctrl+C", "Ctrl+Insert":
		if s := e.SelectedText(); s != "" && a != nil {
			a.setClipboard(s)
		}
	case "Ctrl+X":
		if s := e.SelectedText(); s != "" && a != nil {
			a.setClipboard(s)
			e.deleteSelection()
		}
	case "Ctrl+V", "Insert", "Shift+Insert":
		if a == nil {
			break
		}
		if s := a.pasteText(); s != "" {
			e.insert(firstLine(s))
		} else {
			a.screen.GetClipboard()
		}
	default:
		if ev.Key() == tcell.KeyRune && ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt|tcell.ModMeta) == 0 {
			e.insert(string(ev.Rune()))
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
	from, to, hasSel := e.Selection()
	for i := e.off; i < end; i++ {
		cs := st
		if hasSel && i >= from && i < to {
			cs = st.Reverse(true)
		}
		a.print(x+i-e.off, y, 1, cs, string(e.Text[i]))
	}
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
		d.Edit.HandleKey(ev, a)
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
	Dim    bool // disabled entry, shown greyed
}

type ListDialog struct {
	Title      string
	Items      []ListItem
	Cursor     int
	Top        int
	Width      int
	Hint       string
	Searchable bool
	Menu       bool // FAR menu colours (teal) instead of dialog grey
	OnSelect   func(i int, it ListItem)
	OnKey      func(a *App, d *ListDialog, ev *tcell.EventKey) bool
	OnClose    func() // called when the list is left with Esc
	filter     []rune
	box        rect // where the list was drawn, for the mouse
	rowsShown  int
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
		if d.OnClose != nil {
			d.OnClose()
		}
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
		if hw := runewidth.StringWidth(d.Hint) + 4; hw > w {
			w = hw // the key hints of a menu must stay readable
		}
	}
	w = clampInt(w, 24, a.w-2)
	h := rows + 2
	if d.Hint != "" || len(d.filter) > 0 {
		h++
	}
	x, y := a.center(w, h)
	d.box, d.rowsShown = rect{x, y, w, h}, rows
	body, cursorSt, hotSt, dimSt, hintSt := th.Dialog, th.ListCursor, th.Hotkey, th.ListDim, th.DialogHint
	if d.Menu {
		body, cursorSt, hotSt, dimSt, hintSt = th.Menu, th.MenuCursor, th.MenuHot, th.MenuDim, th.MenuDim
	}
	a.drawBox(x, y, w, h, body, d.Title)
	for i := 0; i < rows; i++ {
		idx := d.Top + i
		if idx >= len(d.Items) {
			break
		}
		it := d.Items[idx]
		ry := y + 1 + i
		if it.Sep {
			a.hline(x+1, ry, w-2, body, '─')
			a.screen.SetContent(x, ry, '╟', nil, body)
			a.screen.SetContent(x+w-1, ry, '╢', nil, body)
			continue
		}
		st := body
		hot := hotSt
		if it.Dim {
			st, hot = dimSt, dimSt
		}
		if idx == d.Cursor {
			st = cursorSt
			hot = cursorSt.Bold(true)
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
		a.print(x+2, y+h-2, w-4, hintSt, "Search: "+string(d.filter))
	} else if d.Hint != "" {
		a.print(x+2, y+h-2, w-4, hintSt, fitLeft(d.Hint, w-4))
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

// HandleMouse: a click on an entry activates it, the wheel scrolls, a
// click outside the list closes it like Esc.
func (d *ListDialog) HandleMouse(a *App, ev *tcell.EventMouse) {
	x, y := ev.Position()
	b := ev.Buttons()
	switch {
	case b&tcell.WheelUp != 0:
		d.move(-1, d.rowsShown)
	case b&tcell.WheelDown != 0:
		d.move(1, d.rowsShown)
	case b&(tcell.ButtonPrimary|tcell.ButtonSecondary) != 0:
		inside := x >= d.box.x && x < d.box.x+d.box.w && y >= d.box.y && y < d.box.y+d.box.h
		if !inside {
			a.pop()
			if d.OnClose != nil {
				d.OnClose()
			}
			return
		}
		row := y - (d.box.y + 1)
		if row < 0 || row >= d.rowsShown || d.Top+row >= len(d.Items) || d.Items[d.Top+row].Sep {
			return
		}
		d.Cursor = d.Top + row
		if b&tcell.ButtonPrimary != 0 {
			d.select_(a)
		}
	}
}
