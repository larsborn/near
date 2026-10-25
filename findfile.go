package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// findAreas are the choices of FAR's "Select search area" field.
var findAreas = []string{"From the current folder", "Current folder only", "Selected folders", "From the root folder"}

// findSpec is what the Find file dialog asks for.
type findSpec struct {
	Masks      string // "*.*", "*.go;*.md", ...
	Text       string // content to look for, "" for none
	Hex        bool   // Text is a hex string
	CaseSens   bool
	WholeWords bool
	NotContain bool // keep files that do not contain Text
	Folders    bool // directories can be results too
	Links      bool // follow symbolic links to directories
	Area       int  // index into findAreas
}

// findState marks a tab that shows the results of a search instead of a
// plain directory listing. Result names are paths relative to the tab's
// directory, so everything that joins Path and Name keeps working.
type findState struct {
	spec    findSpec
	results []Entry
}

// label describes the search for the panel's path row, in at most w cells;
// the directory is clipped from the left, the description stays.
func (f *findState) label(path string, w int) string {
	s := fmt.Sprintf("Find %s", f.spec.Masks)
	if f.spec.Text != "" {
		s += fmt.Sprintf(" containing %q", f.spec.Text)
	}
	s += " in "
	return s + fitRight(path, maxInt(0, w-runewidth.StringWidth(s)))
}

// FindDialog is FAR's Alt+F7 "Find file" dialog.
type FindDialog struct {
	spec  findSpec
	masks *LineEdit
	text  *LineEdit
	focus int // see findFocus*
}

const (
	findFocusMasks = iota
	findFocusText
	findFocusTextHex
	findFocusCase
	findFocusWords
	findFocusNot
	findFocusFolders
	findFocusLinks
	findFocusArea
	findFocusFind
	findFocusCancel
	findFocusCount
)

// showFindFile opens the Find file dialog with the last settings.
func (a *App) showFindFile() {
	spec := a.lastFind
	if spec.Masks == "" {
		spec = findSpec{Masks: "*.*", Folders: true, Links: true}
	}
	d := &FindDialog{spec: spec, masks: newLineEdit(spec.Masks), text: newLineEdit(spec.Text)}
	d.masks.Anchor, d.masks.Cur = 0, len(d.masks.Text) // typing replaces the old mask
	a.push(d)
}

func (d *FindDialog) accept(a *App) {
	d.spec.Masks = strings.TrimSpace(d.masks.String())
	d.spec.Text = d.text.String()
	if d.spec.Masks == "" {
		d.spec.Masks = "*.*"
	}
	if d.spec.Hex {
		if _, err := hex.DecodeString(strings.ReplaceAll(d.spec.Text, " ", "")); err != nil {
			a.setErr(fmt.Errorf("hex string: %v", err))
			return
		}
	}
	a.pop()
	a.lastFind = d.spec
	a.runFind(a.tab(), d.spec)
}

func (d *FindDialog) toggle(which int) {
	d.focus = which
	switch which {
	case findFocusCase:
		d.spec.CaseSens = !d.spec.CaseSens
	case findFocusWords:
		d.spec.WholeWords = !d.spec.WholeWords
	case findFocusNot:
		d.spec.NotContain = !d.spec.NotContain
	case findFocusFolders:
		d.spec.Folders = !d.spec.Folders
	case findFocusLinks:
		d.spec.Links = !d.spec.Links
	}
}

func (d *FindDialog) HandleKey(a *App, ev *tcell.EventKey) {
	name := keyName(ev)
	switch name {
	case "Esc":
		a.pop()
		return
	case "Tab", "Down":
		d.focus = (d.focus + 1) % findFocusCount
		return
	case "Shift+Tab", "Up":
		d.focus = (d.focus + findFocusCount - 1) % findFocusCount
		return
	case "Enter":
		if d.focus == findFocusCancel {
			a.pop()
		} else {
			d.accept(a)
		}
		return
	}
	inText := d.focus == findFocusMasks || d.focus == findFocusText
	if ev.Key() == tcell.KeyRune && (ev.Modifiers()&tcell.ModAlt != 0 || !inText) {
		switch unicode.ToLower(ev.Rune()) {
		case 'm':
			d.focus = findFocusMasks
			return
		case 't':
			d.focus = findFocusTextHex
			d.spec.Hex = false
			return
		case 'h':
			d.focus = findFocusTextHex
			d.spec.Hex = true
			return
		case 'c':
			d.toggle(findFocusCase)
			return
		case 'w':
			d.toggle(findFocusWords)
			return
		case 'n':
			d.toggle(findFocusNot)
			return
		case 'o':
			d.toggle(findFocusFolders)
			return
		case 'k':
			d.toggle(findFocusLinks)
			return
		case 'a':
			d.focus = findFocusArea
			return
		case 'f':
			d.accept(a)
			return
		}
	}
	switch d.focus {
	case findFocusMasks:
		d.masks.HandleKey(ev, a)
	case findFocusText:
		d.text.HandleKey(ev, a)
	case findFocusTextHex:
		switch name {
		case "Left", "Right", "Space":
			d.spec.Hex = !d.spec.Hex
		}
	case findFocusCase, findFocusWords, findFocusNot, findFocusFolders, findFocusLinks:
		if name == "Space" {
			d.toggle(d.focus)
		}
	case findFocusArea:
		switch name {
		case "Left":
			d.spec.Area = (d.spec.Area + len(findAreas) - 1) % len(findAreas)
		case "Right", "Space":
			d.spec.Area = (d.spec.Area + 1) % len(findAreas)
		}
	default:
		switch name {
		case "Left", "Right":
			if d.focus == findFocusFind {
				d.focus = findFocusCancel
			} else {
				d.focus = findFocusFind
			}
		case "Space":
			if d.focus == findFocusCancel {
				a.pop()
			} else {
				d.accept(a)
			}
		}
	}
}

func (d *FindDialog) Draw(a *App) {
	th := a.theme
	w := clampInt(a.w-10, 56, 76)
	h := 16
	x, y := a.center(w, h)
	a.drawBox(x, y, w, h, th.Dialog, "Find file")
	inner := w - 4
	fieldW := inner - 1
	field := func(row int, e *LineEdit, focused bool) {
		st := th.Input
		if focused {
			st = th.InputFocus
		}
		e.Draw(a, x+2, row, fieldW, st, focused)
		a.print(x+2+fieldW, row, 1, th.Dialog, "↓")
	}
	check := func(cx, row int, on, focused bool, text string, hot int) {
		box := "[ ] "
		if on {
			box = "[x] "
		}
		bst := th.Dialog
		if focused {
			bst = th.ButtonActive
		}
		a.print(cx, row, 4, bst, box)
		a.printHot(cx+4, row, x+w-2-(cx+4), th.Dialog, th.Hotkey, text, hot)
	}
	radio := func(cx, row int, on, focused bool, text string, hot int) int {
		dot := "( ) "
		if on {
			dot = "(•) "
		}
		bst := th.Dialog
		if focused {
			bst = th.ButtonActive
		}
		a.print(cx, row, 4, bst, dot)
		a.printHot(cx+4, row, inner, th.Dialog, th.Hotkey, text, hot)
		return cx + 4 + runewidth.StringWidth(text) + 2
	}

	a.printHot(x+2, y+1, inner, th.Dialog, th.Hotkey, "A file mask or several file masks:", 7) // m
	field(y+2, d.masks, d.focus == findFocusMasks)
	a.separator(x, y+3, w, th.Dialog)

	a.print(x+2, y+4, inner, th.Dialog, "Containing")
	cx := radio(x+2+12, y+4, !d.spec.Hex, d.focus == findFocusTextHex && !d.spec.Hex, "Text", 0)
	radio(cx, y+4, d.spec.Hex, d.focus == findFocusTextHex && d.spec.Hex, "Hex", 0)
	field(y+5, d.text, d.focus == findFocusText)
	a.separator(x, y+6, w, th.Dialog)

	mid := x + w/2
	check(x+2, y+7, d.spec.CaseSens, d.focus == findFocusCase, "Case sensitive", 0)
	check(x+2, y+8, d.spec.WholeWords, d.focus == findFocusWords, "Whole words", 0)
	check(x+2, y+9, d.spec.NotContain, d.focus == findFocusNot, "Not containing", 0)
	for row := y + 7; row <= y+9; row++ {
		a.screen.SetContent(mid, row, '│', nil, th.Dialog)
	}
	check(mid+2, y+7, d.spec.Folders, d.focus == findFocusFolders, "Search for folders", 12)   // o
	check(mid+2, y+8, d.spec.Links, d.focus == findFocusLinks, "Search in symbolic links", 23) // k
	a.separator(x, y+10, w, th.Dialog)

	a.printHot(x+2, y+11, inner, th.Dialog, th.Hotkey, "Select search area:", 14) // a
	ast := th.Input
	if d.focus == findFocusArea {
		ast = th.InputFocus
	}
	aw := clampInt(fieldW/2, 24, fieldW)
	a.print(x+2, y+12, aw, ast, padRight(findAreas[d.spec.Area], aw))
	a.print(x+2+aw, y+12, 1, th.Dialog, "↓")
	a.separator(x, y+13, w, th.Dialog)

	find := "[ Find ]"
	if d.focus != findFocusCancel {
		find = "{ Find }"
	}
	cancel := "[ Cancel ]"
	bw := runewidth.StringWidth(find) + 1 + runewidth.StringWidth(cancel)
	bx := x + (w-bw)/2
	fSt, cSt := th.Button, th.Button
	if d.focus == findFocusFind {
		fSt = th.ButtonActive
	}
	if d.focus == findFocusCancel {
		cSt = th.ButtonActive
	}
	a.printHot(bx, y+14, w, fSt, th.Hotkey.Background(bgOf(fSt)), find, 2)
	a.printHot(bx+runewidth.StringWidth(find)+1, y+14, w, cSt, th.Hotkey.Background(bgOf(cSt)), cancel, -1)
}

// ---------------------------------------------------------------- running

// runFind searches below the tab's directory and turns the tab into a
// results listing. Esc stops the search and shows what was found so far.
func (a *App) runFind(t *Tab, spec findSpec) {
	root := t.Path
	var starts []string
	switch findAreas[spec.Area] {
	case "From the root folder":
		root = filepath.VolumeName(t.Path) + string(filepath.Separator)
		starts = []string{root}
	case "Selected folders":
		for _, e := range t.Targets() {
			if e.IsDir && !e.IsUp {
				starts = append(starts, filepath.Join(t.Path, e.Name))
			}
		}
		if len(starts) == 0 {
			a.setErr(fmt.Errorf("no folders selected"))
			return
		}
	default:
		starts = []string{t.Path}
	}
	recurse := findAreas[spec.Area] != "Current folder only"
	m, err := newFindMatcher(spec, a.cfg.NotesFile)
	if err != nil {
		a.setErr(err)
		return
	}
	var results []Entry
	stopped := false
	for _, s := range starts {
		if !findWalk(s, root, recurse, spec, m, func(dir string, e *Entry) bool {
			if e != nil {
				results = append(results, *e)
			}
			return a.progress("Find file", fmt.Sprintf("%d found  %s", len(results), dir), -1, -1)
		}) {
			stopped = true
			break
		}
	}
	a.lastProgress = time.Time{}
	if root != t.Path {
		if cur := t.Current(); cur != nil {
			t.posMem[t.Path] = cur.Name
		}
		t.Path = root
	}
	t.Find = &findState{spec: spec, results: results}
	t.Cursor, t.Top = 0, 0
	t.Selected = map[string]bool{}
	a.applyFind(t)
	msg := fmt.Sprintf("Found %d entries", len(results))
	if stopped {
		msg = fmt.Sprintf("Search stopped: %d entries found so far", len(results))
	}
	a.setStatus(msg)
}

// applyFind installs the results as the tab's listing, dropping entries
// that have disappeared since the search.
func (a *App) applyFind(t *Tab) {
	entries := make([]Entry, 0, len(t.Find.results))
	for _, e := range t.Find.results {
		if _, err := os.Lstat(filepath.Join(t.Path, e.Name)); err == nil {
			entries = append(entries, e)
		}
	}
	t.Find.results = entries
	t.Apply(append([]Entry{}, entries...), nil, nil)
}

// leaveFind returns a results tab to the plain listing of its directory.
func (a *App) leaveFind(t *Tab) {
	t.Find = nil
	t.Cursor, t.Top = 0, 0
	t.Selected = map[string]bool{}
	a.loadTab(t)
}

// findWalk lists dir and its subdirectories (when recurse), reporting every
// matching entry with its name relative to root. It returns false when the
// visit callback asked to stop.
func findWalk(dir, root string, recurse bool, spec findSpec, m *findMatcher, visit func(dir string, e *Entry) bool) bool {
	if !visit(dir, nil) {
		return false
	}
	des, err := os.ReadDir(dir)
	if err != nil {
		return true
	}
	sort.Slice(des, func(i, j int) bool { return naturalCompare(des[i].Name(), des[j].Name()) < 0 })
	for _, de := range des {
		name := de.Name()
		if name == m.notesFile {
			continue
		}
		full := filepath.Join(dir, name)
		rel, err := filepath.Rel(root, full)
		if err != nil {
			continue
		}
		e := Entry{Name: rel}
		if info, ierr := de.Info(); ierr == nil {
			e.Mode, e.Size, e.ModTime = info.Mode(), info.Size(), info.ModTime()
			e.Hidden = strings.HasPrefix(name, ".") || hasHiddenAttr(info)
		}
		isDir := de.IsDir()
		if de.Type()&fs.ModeSymlink != 0 {
			e.IsLink = true
			if st, serr := os.Stat(full); serr == nil && st.IsDir() {
				isDir = true
			}
		}
		e.IsDir = isDir
		// Folders are results only when no content is asked for, as in FAR.
		if m.matchName(name, rel) && (!isDir || spec.Folders && m.needle == nil) && (isDir || m.matchContent(full)) {
			if !visit(dir, &e) {
				return false
			}
		}
		if isDir && recurse && (!e.IsLink || spec.Links) {
			if !findWalk(full, root, true, spec, m, visit) {
				return false
			}
		}
	}
	return true
}

// ---------------------------------------------------------------- matching

type findMatcher struct {
	masks     []string // lowercased on case-folding platforms
	needle    []byte   // content to find, lowercased unless case sensitive
	spec      findSpec
	notesFile string
}

func newFindMatcher(spec findSpec, notesFile string) (*findMatcher, error) {
	m := &findMatcher{spec: spec, notesFile: notesFile}
	for _, mask := range strings.FieldsFunc(spec.Masks, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
		if mask == "*.*" {
			mask = "*"
		}
		if pathsEqualFold() {
			mask = strings.ToLower(mask)
		}
		if _, err := filepath.Match(mask, ""); err != nil {
			return nil, fmt.Errorf("mask %q: %v", mask, err)
		}
		m.masks = append(m.masks, mask)
	}
	if spec.Text != "" {
		if spec.Hex {
			b, err := hex.DecodeString(strings.ReplaceAll(spec.Text, " ", ""))
			if err != nil {
				return nil, fmt.Errorf("hex string: %v", err)
			}
			m.needle = b
		} else if spec.CaseSens {
			m.needle = []byte(spec.Text)
		} else {
			m.needle = bytes.ToLower([]byte(spec.Text))
		}
	}
	return m, nil
}

// matchName reports whether a base name (or, for masks with a separator,
// the relative path) matches one of the masks.
func (m *findMatcher) matchName(name, rel string) bool {
	if len(m.masks) == 0 {
		return true
	}
	if pathsEqualFold() {
		name, rel = strings.ToLower(name), strings.ToLower(rel)
	}
	rel = filepath.ToSlash(rel)
	for _, mask := range m.masks {
		subject := name
		if strings.ContainsAny(mask, `/\`) {
			subject = rel
			mask = filepath.ToSlash(mask)
		}
		if ok, _ := filepath.Match(mask, subject); ok {
			return true
		}
	}
	return false
}

// findChunk is how much of a file is examined at a time.
const findChunk = 1 << 20

// matchContent reports whether the file satisfies the content condition.
func (m *findMatcher) matchContent(path string) bool {
	if m.needle == nil {
		return true
	}
	found := m.contains(path)
	if m.spec.NotContain {
		return !found
	}
	return found
}

func (m *findMatcher) contains(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	keep := len(m.needle) - 1
	if m.spec.WholeWords {
		keep++ // one more byte so the boundary after a match is visible
	}
	buf := make([]byte, 0, findChunk+keep)
	for {
		n, err := f.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		if n > 0 {
			hay := buf
			if !m.spec.Hex && !m.spec.CaseSens {
				hay = bytes.ToLower(buf)
			}
			if m.findIn(hay, err == io.EOF) {
				return true
			}
		}
		if err != nil {
			return false
		}
		if len(buf) > keep {
			copy(buf, buf[len(buf)-keep:])
			buf = buf[:keep]
		}
	}
}

// findIn looks for the needle in hay, honouring whole-word boundaries; a
// match at the very end is only accepted when the file ends there (atEnd).
func (m *findMatcher) findIn(hay []byte, atEnd bool) bool {
	off := 0
	for {
		i := bytes.Index(hay[off:], m.needle)
		if i < 0 {
			return false
		}
		i += off
		end := i + len(m.needle)
		if !m.spec.WholeWords {
			return true
		}
		if end == len(hay) && !atEnd {
			return false // decide once the next chunk shows what follows
		}
		before := i == 0 || !isWordByte(hay[i-1])
		after := end >= len(hay) || !isWordByte(hay[end])
		if before && after {
			return true
		}
		off = i + 1
	}
}

func isWordByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}
