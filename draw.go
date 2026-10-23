package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

const panelChrome = 7 // top border, path, header, separator, info, note, bottom border

// bottomRows are the lines below the panels: status, command line, key bar.
const bottomRows = 3

// listRows is the number of entry rows panel i can show.
func (a *App) listRows(i int) int {
	rows := a.panelHeight(i) - panelChrome
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (a *App) draw() {
	a.w, a.h = a.screen.Size()
	a.screen.Clear()
	a.screen.HideCursor()
	ph := a.h - bottomRows
	pw := a.panelSplit()
	// The panel area is blank where no panel is drawn: hidden panels, and
	// the rows below a panel made shorter with Ctrl+Down.
	a.fill(0, 0, a.w, ph, a.theme.Cmd)
	if a.hideAll {
		if a.status == "" {
			a.setStatus("Panels hidden, Ctrl+O shows them again")
		}
	} else {
		if !a.hidden[0] {
			a.drawPanel(0, 0, 0, pw, a.panelHeight(0))
		}
		if !a.hidden[1] {
			a.drawPanel(1, pw, 0, a.w-pw, a.panelHeight(1))
		}
	}
	a.drawStatus()
	a.drawCmdLine()
	a.drawKeyBar()
	for _, d := range a.dialogs {
		d.Draw(a)
	}
	a.screen.Show()
}

func (a *App) drawPanel(idx, x, y, w, h int) {
	th := a.theme
	p := a.panels[idx]
	t := p.Tab()
	active := idx == a.active && len(a.dialogs) == 0
	a.fill(x, y, w, h, th.Panel)

	// frame
	bst := th.Border
	if idx == a.active {
		bst = th.BorderActive
	}
	for i := 1; i < w-1; i++ {
		a.screen.SetContent(x+i, y, '═', nil, bst)
		a.screen.SetContent(x+i, y+h-1, '═', nil, bst)
		a.screen.SetContent(x+i, y+h-4, '─', nil, bst)
	}
	for i := 1; i < h-1; i++ {
		a.screen.SetContent(x, y+i, '║', nil, bst)
		a.screen.SetContent(x+w-1, y+i, '║', nil, bst)
	}
	a.screen.SetContent(x, y, '╔', nil, bst)
	a.screen.SetContent(x+w-1, y, '╗', nil, bst)
	a.screen.SetContent(x, y+h-1, '╚', nil, bst)
	a.screen.SetContent(x+w-1, y+h-1, '╝', nil, bst)
	a.screen.SetContent(x, y+h-4, '╟', nil, bst)
	a.screen.SetContent(x+w-1, y+h-4, '╢', nil, bst)

	// tabs on the top border
	cx := x + 2
	for i, tab := range p.Tabs {
		base := filepath.Base(tab.Path)
		if base == "" || base == string(filepath.Separator) || strings.HasSuffix(tab.Path, ":\\") {
			base = tab.Path
		}
		label := fmt.Sprintf(" %d:%s ", i+1, fitLeft(base, 12))
		st := th.Tab
		if i == p.Active {
			st = th.TabActive
			if idx != a.active {
				st = th.Tab.Bold(true).Underline(true)
			}
		}
		lw := runewidth.StringWidth(label)
		if cx+lw > x+w-2 {
			a.print(cx, y, x+w-2-cx, th.TabBar, "…")
			break
		}
		a.print(cx, y, lw, st, label)
		cx += lw + 1
	}

	inner := w - 2
	ix := x + 1

	// path row, with the hidden-files flag at the right
	pst := th.Path
	if !active {
		pst = th.Info
	}
	flag := ""
	if a.showHidden {
		flag = " hidden "
	}
	a.print(ix, y+1, inner-len(flag), pst, fitRight(t.Path, inner-len(flag)))
	a.print(ix+inner-len(flag), y+1, len(flag), th.Info, flag)

	// columns: Name │ Size │ Date │ Time, FAR style
	type column struct {
		title string
		w     int
	}
	sizeW := 7
	dateW := runewidth.StringWidth(time.Now().Format(a.cfg.DateFormat))
	timeW := runewidth.StringWidth(time.Now().Format(a.cfg.TimeFormat))
	if inner < 50 {
		timeW = 0
	}
	if inner < 40 {
		dateW = 0
	}
	if inner < 28 {
		sizeW = 0
	}
	cols := []column{{"Name", 0}}
	if sizeW > 0 {
		cols = append(cols, column{"Size", sizeW})
	}
	if dateW > 0 {
		cols = append(cols, column{"Date", dateW})
	}
	if timeW > 0 {
		cols = append(cols, column{"Time", timeW})
	}
	nameW := inner - (len(cols) - 1)
	for _, c := range cols[1:] {
		nameW -= c.w
	}
	if nameW < 4 {
		nameW = 4
	}
	cols[0].w = nameW

	// vertical separators run from the header down to the bottom separator
	sepX := []int{}
	for cxx, i := ix, 0; i < len(cols)-1; i++ {
		cxx += cols[i].w
		sepX = append(sepX, cxx)
		cxx++
	}
	for _, sx := range sepX {
		for yy := y + 2; yy < y+h-4; yy++ {
			a.screen.SetContent(sx, yy, '│', nil, bst)
		}
		a.screen.SetContent(sx, y+h-4, '┴', nil, bst)
	}

	// header: sort indicator at the left of Name, titles centred
	sortTag := []string{"n", "x", "s", "m"}[t.Sort]
	if t.SortRev {
		sortTag += "↓"
	}
	hx := ix
	for i, c := range cols {
		title := c.title
		pad := (c.w - runewidth.StringWidth(title)) / 2
		if pad < 0 {
			pad = 0
		}
		a.print(hx+pad, y+2, c.w-pad, th.Header, title)
		if i == 0 {
			a.print(hx, y+2, 3, th.Header, sortTag)
		}
		hx += c.w + 1
	}

	// entries
	rows := a.listRows(idx)
	t.EnsureVisible(rows)
	if t.Err != nil {
		a.print(ix+1, y+3, inner-2, th.Panel.Foreground(tcell.ColorRed).Bold(true), fitLeft(t.Err.Error(), inner-2))
	}
	for i := 0; i < rows; i++ {
		ei := t.Top + i
		if ei >= len(t.Entries) {
			break
		}
		e := &t.Entries[ei]
		ry := y + 3 + i
		st := th.File
		switch {
		case e.IsUp:
			st = th.Dir
		case e.IsLink:
			st = th.Link
		case e.IsDir:
			st = th.Dir
		case e.Hidden:
			st = th.Hidden
		}
		sel := t.Selected[e.Name]
		if sel {
			st = th.Selected
		}
		isCursor := active && ei == t.Cursor
		if isCursor {
			st = th.Cursor
			if sel {
				st = th.CursorSel
			}
			a.fill(ix, ry, inner, 1, st)
		}
		// name cell; a note is flagged by * in the last cell of the column
		name := e.Name
		_, noted := t.Notes[e.Name]
		if noted {
			name = padRight(fitLeft(name, nameW-2), nameW-1) + "*"
		}
		a.print(ix, ry, nameW, st, fitLeft(name, nameW))
		cxx := ix + nameW + 1
		for _, c := range cols[1:] {
			var cell string
			switch c.title {
			case "Size":
				switch {
				case e.IsUp:
					cell = "Up"
				case e.IsDir:
					cell = "Folder"
				default:
					cell = humanSize(e.Size)
				}
			case "Date":
				if !e.IsUp {
					cell = e.ModTime.Format(a.cfg.DateFormat)
				}
			case "Time":
				if !e.IsUp {
					cell = e.ModTime.Format(a.cfg.TimeFormat)
				}
			}
			a.print(cxx, ry, c.w, st, padLeft(cell, c.w))
			cxx += c.w + 1
		}
		if isCursor {
			for _, sx := range sepX {
				a.screen.SetContent(sx, ry, '│', nil, st)
			}
		}
	}

	// info line: details of the current entry
	infoY := y + h - 3
	if cur := t.Current(); cur != nil && !cur.IsUp {
		info := cur.Name
		if !cur.IsDir {
			info += fmt.Sprintf("  %s bytes", groupDigits(cur.Size))
		}
		info += "  " + cur.ModTime.Format(a.cfg.DateFormat+" "+a.cfg.TimeFormat)
		if cur.IsLink {
			info += "  (link)"
		}
		a.print(ix+1, infoY, inner-2, th.Info, fitLeft(info, inner-2))
	}

	// note line or quick search
	noteY := y + h - 2
	if active && a.qsActive {
		a.print(ix+1, noteY, inner-2, th.Note, fitLeft("Quick search: "+string(a.qsText), inner-2))
	} else if cur := t.Current(); cur != nil {
		if note, ok := t.Notes[cur.Name]; ok {
			first := strings.SplitN(note, "\n", 2)[0]
			if strings.Contains(note, "\n") {
				first += " …"
			}
			a.print(ix+1, noteY, inner-2, th.Note, fitLeft("✎ "+first, inner-2))
		}
	}

	// footer, centred on the bottom border like FAR
	var foot string
	if n, size := t.SelectedStats(); n > 0 {
		foot = fmt.Sprintf(" %s in %d selected ", humanSize(size), n)
	} else {
		files, folders := 0, 0
		var total int64
		for _, e := range t.Entries {
			switch {
			case e.IsUp:
			case e.IsDir:
				folders++
			default:
				files++
				total += e.Size
			}
		}
		foot = fmt.Sprintf(" Bytes: %s, files: %d, folders: %d ", humanSize(total), files, folders)
	}
	foot = fitLeft(foot, inner-2)
	a.print(x+(w-runewidth.StringWidth(foot))/2, y+h-1, runewidth.StringWidth(foot), bst, foot)
}

func (a *App) drawStatus() {
	th := a.theme
	y := a.h - 3
	a.fill(0, y, a.w, 1, th.Status)
	st := th.Status
	if a.statusErr {
		st = th.StatusErr
	}
	right := ""
	switch {
	case a.index.Building():
		right = fmt.Sprintf(" indexing… %s ", groupDigits(a.index.progress.Load()))
	case a.index.Count() > 0:
		right = fmt.Sprintf(" index: %s names ", groupDigits(int64(a.index.Count())))
	}
	rw := runewidth.StringWidth(right)
	a.print(1, y, a.w-2-rw, st, fitLeft(a.status, a.w-2-rw))
	a.print(a.w-rw, y, rw, th.Status, right)
}

func (a *App) drawKeyBar() {
	th := a.theme
	y := a.h - 1
	labels := []string{"Help", "Menu", "View", "Edit", "Copy", "Move", "MkDir", "Del", "Menus", "Quit"}
	a.fill(0, y, a.w, 1, th.KeyNum)
	cell := a.w / 10
	if cell < 4 {
		return
	}
	for i, l := range labels {
		x := i * cell
		num := fmt.Sprintf("%2d", i+1)
		a.print(x, y, 2, th.KeyNum, num)
		a.print(x+2, y, cell-3, th.KeyLabel, padRight(l, cell-3))
	}
}

func groupDigits(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
