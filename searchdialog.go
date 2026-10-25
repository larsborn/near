package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// SearchDialog is the Everything-like file name search over the index.
type SearchDialog struct {
	edit    *LineEdit
	results []string
	total   int
	capped  bool
	cursor  int
	top     int
	last    string
	dirty   bool // query changed, search pending
	indexN  int  // index size when the results were computed
}

func (a *App) showSearch() {
	d := &SearchDialog{edit: newLineEdit("")}
	if a.index.Count() == 0 && !a.index.Building() {
		a.rebuildIndex()
	}
	a.push(d)
}

func (d *SearchDialog) update(a *App) {
	q := d.edit.String()
	if q == d.last {
		return
	}
	d.last = q
	d.indexN = a.index.Count()
	d.results, d.total, d.capped = a.index.Search(q, a.cfg.SearchMaxResults)
	d.cursor, d.top = 0, 0
}

func (d *SearchDialog) rows(a *App) int { return maxInt(1, a.h-8) }

func (d *SearchDialog) open(a *App, newTab bool) {
	if d.cursor >= len(d.results) {
		return
	}
	path := d.results[d.cursor]
	a.pop()
	t := a.tab()
	if newTab {
		a.doAction("new_tab")
		t = a.tab()
	}
	dir, name := filepath.Split(path)
	if isDir(path) {
		dir, name = path, ""
	}
	if err := a.navigate(t, filepath.Clean(dir), name); err != nil {
		a.setErr(err)
	}
}

func (d *SearchDialog) HandleKey(a *App, ev *tcell.EventKey) {
	rows := d.rows(a)
	switch keyName(ev) {
	case "Esc":
		a.pop()
	case "Enter":
		d.open(a, false)
	case "Ctrl+T", "Shift+Enter", "Ctrl+Enter":
		d.open(a, true)
	case "Ctrl+R", "F5":
		a.rebuildIndex()
	case "Up":
		d.cursor = clampInt(d.cursor-1, 0, maxInt(0, len(d.results)-1))
	case "Down":
		d.cursor = clampInt(d.cursor+1, 0, maxInt(0, len(d.results)-1))
	case "PgUp":
		d.cursor = clampInt(d.cursor-rows, 0, maxInt(0, len(d.results)-1))
	case "PgDn":
		d.cursor = clampInt(d.cursor+rows, 0, maxInt(0, len(d.results)-1))
	case "Home":
		d.cursor = 0
	case "End":
		d.cursor = maxInt(0, len(d.results)-1)
	default:
		if d.edit.HandleKey(ev, a) {
			d.dirty = true
		}
	}
}

func (d *SearchDialog) Draw(a *App) {
	th := a.theme
	// Searching a large index takes a noticeable time, so a pending search
	// waits until no further key presses are queued: typing fast only
	// searches once, for the final text.
	if d.dirty && !a.screen.HasPendingEvent() {
		d.dirty = false
		d.update(a)
	}
	// Results may have changed because the index finished building.
	if d.last != "" && a.index.Count() != d.indexN {
		d.last = ""
		d.update(a)
	}
	rows := d.rows(a)
	w := clampInt(a.w-4, 40, a.w)
	h := rows + 5
	x, y := a.center(w, h)
	a.drawBox(x, y, w, h, th.Dialog, "Search file names")
	a.print(x+2, y+1, 8, th.Dialog, "Find:")
	d.edit.Draw(a, x+8, y+1, w-10, th.InputFocus, true)
	a.hline(x+1, y+2, w-2, th.Dialog, '─')
	a.screen.SetContent(x, y+2, '╟', nil, th.Dialog)
	a.screen.SetContent(x+w-1, y+2, '╢', nil, th.Dialog)
	if d.cursor < d.top {
		d.top = d.cursor
	}
	if d.cursor >= d.top+rows {
		d.top = d.cursor - rows + 1
	}
	iw := w - 4
	for i := 0; i < rows; i++ {
		idx := d.top + i
		if idx >= len(d.results) {
			break
		}
		ry := y + 3 + i
		p := d.results[idx]
		dir, name := filepath.Split(p)
		st, dst := th.Dialog, th.ListDim
		if idx == d.cursor {
			st, dst = th.ListCursor, th.ListCursor
			a.fill(x+1, ry, w-2, 1, st)
		}
		nw := runewidth.StringWidth(name)
		dw := iw - nw - 2
		if dw < 10 {
			dw = maxInt(0, iw-nw)
		}
		dir = fitRight(dir, dw)
		cx := x + 2
		cx += a.print(cx, ry, dw, dst, dir)
		a.print(cx, ry, iw-(cx-x-2), st.Bold(idx == d.cursor), name)
	}
	a.hline(x+1, y+h-2, w-2, th.Dialog, '─')
	a.screen.SetContent(x, y+h-2, '╟', nil, th.Dialog)
	a.screen.SetContent(x+w-1, y+h-2, '╢', nil, th.Dialog)
	var status string
	switch {
	case a.index.Building():
		status = fmt.Sprintf("indexing… %s names so far", groupDigits(a.index.progress.Load()))
	case a.index.Count() == 0:
		status = "index is empty, check search_roots in " + a.cfg.path
	case d.last == "":
		status = fmt.Sprintf("%s names, built %s, %s", groupDigits(int64(a.index.Count())), a.index.BuiltAt().Format("15:04"), a.index.WatchStatus())
	case d.capped:
		status = fmt.Sprintf("%s+ matches, showing first %d", groupDigits(int64(d.total)), len(d.results))
	case d.total > len(d.results):
		status = fmt.Sprintf("%s matches, showing first %d", groupDigits(int64(d.total)), len(d.results))
	default:
		status = fmt.Sprintf("%d matches", d.total)
	}
	foot := "Enter: go to   Ctrl+T: open in new tab   Ctrl+R: rebuild index   Esc: close"
	sw := runewidth.StringWidth(status)
	a.print(x+2, y+h-1, w-6-sw, th.DialogHint, fitLeft(foot, w-6-sw))
	a.print(x+w-2-sw, y+h-1, sw, th.DialogHint, status)
	_ = strings.TrimSpace
}
