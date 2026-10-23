package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// Editor is the built-in full-screen text editor, modelled on FAR's.
type Editor struct {
	path      string
	lines     [][]rune
	row, col  int // cursor (rune index)
	wantCol   int // preferred column when moving vertically
	top       int // first visible line
	left      int // first visible display column
	anchor    [2]int
	selected  bool
	modified  bool
	crlf      bool
	bom       bool
	finalNL   bool
	overwrite bool
	tabSize   int
	undo      []editRec
	redo      []editRec
	search    string
	replace   string
	saved     bool
	OnClose   func(saved bool)
	isNew     bool
}

// editRec is one reversible change: at (row,col) text del was replaced by
// ins. The cursor position before the change is kept for undo.
type editRec struct {
	row, col       int
	del, ins       string
	curRow, curCol int
	typing         bool // single-character typing, may be merged with the previous record
}

// ---------------------------------------------------------------- open / save

func newEditor(path string, tabSize int) (*Editor, error) {
	e := &Editor{path: path, tabSize: tabSize, finalNL: true}
	if e.tabSize <= 0 {
		e.tabSize = 4
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		e.isNew = true
		e.lines = [][]rune{{}}
		return e, nil
	}
	if len(data) > viewerMaxBytes {
		return nil, fmt.Errorf("%s is larger than %d MB", path, viewerMaxBytes>>20)
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, fmt.Errorf("%s is a binary file", path)
	}
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		e.bom = true
		data = data[3:]
	}
	text := string(data)
	e.crlf = strings.Contains(text, "\r\n")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	e.finalNL = strings.HasSuffix(text, "\n") || text == ""
	if e.finalNL && text != "" {
		text = text[:len(text)-1]
	}
	for _, l := range strings.Split(text, "\n") {
		e.lines = append(e.lines, []rune(l))
	}
	return e, nil
}

func (e *Editor) text() string {
	parts := make([]string, len(e.lines))
	for i, l := range e.lines {
		parts[i] = string(l)
	}
	s := strings.Join(parts, "\n")
	if e.finalNL {
		s += "\n"
	}
	if e.crlf {
		s = strings.ReplaceAll(s, "\n", "\r\n")
	}
	if e.bom {
		s = "\xEF\xBB\xBF" + s
	}
	return s
}

func (e *Editor) save(a *App, path string) bool {
	if err := os.WriteFile(path, []byte(e.text()), 0o644); err != nil {
		a.setErr(err)
		return false
	}
	e.path = path
	e.modified, e.saved, e.isNew = false, true, false
	a.setStatus("Saved " + path)
	return true
}

// ---------------------------------------------------------------- positions

func (e *Editor) clampCursor() {
	e.row = clampInt(e.row, 0, len(e.lines)-1)
	e.col = clampInt(e.col, 0, len(e.lines[e.row]))
}

// advance returns the position reached from (r,c) after inserting s.
func advance(r, c int, s string) (int, int) {
	for _, ch := range s {
		if ch == '\n' {
			r++
			c = 0
		} else {
			c++
		}
	}
	return r, c
}

// slice returns the text between two positions (r1,c1) <= (r2,c2).
func (e *Editor) slice(r1, c1, r2, c2 int) string {
	if r1 == r2 {
		return string(e.lines[r1][c1:c2])
	}
	var b strings.Builder
	b.WriteString(string(e.lines[r1][c1:]))
	for r := r1 + 1; r < r2; r++ {
		b.WriteByte('\n')
		b.WriteString(string(e.lines[r]))
	}
	b.WriteByte('\n')
	b.WriteString(string(e.lines[r2][:c2]))
	return b.String()
}

// edit replaces the text between (r1,c1) and (r2,c2) by ins, moves the
// cursor to the end of the insertion and returns the undo record.
func (e *Editor) edit(r1, c1, r2, c2 int, ins string) editRec {
	rec := editRec{row: r1, col: c1, del: e.slice(r1, c1, r2, c2), ins: ins, curRow: e.row, curCol: e.col}
	tail := append([]rune{}, e.lines[r2][c2:]...)
	head := e.lines[r1][:c1]
	insLines := strings.Split(ins, "\n")
	newLines := make([][]rune, 0, len(insLines))
	for i, l := range insLines {
		var line []rune
		if i == 0 {
			line = append(append([]rune{}, head...), []rune(l)...)
		} else {
			line = []rune(l)
		}
		if i == len(insLines)-1 {
			line = append(line, tail...)
		}
		newLines = append(newLines, line)
	}
	rest := append([][]rune{}, e.lines[r2+1:]...)
	e.lines = append(append(e.lines[:r1], newLines...), rest...)
	e.row, e.col = advance(r1, c1, ins)
	e.wantCol = e.col
	e.modified = true
	e.selected = false
	return rec
}

// change applies an edit and records it for undo.
func (e *Editor) change(r1, c1, r2, c2 int, ins string, typing bool) {
	rec := e.edit(r1, c1, r2, c2, ins)
	rec.typing = typing
	e.redo = e.redo[:0]
	if typing && len(e.undo) > 0 {
		last := &e.undo[len(e.undo)-1]
		lr, lc := advance(last.row, last.col, last.ins)
		if last.typing && last.del == "" && rec.del == "" && lr == rec.row && lc == rec.col && !strings.Contains(rec.ins, "\n") {
			last.ins += rec.ins
			return
		}
	}
	e.undo = append(e.undo, rec)
}

func (e *Editor) undoOne() bool {
	if len(e.undo) == 0 {
		return false
	}
	rec := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	r2, c2 := advance(rec.row, rec.col, rec.ins)
	inv := e.edit(rec.row, rec.col, r2, c2, rec.del)
	inv.curRow, inv.curCol = rec.curRow, rec.curCol
	e.row, e.col = rec.curRow, rec.curCol
	e.clampCursor()
	e.redo = append(e.redo, editRec{row: rec.row, col: rec.col, del: rec.del, ins: rec.ins, curRow: rec.curRow, curCol: rec.curCol})
	return true
}

func (e *Editor) redoOne() bool {
	if len(e.redo) == 0 {
		return false
	}
	rec := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	r2, c2 := advance(rec.row, rec.col, rec.del)
	e.edit(rec.row, rec.col, r2, c2, rec.ins)
	e.undo = append(e.undo, rec)
	return true
}

// ---------------------------------------------------------------- selection

func (e *Editor) selRange() (r1, c1, r2, c2 int, ok bool) {
	if !e.selected {
		return 0, 0, 0, 0, false
	}
	r1, c1, r2, c2 = e.anchor[0], e.anchor[1], e.row, e.col
	if r1 > r2 || (r1 == r2 && c1 > c2) {
		r1, c1, r2, c2 = r2, c2, r1, c1
	}
	if r1 == r2 && c1 == c2 {
		return 0, 0, 0, 0, false
	}
	return r1, c1, r2, c2, true
}

func (e *Editor) deleteSelection() bool {
	r1, c1, r2, c2, ok := e.selRange()
	if !ok {
		return false
	}
	e.change(r1, c1, r2, c2, "", false)
	return true
}

func (e *Editor) selectedText() string {
	r1, c1, r2, c2, ok := e.selRange()
	if !ok {
		return ""
	}
	return e.slice(r1, c1, r2, c2)
}

// moveWith starts or extends the selection when shift is held, otherwise
// drops it, then runs the movement.
func (e *Editor) moveWith(shift bool, move func()) {
	if shift && !e.selected {
		e.anchor = [2]int{e.row, e.col}
		e.selected = true
	} else if !shift {
		e.selected = false
	}
	move()
	e.clampCursor()
}

// ---------------------------------------------------------------- insertion

func (e *Editor) insert(s string, typing bool) {
	if e.deleteSelection() {
		typing = false
	}
	r2, c2 := e.row, e.col
	if e.overwrite && typing && !strings.Contains(s, "\n") && c2 < len(e.lines[e.row]) {
		c2 += len([]rune(s))
		if c2 > len(e.lines[e.row]) {
			c2 = len(e.lines[e.row])
		}
	}
	e.change(e.row, e.col, r2, c2, s, typing)
}

func (e *Editor) newline() {
	e.deleteSelection()
	// Keep the indentation of the current line.
	line := e.lines[e.row]
	indent := 0
	for indent < len(line) && indent < e.col && (line[indent] == ' ' || line[indent] == '\t') {
		indent++
	}
	e.change(e.row, e.col, e.row, e.col, "\n"+string(line[:indent]), false)
}

func (e *Editor) backspace() {
	if e.deleteSelection() {
		return
	}
	switch {
	case e.col > 0:
		e.change(e.row, e.col-1, e.row, e.col, "", true)
	case e.row > 0:
		e.change(e.row-1, len(e.lines[e.row-1]), e.row, 0, "", false)
	}
}

func (e *Editor) deleteForward() {
	if e.deleteSelection() {
		return
	}
	switch {
	case e.col < len(e.lines[e.row]):
		e.change(e.row, e.col, e.row, e.col+1, "", true)
	case e.row < len(e.lines)-1:
		e.change(e.row, e.col, e.row+1, 0, "", false)
	}
}

func (e *Editor) deleteLine() {
	e.selected = false
	if len(e.lines) == 1 {
		e.change(0, 0, 0, len(e.lines[0]), "", false)
		return
	}
	col := e.col
	if e.row < len(e.lines)-1 {
		e.change(e.row, 0, e.row+1, 0, "", false)
	} else {
		e.change(e.row-1, len(e.lines[e.row-1]), e.row, len(e.lines[e.row]), "", false)
		e.row = len(e.lines) - 1
	}
	e.col = clampInt(col, 0, len(e.lines[e.row]))
}

func (e *Editor) wordLeft() int {
	line := e.lines[e.row]
	i := e.col
	for i > 0 && !isWordRune(line[i-1]) {
		i--
	}
	for i > 0 && isWordRune(line[i-1]) {
		i--
	}
	return i
}

func (e *Editor) wordRight() int {
	line := e.lines[e.row]
	i := e.col
	for i < len(line) && isWordRune(line[i]) {
		i++
	}
	for i < len(line) && !isWordRune(line[i]) {
		i++
	}
	return i
}

// ---------------------------------------------------------------- search

// find moves the cursor to the next occurrence of e.search starting after
// the cursor (or before it when backwards), wrapping around.
func (e *Editor) find(a *App, backwards bool) bool {
	if e.search == "" {
		return false
	}
	q := strings.ToLower(e.search)
	n := len(e.lines)
	for step := 0; step <= n; step++ {
		var r int
		if backwards {
			r = ((e.row-step)%n + n) % n
		} else {
			r = (e.row + step) % n
		}
		line := strings.ToLower(string(e.lines[r]))
		var idx int
		switch {
		case step == 0 && !backwards:
			from := len(string(e.lines[r][:e.col])) + 1
			if from > len(line) {
				idx = -1
			} else {
				idx = strings.Index(line[from:], q)
				if idx >= 0 {
					idx += from
				}
			}
		case step == 0 && backwards:
			upto := len(string(e.lines[r][:e.col]))
			if upto > len(line) {
				upto = len(line)
			}
			idx = strings.LastIndex(line[:upto], q)
		case backwards:
			idx = strings.LastIndex(line, q)
		default:
			idx = strings.Index(line, q)
		}
		if idx >= 0 {
			e.row = r
			e.col = len([]rune(line[:idx]))
			e.anchor = [2]int{e.row, e.col}
			e.col += len([]rune(q))
			e.selected = true
			e.wantCol = e.col
			return true
		}
	}
	a.setStatus("Not found: " + e.search)
	return false
}

func (e *Editor) askFind(a *App, backwards bool) {
	a.input("Search", "Find:", e.search, func(s string) {
		if s == "" {
			return
		}
		e.search = s
		e.find(a, backwards)
	})
}

func (e *Editor) askReplace(a *App) {
	a.input("Replace", "Find:", e.search, func(s string) {
		if s == "" {
			return
		}
		e.search = s
		a.input("Replace", "Replace with:", e.replace, func(r string) {
			e.replace = r
			if !e.find(a, false) {
				return
			}
			e.replaceLoop(a, false)
		})
	})
}

// replaceLoop asks for each match (or replaces all) until no match is left.
func (e *Editor) replaceLoop(a *App, all bool) {
	count := 0
	doReplace := func() {
		r1, c1, r2, c2, ok := e.selRange()
		if ok {
			e.change(r1, c1, r2, c2, e.replace, false)
			count++
		}
	}
	if all {
		// Matches are visited in file order; a match before the previous
		// one means the search wrapped around, so everything is done.
		lastRow, lastCol := -1, -1
		for {
			r1, c1, _, _, ok := e.selRange()
			if !ok || r1 < lastRow || (r1 == lastRow && c1 < lastCol) || count > 1000000 {
				break
			}
			lastRow, lastCol = r1, c1
			doReplace()
			lastCol += len([]rune(e.replace)) - len([]rune(e.search))
			if !e.find(a, false) {
				break
			}
		}
		e.selected = false
		a.setStatus(fmt.Sprintf("Replaced %d occurrences", count))
		return
	}
	a.confirm("Replace?", []string{fitLeft(string(e.lines[e.row]), a.w-10)}, []string{"Replace", "All", "Skip", "Cancel"}, func(r int) {
		switch r {
		case 0:
			doReplace()
			if e.find(a, false) {
				e.replaceLoop(a, false)
			}
		case 1:
			e.replaceLoop(a, true)
		case 2:
			if e.find(a, false) {
				e.replaceLoop(a, false)
			}
		}
	})
}

// ---------------------------------------------------------------- keys

func (e *Editor) rows(a *App) int { return maxInt(1, a.h-2) }

func (e *Editor) HandleKey(a *App, ev *tcell.EventKey) {
	rows := e.rows(a)
	name := keyName(ev)
	shift := strings.Contains(name, "Shift+")
	base := strings.ReplaceAll(name, "Shift+", "")
	switch base {
	case "Up":
		e.moveWith(shift, func() {
			if e.row > 0 {
				e.row--
				e.col = clampInt(e.wantCol, 0, len(e.lines[e.row]))
			}
		})
		return
	case "Down":
		e.moveWith(shift, func() {
			if e.row < len(e.lines)-1 {
				e.row++
				e.col = clampInt(e.wantCol, 0, len(e.lines[e.row]))
			}
		})
		return
	case "Left":
		e.moveWith(shift, func() {
			if e.col > 0 {
				e.col--
			} else if e.row > 0 {
				e.row--
				e.col = len(e.lines[e.row])
			}
			e.wantCol = e.col
		})
		return
	case "Right":
		e.moveWith(shift, func() {
			if e.col < len(e.lines[e.row]) {
				e.col++
			} else if e.row < len(e.lines)-1 {
				e.row++
				e.col = 0
			}
			e.wantCol = e.col
		})
		return
	case "Ctrl+Left":
		e.moveWith(shift, func() { e.col = e.wordLeft(); e.wantCol = e.col })
		return
	case "Ctrl+Right":
		e.moveWith(shift, func() { e.col = e.wordRight(); e.wantCol = e.col })
		return
	case "Home":
		e.moveWith(shift, func() { e.col = 0; e.wantCol = 0 })
		return
	case "End":
		e.moveWith(shift, func() { e.col = len(e.lines[e.row]); e.wantCol = e.col })
		return
	case "PgUp":
		e.moveWith(shift, func() {
			e.row = maxInt(0, e.row-rows)
			e.top = maxInt(0, e.top-rows)
			e.col = clampInt(e.wantCol, 0, len(e.lines[e.row]))
		})
		return
	case "PgDn":
		e.moveWith(shift, func() {
			e.row = clampInt(e.row+rows, 0, len(e.lines)-1)
			e.top = clampInt(e.top+rows, 0, maxInt(0, len(e.lines)-rows))
			e.col = clampInt(e.wantCol, 0, len(e.lines[e.row]))
		})
		return
	case "Ctrl+Home":
		e.moveWith(shift, func() { e.row, e.col, e.wantCol = 0, 0, 0 })
		return
	case "Ctrl+End":
		e.moveWith(shift, func() { e.row = len(e.lines) - 1; e.col = len(e.lines[e.row]); e.wantCol = e.col })
		return
	case "Ctrl+N":
		e.moveWith(shift, func() { e.row = e.top; e.col = clampInt(e.wantCol, 0, len(e.lines[e.row])) })
		return
	case "Ctrl+E":
		e.moveWith(shift, func() {
			e.row = clampInt(e.top+rows-1, 0, len(e.lines)-1)
			e.col = clampInt(e.wantCol, 0, len(e.lines[e.row]))
		})
		return
	case "Ctrl+Up":
		if e.top > 0 {
			e.top--
			if e.row >= e.top+rows {
				e.row--
			}
		}
		return
	case "Ctrl+Down":
		if e.top < len(e.lines)-1 {
			e.top++
			if e.row < e.top {
				e.row++
			}
		}
		e.clampCursor()
		return
	}
	switch name {
	case "Esc", "F10":
		e.close(a)
	case "F2":
		e.save(a, e.path)
	case "Shift+F2":
		a.input("Save as", "File name:", e.path, func(p string) {
			if p = strings.TrimSpace(p); p != "" {
				e.save(a, absPath(p))
			}
		})
	case "F7":
		e.askFind(a, false)
	case "Shift+F7":
		if e.search == "" {
			e.askFind(a, false)
		} else {
			e.find(a, false)
		}
	case "Alt+F7":
		if e.search == "" {
			e.askFind(a, true)
		} else {
			e.find(a, true)
		}
	case "Ctrl+F7":
		e.askReplace(a)
	case "Alt+F8", "Ctrl+G":
		a.input("Go to line", fmt.Sprintf("Line (1-%d):", len(e.lines)), "", func(s string) {
			var n int
			if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n); err == nil && n >= 1 {
				e.selected = false
				e.row = clampInt(n-1, 0, len(e.lines)-1)
				e.col, e.wantCol = 0, 0
			}
		})
	case "F1":
		a.push(newViewer("Editor keys", editorHelp, false))
	case "Ctrl+Z", "Alt+Backspace":
		if !e.undoOne() {
			a.setStatus("Nothing to undo")
		}
	case "Ctrl+Shift+Z":
		if !e.redoOne() {
			a.setStatus("Nothing to redo")
		}
	case "Ctrl+A":
		e.anchor = [2]int{0, 0}
		e.row = len(e.lines) - 1
		e.col = len(e.lines[e.row])
		e.selected = true
	case "Ctrl+U":
		e.selected = false
	case "Ctrl+C", "Ctrl+Insert":
		if s := e.selectedText(); s != "" {
			a.setClipboard(s)
			a.setStatus("Copied")
		}
	case "Ctrl+X", "Shift+Delete":
		if s := e.selectedText(); s != "" {
			a.setClipboard(s)
			e.deleteSelection()
		}
	case "Ctrl+V", "Shift+Insert":
		if a.clipboard != "" {
			e.insert(a.clipboard, false)
		} else {
			a.screen.GetClipboard()
		}
	case "Ctrl+D":
		e.deleteSelection()
	case "Enter":
		e.newline()
	case "Backspace":
		e.backspace()
	case "Ctrl+Backspace":
		if !e.deleteSelection() && e.col > 0 {
			e.change(e.row, e.wordLeft(), e.row, e.col, "", false)
		}
	case "Delete":
		e.deleteForward()
	case "Ctrl+Y":
		e.deleteLine()
	case "Ctrl+K":
		if !e.deleteSelection() {
			e.change(e.row, e.col, e.row, len(e.lines[e.row]), "", false)
		}
	case "Insert":
		e.overwrite = !e.overwrite
	case "Tab":
		e.insert("\t", true)
	default:
		if ev.Key() == tcell.KeyRune && ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt|tcell.ModMeta) == 0 {
			e.insert(string(ev.Rune()), true)
		}
	}
}

// pasteText inserts clipboard contents delivered asynchronously.
func (e *Editor) pasteText(s string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	e.insert(s, false)
}

func (e *Editor) close(a *App) {
	finish := func(saved bool) {
		a.pop()
		if e.OnClose != nil {
			e.OnClose(saved || e.saved)
		}
	}
	if !e.modified {
		finish(false)
		return
	}
	a.confirm("File was changed", []string{fitLeft(e.path, a.w-10)}, []string{"Save", "Don't save", "Cancel"}, func(r int) {
		switch r {
		case 0:
			if e.save(a, e.path) {
				finish(true)
			}
		case 1:
			finish(false)
		}
	})
}

// ---------------------------------------------------------------- drawing

// visualCol returns the display column of rune index col in line.
func (e *Editor) visualCol(line []rune, col int) int {
	x := 0
	for i := 0; i < col && i < len(line); i++ {
		if line[i] == '\t' {
			x += e.tabSize - x%e.tabSize
		} else {
			x += runewidth.RuneWidth(line[i])
		}
	}
	return x
}

func (e *Editor) Draw(a *App) {
	th := a.theme
	rows := e.rows(a)
	a.fill(0, 0, a.w, a.h, th.Viewer)
	// Keep the cursor visible.
	if e.row < e.top {
		e.top = e.row
	}
	if e.row >= e.top+rows {
		e.top = e.row - rows + 1
	}
	cx := e.visualCol(e.lines[e.row], e.col)
	if cx < e.left {
		e.left = cx
	}
	if cx >= e.left+a.w {
		e.left = cx - a.w + 1
	}

	// Title bar.
	mod := ""
	if e.modified {
		mod = " *"
	}
	eol := "LF"
	if e.crlf {
		eol = "CRLF"
	}
	mode := "Ins"
	if e.overwrite {
		mode = "Ovr"
	}
	right := fmt.Sprintf(" Line %d/%d  Col %d  %s  %s ", e.row+1, len(e.lines), cx+1, mode, eol)
	a.fill(0, 0, a.w, 1, th.KeyLabel)
	a.print(1, 0, a.w-2-runewidth.StringWidth(right), th.KeyLabel, fitRight(e.path+mod, a.w-2-runewidth.StringWidth(right)))
	a.print(a.w-runewidth.StringWidth(right), 0, runewidth.StringWidth(right), th.KeyLabel, right)

	r1, c1, r2, c2, hasSel := e.selRange()
	selSt := th.Cursor
	for i := 0; i < rows; i++ {
		idx := e.top + i
		if idx >= len(e.lines) {
			break
		}
		y := 1 + i
		line := e.lines[idx]
		x := 0
		for j, ch := range line {
			w := runewidth.RuneWidth(ch)
			text := string(ch)
			if ch == '\t' {
				w = e.tabSize - x%e.tabSize
				text = strings.Repeat(" ", w)
			}
			st := th.Viewer
			if hasSel && (idx > r1 || (idx == r1 && j >= c1)) && (idx < r2 || (idx == r2 && j < c2)) {
				st = selSt
			}
			if x+w > e.left && x-e.left < a.w {
				a.print(maxInt(0, x-e.left), y, w, st, text)
			}
			x += w
		}
		// A selection that spans the line end is shown one cell past it.
		if hasSel && idx >= r1 && idx < r2 && x-e.left < a.w {
			a.print(maxInt(0, x-e.left), y, 1, selSt, " ")
		}
	}
	a.screen.ShowCursor(cx-e.left, 1+e.row-e.top)

	foot := " F2 Save  Shift+F2 Save as  F7 Find  Shift+F7 Next  Ctrl+F7 Replace  Alt+F8 Go to  Ctrl+Z Undo  Ctrl+Y Del line  F1 Keys  Esc Exit "
	a.fill(0, a.h-1, a.w, 1, th.KeyLabel)
	a.print(0, a.h-1, a.w, th.KeyLabel, fitLeft(foot, a.w))
}

const editorHelp = `near editor keys (modelled on FAR's editor)

  F2                 Save             Shift+F2           Save as
  F10, Esc           Exit (asks to save)
  F7                 Find             Shift+F7 / Alt+F7  Find next / previous
  Ctrl+F7            Replace          Alt+F8, Ctrl+G     Go to line
  Ctrl+Z, Alt+Backspace  Undo         Ctrl+Shift+Z       Redo

  Arrows, Home, End, PgUp, PgDn       Move
  Ctrl+Left / Ctrl+Right              Word left / right
  Ctrl+Home / Ctrl+End                Start / end of file
  Ctrl+N / Ctrl+E                     Top / bottom of screen
  Ctrl+Up / Ctrl+Down                 Scroll by one line

  Shift+movement                      Select
  Ctrl+A                              Select all
  Ctrl+U                              Deselect
  Ctrl+C, Ctrl+Insert                 Copy
  Ctrl+X, Shift+Delete                Cut
  Ctrl+V, Shift+Insert                Paste
  Ctrl+D, Delete                      Delete selection

  Ctrl+Y                              Delete line
  Ctrl+K                              Delete to end of line
  Ctrl+Backspace                      Delete word left
  Insert                              Toggle insert / overwrite
  Tab                                 Insert a tab character
  Enter                               New line, keeping the indentation

Line endings (LF or CRLF) and a UTF-8 byte order mark are preserved on save.`
