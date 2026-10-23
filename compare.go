package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// showCompareMenu offers the compare operations.
func (a *App) showCompareMenu() {
	t, o := a.tab(), a.other()
	items := []ListItem{
		{Label: "Directories: mark entries that differ by size or date", Hotkey: 'd', Data: "dirs"},
		{Label: "Directories: mark entries that differ in content", Hotkey: 'c', Data: "content"},
		{Label: "Files: show the two current files side by side", Hotkey: 'f', Data: "files"},
	}
	d := a.list("Compare", items, func(_ int, li ListItem) {
		switch li.Data.(string) {
		case "dirs":
			a.compareDirs(false)
		case "content":
			a.compareDirs(true)
		case "files":
			a.compareFiles()
		}
	})
	d.Hint = fitLeft(fitRight(t.Path, 30)+"  ↔  "+fitRight(o.Path, 30), a.w-10)
}

// compareDirs marks, in both panels, files that are missing on the other
// side, differ in size, are newer than their counterpart, or (with content
// set) have different bytes. Directories are marked only when missing.
func (a *App) compareDirs(content bool) {
	t, o := a.tab(), a.other()
	if t.Path == o.Path {
		a.setStatus("Both panels show the same directory")
		return
	}
	byName := func(tab *Tab) map[string]*Entry {
		m := map[string]*Entry{}
		for i := range tab.Entries {
			if !tab.Entries[i].IsUp {
				m[tab.Entries[i].Name] = &tab.Entries[i]
			}
		}
		return m
	}
	tm, om := byName(t), byName(o)
	t.Selected, o.Selected = map[string]bool{}, map[string]bool{}
	marked := 0
	mark := func(tab *Tab, name string) {
		if !tab.Selected[name] {
			tab.Selected[name] = true
			marked++
		}
	}
	compared := 0
	for name, e := range tm {
		other, ok := om[name]
		switch {
		case !ok:
			mark(t, name)
		case e.IsDir || other.IsDir:
			// Present on both sides; directories are not compared further.
		case e.Size != other.Size:
			mark(t, name)
			mark(o, name)
		case !e.ModTime.Equal(other.ModTime):
			if e.ModTime.After(other.ModTime) {
				mark(t, name)
			} else {
				mark(o, name)
			}
		case content:
			compared++
			if !a.progress("Comparing contents", name) {
				a.setStatus("Compare cancelled")
				return
			}
			same, err := sameContent(filepath.Join(t.Path, name), filepath.Join(o.Path, name))
			if err != nil || !same {
				mark(t, name)
				mark(o, name)
			}
		}
	}
	for name := range om {
		if _, ok := tm[name]; !ok {
			mark(o, name)
		}
	}
	if marked == 0 {
		a.setStatus("No differences found")
		return
	}
	a.setStatus(fmt.Sprintf("%d differing entries marked", marked))
}

func sameContent(p1, p2 string) (bool, error) {
	f1, err := os.Open(p1)
	if err != nil {
		return false, err
	}
	defer f1.Close()
	f2, err := os.Open(p2)
	if err != nil {
		return false, err
	}
	defer f2.Close()
	b1, b2 := make([]byte, 64*1024), make([]byte, 64*1024)
	for {
		n1, err1 := readFull(f1, b1)
		n2, err2 := readFull(f2, b2)
		if n1 != n2 || !bytes.Equal(b1[:n1], b2[:n2]) {
			return false, nil
		}
		if err1 != nil || err2 != nil || n1 == 0 {
			return err1 == err2 || (n1 == 0 && n2 == 0), nil
		}
	}
}

func readFull(f *os.File, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		m, err := f.Read(buf[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// ---------------------------------------------------------------- file diff

func (a *App) compareFiles() {
	t, o := a.tab(), a.other()
	e1, e2 := t.Current(), o.Current()
	if e1 == nil || e2 == nil || e1.IsUp || e2.IsUp || e1.IsDir || e2.IsDir {
		a.setStatus("Put the cursor on a file in each panel")
		return
	}
	p1, p2 := filepath.Join(t.Path, e1.Name), filepath.Join(o.Path, e2.Name)
	if a.active == 1 {
		p1, p2 = p2, p1 // left panel on the left
	}
	v, err := newDiffViewer(p1, p2)
	if err != nil {
		a.setErr(err)
		return
	}
	a.push(v)
}

const diffMaxLines = 20000

type diffRow struct {
	kind        byte // '=', '-', '+', '~'
	left, right string
	lno, rno    int // 1-based line numbers, 0 = none
}

// DiffViewer shows two text files side by side with differences coloured.
type DiffViewer struct {
	left, right string
	rows        []diffRow
	top         int
	changes     int
	off         int // horizontal scroll
}

func newDiffViewer(p1, p2 string) (*DiffViewer, error) {
	read := func(p string) ([]string, error) {
		st, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if st.Size() > viewerMaxBytes {
			return nil, fmt.Errorf("%s is larger than %d MB", st.Name(), viewerMaxBytes>>20)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		if bytes.IndexByte(data, 0) >= 0 {
			return nil, fmt.Errorf("%s is a binary file", st.Name())
		}
		lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		if len(lines) > diffMaxLines {
			return nil, fmt.Errorf("%s has more than %d lines", st.Name(), diffMaxLines)
		}
		return lines, nil
	}
	l, err := read(p1)
	if err != nil {
		return nil, err
	}
	r, err := read(p2)
	if err != nil {
		return nil, err
	}
	v := &DiffViewer{left: p1, right: p2}
	v.rows = diffRows(l, r)
	for _, row := range v.rows {
		if row.kind != '=' {
			v.changes++
		}
	}
	return v, nil
}

// diffRows aligns two line lists using Myers' algorithm. Runs of removed
// and added lines are paired up into "changed" rows.
func diffRows(a, b []string) []diffRow {
	ops := myersDiff(a, b)
	var rows []diffRow
	i := 0
	for i < len(ops) {
		op := ops[i]
		if op.kind == '=' {
			rows = append(rows, diffRow{'=', a[op.a], b[op.b], op.a + 1, op.b + 1})
			i++
			continue
		}
		var dels, adds []diffOp
		for i < len(ops) && ops[i].kind != '=' {
			if ops[i].kind == '-' {
				dels = append(dels, ops[i])
			} else {
				adds = append(adds, ops[i])
			}
			i++
		}
		n := len(dels)
		if len(adds) > n {
			n = len(adds)
		}
		for k := 0; k < n; k++ {
			row := diffRow{kind: '~'}
			if k < len(dels) {
				row.left, row.lno = a[dels[k].a], dels[k].a+1
			}
			if k < len(adds) {
				row.right, row.rno = b[adds[k].b], adds[k].b+1
			}
			switch {
			case k >= len(adds):
				row.kind = '-'
			case k >= len(dels):
				row.kind = '+'
			}
			rows = append(rows, row)
		}
	}
	return rows
}

type diffOp struct {
	kind byte // '=', '-', '+'
	a, b int  // indexes into the two inputs
}

// myersDiff returns the edit script turning a into b.
func myersDiff(a, b []string) []diffOp {
	n, m := len(a), len(b)
	max := n + m
	if max == 0 {
		return nil
	}
	v := make([]int, 2*max+2)
	var trace [][]int
	idx := func(k int) int { return k + max }
	found := false
	for d := 0; d <= max && !found; d++ {
		trace = append(trace, append([]int(nil), v...))
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[idx(k-1)] < v[idx(k+1)]) {
				x = v[idx(k+1)]
			} else {
				x = v[idx(k-1)] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[idx(k)] = x
			if x >= n && y >= m {
				found = true
				break
			}
		}
	}
	// Backtrack.
	var ops []diffOp
	x, y := n, m
	for d := len(trace) - 1; d >= 0; d-- {
		vv := trace[d]
		k := x - y
		var prevK int
		if k == -d || (k != d && vv[idx(k-1)] < vv[idx(k+1)]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := vv[idx(prevK)]
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			x--
			y--
			ops = append(ops, diffOp{'=', x, y})
		}
		if d > 0 {
			if x == prevX {
				y--
				ops = append(ops, diffOp{'+', x, y})
			} else {
				x--
				ops = append(ops, diffOp{'-', x, y})
			}
		}
	}
	for i, j := 0, len(ops)-1; i < j; i, j = i+1, j-1 {
		ops[i], ops[j] = ops[j], ops[i]
	}
	return ops
}

func (v *DiffViewer) rowsVisible(a *App) int { return maxInt(1, a.h-2) }

func (v *DiffViewer) jump(a *App, dir int) {
	i := v.top
	for {
		i += dir
		if i < 0 || i >= len(v.rows) {
			a.setStatus("No more differences")
			return
		}
		if v.rows[i].kind != '=' && (i == 0 || v.rows[i-1].kind == '=') {
			v.top = i
			return
		}
	}
}

func (v *DiffViewer) HandleKey(a *App, ev *tcell.EventKey) {
	rows := v.rowsVisible(a)
	maxTop := maxInt(0, len(v.rows)-rows)
	switch keyName(ev) {
	case "Esc", "F3", "F10", "q", "Q":
		a.pop()
	case "Up":
		v.top = clampInt(v.top-1, 0, maxTop)
	case "Down":
		v.top = clampInt(v.top+1, 0, maxTop)
	case "PgUp":
		v.top = clampInt(v.top-rows, 0, maxTop)
	case "PgDn", "Space":
		v.top = clampInt(v.top+rows, 0, maxTop)
	case "Home":
		v.top = 0
	case "End":
		v.top = maxTop
	case "Left":
		v.off = maxInt(0, v.off-8)
	case "Right":
		v.off += 8
	case "n", "N", "F7":
		v.jump(a, 1)
	case "p", "P", "Shift+F7":
		v.jump(a, -1)
	}
}

func (v *DiffViewer) Draw(a *App) {
	th := a.theme
	a.fill(0, 0, a.w, a.h, th.Viewer)
	half := (a.w - 1) / 2
	a.fill(0, 0, a.w, 1, th.KeyLabel)
	a.print(0, 0, half, th.KeyLabel, fitRight(v.left, half))
	a.print(half+1, 0, a.w-half-1, th.KeyLabel, fitRight(v.right, a.w-half-1))
	rows := v.rowsVisible(a)
	gut := 6 // line number + marker
	_, bg, _ := th.Viewer.Decompose()
	numSt := th.ListDim.Background(bg)
	sepSt := th.Border.Background(bg)
	for i := 0; i < rows; i++ {
		idx := v.top + i
		if idx >= len(v.rows) {
			break
		}
		row := v.rows[idx]
		y := 1 + i
		st := th.Viewer
		mark := ' '
		switch row.kind {
		case '-':
			st, mark = th.DiffDel, '-'
		case '+':
			st, mark = th.DiffAdd, '+'
		case '~':
			st, mark = th.DiffChange, '~'
		}
		draw := func(x, w int, text string, no int, present bool) {
			if !present {
				return
			}
			num := ""
			if no > 0 {
				num = fmt.Sprintf("%4d", no)
			}
			a.print(x, y, 4, numSt, num)
			a.print(x+4, y, 1, st, string(mark))
			text = strings.ReplaceAll(text, "\t", "    ")
			if v.off > 0 {
				text = runewidth.TruncateLeft(text, v.off, "")
			}
			a.print(x+gut, y, w-gut, st, text)
		}
		draw(0, half, row.left, row.lno, row.lno > 0 || row.kind == '=')
		a.screen.SetContent(half, y, '│', nil, sepSt)
		draw(half+1, a.w-half-1, row.right, row.rno, row.rno > 0 || row.kind == '=')
	}
	foot := fmt.Sprintf(" %d differing rows   n/p: next/previous   Left/Right: scroll   Esc: close ", v.changes)
	a.fill(0, a.h-1, a.w, 1, th.KeyLabel)
	a.print(0, a.h-1, a.w, th.KeyLabel, fitLeft(foot, a.w))
}
