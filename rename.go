package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

type renamePair struct {
	Old, New string
	Err      string
}

var caseNames = []string{"keep", "lower", "UPPER", "Title", "Sentence"}
var targetNames = []string{"name", "extension", "full name"}

// MassRenameDialog renames the selected entries with a regex rule and a
// live preview.
type MassRenameDialog struct {
	dir     string
	entries []Entry
	find    *LineEdit
	repl    *LineEdit
	start   *LineEdit
	stp     *LineEdit
	pad     *LineEdit
	caseM   int
	target  int
	focus   int
	pairs   []renamePair
	errs    int
	top     int
	findErr string
	onDone  func(pairs []renamePair)
}

func (a *App) massRename() {
	t := a.tab()
	targets := t.Targets()
	if len(targets) == 0 {
		return
	}
	d := &MassRenameDialog{
		dir:     t.Path,
		entries: targets,
		find:    newLineEdit(""),
		repl:    newLineEdit(""),
		start:   newLineEdit("1"),
		stp:     newLineEdit("1"),
		pad:     newLineEdit("1"),
	}
	d.onDone = func(pairs []renamePair) {
		if err := renameMany(t.Path, pairs, a.cfg.NotesFile); err != nil {
			a.setErr(err)
		} else {
			n := 0
			for _, p := range pairs {
				if p.Old != p.New {
					n++
				}
			}
			a.setStatus(fmt.Sprintf("Renamed %d entries", n))
		}
		t.Selected = map[string]bool{}
		a.reloadAll()
		if len(pairs) > 0 {
			t.SetCursorOn(pairs[0].New)
		}
	}
	d.recompute()
	a.push(d)
}

func (d *MassRenameDialog) intField(e *LineEdit, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(e.String()))
	if err != nil {
		return def
	}
	return n
}

// recompute builds the preview from the current rule.
func (d *MassRenameDialog) recompute() {
	d.findErr = ""
	var re *regexp.Regexp
	if f := d.find.String(); f != "" {
		var err error
		re, err = regexp.Compile(f)
		if err != nil {
			d.findErr = err.Error()
		}
	}
	start, step, pad := d.intField(d.start, 1), d.intField(d.stp, 1), d.intField(d.pad, 1)
	d.pairs = d.pairs[:0]
	n := start
	for _, e := range d.entries {
		name, ext := e.Name, ""
		if !e.IsDir {
			if x := filepath.Ext(e.Name); x != "" && x != e.Name {
				name, ext = e.Name[:len(e.Name)-len(x)], x[1:]
			}
		}
		tokens := map[string]string{
			"{n}":      fmt.Sprintf("%0*d", pad, n),
			"{name}":   name,
			"{ext}":    ext,
			"{date}":   e.ModTime.Format("2006-01-02"),
			"{time}":   e.ModTime.Format("150405"),
			"{parent}": filepath.Base(d.dir),
		}
		n += step
		template := d.repl.String()
		for k, v := range tokens {
			// $ in token values would be read as a group reference by regexp.
			template = strings.ReplaceAll(template, k, strings.ReplaceAll(v, "$", "$$"))
		}
		var subject string
		switch d.target {
		case 0:
			subject = name
		case 1:
			subject = ext
		default:
			subject = e.Name
		}
		var out string
		switch {
		case d.findErr != "":
			out = subject
		case re != nil:
			out = re.ReplaceAllString(subject, template)
		case template != "":
			out = strings.ReplaceAll(template, "$$", "$")
		default:
			out = subject
		}
		out = applyCase(out, d.caseM)
		var newName string
		switch d.target {
		case 0:
			newName = out
			if ext != "" {
				newName += "." + ext
			}
		case 1:
			newName = name
			if out != "" {
				newName += "." + out
			}
		default:
			newName = out
		}
		p := renamePair{Old: e.Name, New: newName}
		switch {
		case newName == "" || newName == "." || newName == "..":
			p.Err = "empty name"
		case strings.ContainsAny(newName, `/\`):
			p.Err = "path separator"
		}
		d.pairs = append(d.pairs, p)
	}
	d.errs = checkConflicts(d.dir, d.pairs)
}

// checkConflicts marks duplicate or already existing target names.
func checkConflicts(dir string, pairs []renamePair) int {
	fold := pathsEqualFold()
	key := func(s string) string {
		if fold {
			return strings.ToLower(s)
		}
		return s
	}
	olds := map[string]bool{}
	for _, p := range pairs {
		olds[key(p.Old)] = true
	}
	seen := map[string]int{}
	for i := range pairs {
		seen[key(pairs[i].New)]++
	}
	errs := 0
	for i := range pairs {
		p := &pairs[i]
		if p.Err == "" {
			switch {
			case seen[key(p.New)] > 1:
				p.Err = "duplicate"
			case p.Old != p.New && !olds[key(p.New)]:
				if _, err := os.Lstat(filepath.Join(dir, p.New)); err == nil {
					p.Err = "exists"
				}
			}
		}
		if p.Err != "" {
			errs++
		}
	}
	return errs
}

func applyCase(s string, mode int) string {
	switch mode {
	case 1:
		return strings.ToLower(s)
	case 2:
		return strings.ToUpper(s)
	case 3:
		r := []rune(strings.ToLower(s))
		up := true
		for i, c := range r {
			if up && unicode.IsLetter(c) {
				r[i] = unicode.ToUpper(c)
			}
			up = !unicode.IsLetter(c) && !unicode.IsDigit(c)
		}
		return string(r)
	case 4:
		r := []rune(strings.ToLower(s))
		for i, c := range r {
			if unicode.IsLetter(c) {
				r[i] = unicode.ToUpper(c)
				break
			}
		}
		return string(r)
	}
	return s
}

const renameFields = 7

func (d *MassRenameDialog) HandleKey(a *App, ev *tcell.EventKey) {
	rows := d.previewRows(a)
	switch keyName(ev) {
	case "Esc":
		a.pop()
		return
	case "Tab", "Down":
		d.focus = (d.focus + 1) % renameFields
		return
	case "Shift+Tab", "Up":
		d.focus = (d.focus + renameFields - 1) % renameFields
		return
	case "PgDn":
		d.top = clampInt(d.top+rows, 0, maxInt(0, len(d.pairs)-rows))
		return
	case "PgUp":
		d.top = clampInt(d.top-rows, 0, maxInt(0, len(d.pairs)-rows))
		return
	case "F4":
		a.pop()
		a.renameInEditor(d.dir, d.entries, d.onDone)
		return
	case "Enter":
		if d.findErr != "" {
			a.setErr(fmt.Errorf("regex: %s", d.findErr))
			return
		}
		if d.errs > 0 {
			a.setErr(fmt.Errorf("%d conflicting names, nothing renamed", d.errs))
			return
		}
		a.pop()
		d.onDone(d.pairs)
		return
	}
	switch d.focus {
	case 5, 6:
		n := keyName(ev)
		if n == "Left" || n == "Right" || n == "Space" {
			delta := 1
			if n == "Left" {
				delta = -1
			}
			if d.focus == 5 {
				d.caseM = (d.caseM + delta + len(caseNames)) % len(caseNames)
			} else {
				d.target = (d.target + delta + len(targetNames)) % len(targetNames)
			}
			d.recompute()
		}
	default:
		if d.field().HandleKey(ev) {
			d.recompute()
		}
	}
}

func (d *MassRenameDialog) field() *LineEdit {
	return []*LineEdit{d.find, d.repl, d.start, d.stp, d.pad}[d.focus]
}

func (d *MassRenameDialog) previewRows(a *App) int {
	return maxInt(1, a.h-4-9)
}

func (d *MassRenameDialog) Draw(a *App) {
	th := a.theme
	w := clampInt(a.w-4, 50, 120)
	rows := d.previewRows(a)
	if rows > len(d.pairs) {
		rows = len(d.pairs)
	}
	h := rows + 9
	x, y := a.center(w, h)
	a.drawBox(x, y, w, h, th.Dialog, fmt.Sprintf("Mass rename (%d entries)", len(d.entries)))
	lx := x + 2
	fx := x + 16
	fw := w - 18
	a.print(lx, y+1, 14, th.Dialog, "Find (regex):")
	a.print(lx, y+2, 14, th.Dialog, "Replace:")
	a.print(lx, y+3, 14, th.Dialog, "Counter:")
	a.print(lx, y+4, 14, th.Dialog, "Case/target:")
	styleFor := func(i int) tcell.Style {
		if d.focus == i {
			return th.InputFocus
		}
		return th.Input
	}
	d.find.Draw(a, fx, y+1, fw, styleFor(0), d.focus == 0)
	d.repl.Draw(a, fx, y+2, fw, styleFor(1), d.focus == 1)
	a.print(fx, y+3, 6, th.Dialog, "start")
	d.start.Draw(a, fx+6, y+3, 6, styleFor(2), d.focus == 2)
	a.print(fx+14, y+3, 5, th.Dialog, "step")
	d.stp.Draw(a, fx+19, y+3, 4, styleFor(3), d.focus == 3)
	a.print(fx+25, y+3, 4, th.Dialog, "pad")
	d.pad.Draw(a, fx+29, y+3, 4, styleFor(4), d.focus == 4)
	caseLabel := "< " + caseNames[d.caseM] + " >"
	a.print(fx, y+4, len(caseLabel), styleFor(5), caseLabel)
	tgtLabel := "< " + targetNames[d.target] + " >"
	a.print(fx+14, y+4, len(tgtLabel), styleFor(6), tgtLabel)
	hint := "Tokens: {n} {name} {ext} {date} {time} {parent}  groups: $1 ${name}"
	if d.findErr != "" {
		a.print(fx+32, y+4, w-(fx+32-x)-2, th.Conflict, fitLeft("regex: "+d.findErr, w-(fx+32-x)-2))
	} else {
		a.print(fx+32, y+4, w-(fx+32-x)-2, th.DialogHint, fitLeft(hint, w-(fx+32-x)-2))
	}
	a.hline(x+1, y+5, w-2, th.Dialog, '─')
	a.screen.SetContent(x, y+5, '├', nil, th.Dialog)
	a.screen.SetContent(x+w-1, y+5, '┤', nil, th.Dialog)

	// preview
	half := (w - 8) / 2
	if d.top > maxInt(0, len(d.pairs)-rows) {
		d.top = maxInt(0, len(d.pairs)-rows)
	}
	for i := 0; i < rows; i++ {
		idx := d.top + i
		if idx >= len(d.pairs) {
			break
		}
		p := d.pairs[idx]
		ry := y + 6 + i
		st := th.Dialog
		if p.Old == p.New {
			st = th.ListDim
		}
		a.print(x+2, ry, half, st, fitLeft(p.Old, half))
		a.print(x+2+half, ry, 3, th.DialogHint, " → ")
		nst := st
		if p.Err != "" {
			nst = th.Conflict
		}
		right := p.New
		if p.Err != "" {
			right += "  [" + p.Err + "]"
		}
		a.print(x+5+half, ry, half, nst, fitLeft(right, half))
	}
	a.hline(x+1, y+h-3, w-2, th.Dialog, '─')
	a.screen.SetContent(x, y+h-3, '├', nil, th.Dialog)
	a.screen.SetContent(x+w-1, y+h-3, '┤', nil, th.Dialog)
	status := fmt.Sprintf("%d entries", len(d.pairs))
	if d.errs > 0 {
		status = fmt.Sprintf("%d conflicts", d.errs)
	}
	foot := "Enter: rename   Tab: next field   Left/Right: change option   F4: edit names in editor   Esc: cancel"
	a.print(x+2, y+h-2, w-4-runewidth.StringWidth(status)-1, th.DialogHint, fitLeft(foot, w-4-runewidth.StringWidth(status)-1))
	sst := th.DialogHint
	if d.errs > 0 {
		sst = th.Conflict
	}
	a.print(x+w-2-runewidth.StringWidth(status), y+h-2, runewidth.StringWidth(status), sst, status)
}

// renameInEditor writes the names to a temporary file, opens the editor and
// applies the edited list line by line.
func (a *App) renameInEditor(dir string, entries []Entry, onDone func([]renamePair)) {
	f, err := os.CreateTemp("", "near-rename-*.txt")
	if err != nil {
		a.setErr(err)
		return
	}
	tmp := f.Name()
	for _, e := range entries {
		fmt.Fprintln(f, e.Name)
	}
	f.Close()
	a.editFileThen(tmp, !a.cfg.InternalEditor, func() {
		defer os.Remove(tmp)
		a.applyEditedNames(tmp, dir, entries, onDone)
	})
}

// applyEditedNames reads the edited name list back and applies it.
func (a *App) applyEditedNames(tmp, dir string, entries []Entry, onDone func([]renamePair)) {
	data, err := os.ReadFile(tmp)
	if err != nil {
		a.setErr(err)
		return
	}
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"), "\n")
	if len(lines) != len(entries) {
		a.message("Mass rename", fmt.Sprintf("Expected %d lines, got %d. Nothing renamed.", len(entries), len(lines)))
		return
	}
	var pairs []renamePair
	changed := 0
	for i, e := range entries {
		p := renamePair{Old: e.Name, New: strings.TrimSpace(lines[i])}
		if p.New == "" {
			p.Err = "empty name"
		} else if strings.ContainsAny(p.New, `/\`) {
			p.Err = "path separator"
		}
		if p.Old != p.New {
			changed++
		}
		pairs = append(pairs, p)
	}
	if errs := checkConflicts(dir, pairs); errs > 0 {
		var lines []string
		for _, p := range pairs {
			if p.Err != "" && len(lines) < 10 {
				lines = append(lines, fmt.Sprintf("%s → %s: %s", p.Old, p.New, p.Err))
			}
		}
		a.message(fmt.Sprintf("%d conflicts, nothing renamed", errs), lines...)
		return
	}
	if changed == 0 {
		a.setStatus("No names changed")
		return
	}
	a.confirm("Mass rename", []string{fmt.Sprintf("Rename %d entries?", changed)}, []string{"Rename", "Cancel"}, func(r int) {
		if r == 0 {
			onDone(pairs)
		}
	})
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
