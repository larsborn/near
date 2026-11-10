package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// SearchDialog is the Everything-like file name search over the index.
// Searches run in a goroutine so typing is never held up; each result
// carries the generation of the query it answers and stale ones are dropped.
type SearchDialog struct {
	edit    *LineEdit
	results []string
	total   int
	capped  bool
	cursor  int
	top     int
	last    string // query the shown results belong to
	indexN  int    // index size when the results were computed
	gen     int    // generation of the latest query
	busy    bool   // a search is running
	pending bool   // the query changed while a search was running
	note    string // what the last action did, shown in the footer
	box     rect   // where the dialog was drawn, for the mouse
	listTop int    // screen row of the first result
}

// EventSearchResult delivers results computed in the background.
type EventSearchResult struct {
	tcell.EventTime
	gen     int
	query   string
	results []string
	total   int
	capped  bool
	indexN  int
}

func (a *App) showSearch() {
	d := &SearchDialog{edit: newLineEdit("")}
	a.ensureIndex() // the index is only loaded once something wants it
	if a.index.Count() == 0 && !a.index.Building() && !a.index.Loading() {
		a.rebuildIndex()
	}
	a.push(d)
}

// update starts a search for the current text unless one is running, in
// which case it runs again as soon as that one finishes.
func (d *SearchDialog) update(a *App) {
	if d.busy {
		d.pending = true
		return
	}
	q := d.edit.String()
	if q == d.last && a.index.Count() == d.indexN {
		return
	}
	d.busy = true
	d.gen++
	gen, max, screen := d.gen, a.cfg.SearchMaxResults, a.screen
	n := a.index.Count()
	ix := a.index
	go func() {
		out, total, capped := ix.Search(q, max)
		ev := &EventSearchResult{gen: gen, query: q, results: out, total: total, capped: capped, indexN: n}
		ev.SetEventNow()
		_ = screen.PostEvent(ev)
	}()
}

// apply takes a background result; only the latest generation counts.
func (d *SearchDialog) apply(a *App, ev *EventSearchResult) {
	d.busy = false
	if ev.gen == d.gen {
		d.results, d.total, d.capped = ev.results, ev.total, ev.capped
		d.last, d.indexN = ev.query, ev.indexN
		d.cursor, d.top = 0, 0
	}
	if d.pending || ev.gen != d.gen || d.edit.String() != d.last {
		d.pending = false
		d.update(a)
	}
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
	name := keyName(ev)
	d.note = ""
	// The panel keys for copying a path and for the context menu work on a
	// result too, whatever they are bound to.
	if ok, dirOnly := a.copyPathKey(name); ok {
		d.copyResult(a, dirOnly)
		return
	}
	if a.keymap[name] == "context_menu" {
		d.showMenu(a)
		return
	}
	switch name {
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
			d.update(a)
		}
	}
}

// current returns the selected result, or "".
func (d *SearchDialog) current() string {
	if d.cursor < 0 || d.cursor >= len(d.results) {
		return ""
	}
	return d.results[d.cursor]
}

// copyResult copies the selected result's path, or only the directory it
// is in, to the clipboard.
func (d *SearchDialog) copyResult(a *App, dirOnly bool) {
	p := d.current()
	if p == "" {
		return
	}
	what := "path"
	if dirOnly {
		p, what = filepath.Dir(p), "directory"
	}
	a.setClipboard(p)
	d.note = "Copied the " + what + ": " + p
}

// showMenu offers what can be done with the selected result, with the keys
// that do the same thing.
func (d *SearchDialog) showMenu(a *App) {
	if d.current() == "" {
		return
	}
	copyKey := a.keyFor("copy_path")
	items := []ListItem{
		{Label: "Go to the file", Hotkey: 'G', Right: "Enter", Data: "goto"},
		{Label: "Open in a new tab", Hotkey: 'O', Right: "Ctrl+T", Data: "tab"},
		{Sep: true},
		{Label: "Copy the path", Hotkey: 'C', Right: copyKey, Data: "copy"},
		{Label: "Copy the directory", Hotkey: 'D', Right: shiftKey(copyKey), Data: "copydir"},
		{Sep: true},
		{Label: "Rebuild the index", Hotkey: 'R', Right: "Ctrl+R", Data: "reindex"},
	}
	m := a.list(filepath.Base(d.current()), items, func(_ int, li ListItem) {
		switch li.Data.(string) {
		case "goto":
			d.open(a, false)
		case "tab":
			d.open(a, true)
		case "copy":
			d.copyResult(a, false)
		case "copydir":
			d.copyResult(a, true)
		case "reindex":
			a.rebuildIndex()
		}
	})
	m.Menu = true
}

// HandleMouse lets the mouse pick a result and open the menu on it.
func (d *SearchDialog) HandleMouse(a *App, ev *tcell.EventMouse) {
	x, y := ev.Position()
	b := ev.Buttons()
	switch {
	case b&tcell.WheelUp != 0:
		d.cursor = clampInt(d.cursor-1, 0, maxInt(0, len(d.results)-1))
		return
	case b&tcell.WheelDown != 0:
		d.cursor = clampInt(d.cursor+1, 0, maxInt(0, len(d.results)-1))
		return
	}
	if x < d.box.x || x >= d.box.x+d.box.w || y < d.listTop || y >= d.listTop+d.rows(a) {
		return
	}
	if idx := d.top + (y - d.listTop); idx < len(d.results) {
		d.cursor = idx
		d.note = ""
		if b&tcell.ButtonSecondary != 0 {
			d.showMenu(a)
		}
	}
}

func (d *SearchDialog) Draw(a *App) {
	th := a.theme
	// Results may have changed because the index finished building.
	if d.last != "" && a.index.Count() != d.indexN {
		d.update(a)
	}
	rows := d.rows(a)
	w := clampInt(a.w-4, 40, a.w)
	h := rows + 5
	x, y := a.center(w, h)
	d.box, d.listTop = rect{x, y, w, h}, y+3
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
	if len(d.results) == 0 {
		// Say what is going on instead of leaving the list blank.
		for i, line := range d.stateLines(a) {
			if i < rows {
				a.print(x+2, y+3+i, iw, th.ListDim, fitLeft(line, iw))
			}
		}
	}
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
		status = fmt.Sprintf("indexing %s… %s names so far", rootsLabel(a.index.Roots()), groupDigits(a.index.progress.Load()))
	case a.index.Count() == 0 && a.index.Err() != nil:
		status = "indexing failed: " + a.index.Err().Error()
	case a.index.Count() == 0:
		status = "index is empty, Ctrl+R rebuilds it"
	case d.last == "" && d.busy:
		status = "searching…"
	case d.last == "":
		status = fmt.Sprintf("%s names, built %s, %s", groupDigits(int64(a.index.Count())), a.index.BuiltAt().Format("15:04"), a.index.WatchStatus())
	case d.capped:
		status = fmt.Sprintf("%s+ matches, showing first %d", groupDigits(int64(d.total)), len(d.results))
	case d.total > len(d.results):
		status = fmt.Sprintf("%s matches, showing first %d", groupDigits(int64(d.total)), len(d.results))
	default:
		status = fmt.Sprintf("%d matches", d.total)
	}
	foot := fmt.Sprintf("Enter: go to   Ctrl+T: new tab   Ctrl+R: rebuild   %s: menu   Esc: close", a.keyFor("context_menu"))
	if d.note != "" {
		foot = d.note
	}
	status = fitLeft(status, w-4)
	sw := runewidth.StringWidth(status)
	if fw := w - 6 - sw; fw > 0 {
		a.print(x+2, y+h-1, fw, th.DialogHint, fitLeft(foot, fw))
	}
	a.print(x+w-2-sw, y+h-1, sw, th.DialogHint, status)
	_ = strings.TrimSpace
}

// stateLines describes the dialog's state while there are no results to
// show: the index being built, failed or empty, a search running, or a
// query without matches.
func (d *SearchDialog) stateLines(a *App) []string {
	ix := a.index
	fix := []string{"", "Check search_roots in", "  " + a.cfg.path, "and press Ctrl+R to rebuild the index."}
	switch {
	case ix.Loading():
		return []string{"Loading the index of " + rootsLabel(ix.Roots()) + "…", "", "Results appear in a moment."}
	case ix.Building():
		lines := []string{"Building the index of " + rootsLabel(ix.Roots()) + "…", groupDigits(ix.progress.Load()) + " names so far"}
		if dir := ix.Walking(); dir != "" {
			lines = append(lines, "Reading "+dir)
		}
		if ix.Count() == 0 {
			lines = append(lines, "", "Results appear when the build is done.")
		}
		return lines
	case ix.Count() == 0 && ix.Err() != nil:
		return append([]string{"Indexing failed:", "  " + ix.Err().Error()}, fix...)
	case ix.Count() == 0:
		return append([]string{"The index is empty: nothing was found below", "  " + rootsLabel(ix.Roots())}, fix...)
	case d.busy:
		return []string{"Searching…"}
	case d.edit.String() == "":
		lines := []string{fmt.Sprintf("Type to search %s names in %s.", groupDigits(int64(ix.Count())), rootsLabel(ix.Roots()))}
		if ix.CatchingUp() {
			lines = append(lines, "Catching up with the change journal…")
		} else if note := ix.CatchUpNote(); note != "" {
			lines = append(lines, "The index "+note+".")
		}
		return lines
	case d.last == d.edit.String():
		return []string{fmt.Sprintf("No matches for %q.", d.last)}
	}
	return nil
}

// rootsLabel lists the index roots for messages.
func rootsLabel(roots []string) string {
	if len(roots) == 0 {
		return "no search roots"
	}
	return strings.Join(roots, ", ")
}
