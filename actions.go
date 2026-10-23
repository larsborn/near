package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

// ---------------------------------------------------------------- selection

func (a *App) selectPattern(selectIt bool) {
	verb := "Select"
	if !selectIt {
		verb = "Unselect"
	}
	a.input(verb+" by pattern", "Glob (*.txt), or re:<regex>:", "*", func(pat string) {
		pat = strings.TrimSpace(pat)
		if pat == "" {
			return
		}
		t := a.tab()
		var re *regexp.Regexp
		if strings.HasPrefix(pat, "re:") {
			var err error
			if re, err = regexp.Compile("(?i)" + pat[3:]); err != nil {
				a.setErr(err)
				return
			}
		}
		n := 0
		for _, e := range t.Entries {
			if e.IsUp {
				continue
			}
			var ok bool
			if re != nil {
				ok = re.MatchString(e.Name)
			} else {
				ok, _ = filepath.Match(strings.ToLower(pat), strings.ToLower(e.Name))
			}
			if !ok {
				continue
			}
			n++
			if selectIt {
				t.Selected[e.Name] = true
			} else {
				delete(t.Selected, e.Name)
			}
		}
		a.setStatus(fmt.Sprintf("%d entries matched", n))
	})
}

// ---------------------------------------------------------------- progress

// progress draws the box for a long-running synchronous operation
// (deleting, comparing) and returns false when the user pressed Esc
// meanwhile. Copies and moves use the transfer queue instead.
func (a *App) progress(title, item string, done, total int64) bool {
	for a.screen.HasPendingEvent() {
		if ev, ok := a.screen.PollEvent().(*tcell.EventKey); ok && ev.Key() == tcell.KeyEscape {
			a.lastProgress = time.Time{}
			return false
		}
	}
	if item == a.lastProgressItem && time.Since(a.lastProgress) < 80*time.Millisecond {
		return true
	}
	a.lastProgress, a.lastProgressItem = time.Now(), item
	a.spinner++
	a.drawAll() // base screen without presenting it; the box goes on top, then one Show
	th := a.theme
	w := clampInt(a.w-10, 40, 76)
	h := 5
	x, y := a.center(w, h)
	spin := []string{"|", "/", "-", "\\"}[a.spinner%4]
	a.drawBox(x, y, w, h, th.Dialog, title+" "+spin)
	a.print(x+2, y+1, w-4, th.Dialog, fitRight(item, w-4))
	a.print(x+2, y+3, w-4, th.DialogHint, "Esc: cancel")
	a.screen.Show()
	return true
}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	sec := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%d:%02d", m, sec)
}

// ---------------------------------------------------------------- copy / move

func (a *App) copyOrMove(move bool) {
	t := a.tab()
	targets := t.Targets()
	if len(targets) == 0 {
		return
	}
	a.copyMoveDialog(move, targets, a.other().Path, func(dst string, opts copyOptions) {
		dst = strings.TrimSpace(dst)
		if dst == "" {
			return
		}
		if !filepath.IsAbs(expandPath(dst)) {
			dst = filepath.Join(t.Path, dst)
		}
		dst = absPath(dst)
		a.doCopyOrMove(t, targets, dst, move, opts)
	})
}

// doCopyOrMove copies or moves targets to dst. opts.Policy is one of
// existingPolicies: "Ask" confirms when destinations exist, "Overwrite" and
// "Skip" decide for all, "Only newer" overwrites when the source is newer.
func (a *App) doCopyOrMove(tab *Tab, targets []Entry, dst string, move bool, opts copyOptions) {
	policy := opts.Policy
	dstIsDir := isDir(dst)
	if len(targets) > 1 && !dstIsDir {
		a.confirm("Destination does not exist", []string{"Create directory " + fitRight(dst, 50) + "?"}, []string{"Create", "Cancel"}, func(r int) {
			if r != 0 {
				return
			}
			if err := os.MkdirAll(dst, 0o755); err != nil {
				a.setErr(err)
				return
			}
			a.doCopyOrMove(tab, targets, dst, move, opts)
		})
		return
	}
	// Destination path per entry.
	type job struct{ src, dst, name string }
	var jobs []job
	conflicts := 0
	for _, e := range targets {
		src := filepath.Join(tab.Path, e.Name)
		d := dst
		if dstIsDir {
			d = filepath.Join(dst, e.Name)
		}
		if _, err := os.Lstat(d); err == nil && !sameFile(src, d) {
			conflicts++
		}
		jobs = append(jobs, job{src, d, e.Name})
	}
	run := func(overwriteAll bool) {
		t := &transfer{move: move, flags: copyFlags{Overwrite: overwriteAll, Deref: opts.Deref, KeepTimes: opts.KeepTimes},
			policy: policy, srcDir: tab.Path, notes: a.cfg.NotesFile}
		for _, j := range jobs {
			t.jobs = append(t.jobs, transferJob{src: j.src, dst: j.dst, name: j.name})
		}
		tab.Selected = map[string]bool{}
		a.startTransfer(t, opts.Background)
	}
	switch policy {
	case "Overwrite":
		run(true)
	case "Skip", "Only newer":
		run(false)
	default: // Ask
		if conflicts > 0 {
			a.confirm("Destination exists", []string{fmt.Sprintf("%d entries already exist in the destination.", conflicts)},
				[]string{"Overwrite", "Skip existing", "Cancel"}, func(r int) {
					switch r {
					case 0:
						run(true)
					case 1:
						run(false)
					}
				})
			return
		}
		run(false)
	}
}

// ---------------------------------------------------------------- rename / mkdir / delete

func (a *App) renameCurrent() {
	t := a.tab()
	e := t.Current()
	if e == nil || e.IsUp {
		return
	}
	old := e.Name
	a.input("Rename", "New name:", old, func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || name == old {
			return
		}
		if strings.ContainsAny(name, `/\`) {
			a.setErr(fmt.Errorf("name must not contain path separators"))
			return
		}
		if _, err := os.Lstat(filepath.Join(t.Path, name)); err == nil && !strings.EqualFold(name, old) {
			a.setErr(fmt.Errorf("%s already exists", name))
			return
		}
		if err := renameMany(t.Path, []renamePair{{Old: old, New: name}}, a.cfg.NotesFile); err != nil {
			a.setErr(err)
		}
		a.reloadAll()
		t.SetCursorOn(name)
	})
}

func (a *App) makeDir() {
	t := a.tab()
	a.input("Create directory", "Name:", "", func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		full := name
		if !filepath.IsAbs(expandPath(name)) {
			full = filepath.Join(t.Path, name)
		}
		if err := os.MkdirAll(full, 0o755); err != nil {
			a.setErr(err)
			return
		}
		a.reloadAll()
		t.SetCursorOn(strings.Split(name, string(filepath.Separator))[0])
	})
}

func (a *App) deleteSelected() {
	t := a.tab()
	targets := t.Targets()
	if len(targets) == 0 {
		return
	}
	run := func() {
		var err error
		done := 0
		for _, e := range targets {
			full := filepath.Join(t.Path, e.Name)
			if err = deleteTree(full, func(item string, _, _ int64) bool { return a.progress("Deleting", item, -1, -1) }); err != nil {
				break
			}
			deleteNote(t.Path, e.Name, a.cfg.NotesFile)
			done++
		}
		t.Selected = map[string]bool{}
		a.reloadAll()
		if err != nil {
			a.setErr(fmt.Errorf("delete: %v (%d done)", err, done))
		} else {
			a.setStatus(fmt.Sprintf("Deleted %d entries", done))
		}
	}
	if !a.cfg.ConfirmDelete {
		run()
		return
	}
	lines := []string{}
	for i, e := range targets {
		if i == 8 {
			lines = append(lines, fmt.Sprintf("… and %d more", len(targets)-8))
			break
		}
		n := e.Name
		if e.IsDir {
			n += string(filepath.Separator)
		}
		lines = append(lines, "  "+n)
	}
	a.confirm(fmt.Sprintf("Delete %d entries permanently?", len(targets)), lines, []string{"Delete", "Cancel"}, func(r int) {
		if r == 0 {
			run()
		}
	})
}

// ---------------------------------------------------------------- notes

func (a *App) editNote() {
	t := a.tab()
	e := t.Current()
	if e == nil || e.IsUp {
		return
	}
	name := e.Name
	a.textDialog("Note for "+fitLeft(name, 40), t.Notes[name], func(text string) {
		notes := loadNotes(t.Path, a.cfg.NotesFile)
		if strings.TrimSpace(text) == "" {
			delete(notes, name)
		} else {
			notes[name] = text
		}
		if err := saveNotes(t.Path, a.cfg.NotesFile, notes); err != nil {
			a.setErr(err)
			return
		}
		t.Notes = notes
		if strings.TrimSpace(text) == "" {
			a.setStatus("Note removed")
		} else {
			a.setStatus("Note saved")
		}
	})
}

// ---------------------------------------------------------------- menus

func (a *App) openMenu(m *Menu) {
	items := make([]ListItem, 0, len(m.Items))
	for i, it := range m.Items {
		if it.Title == "-" || it.Title == "---" {
			items = append(items, ListItem{Sep: true})
			continue
		}
		var hk rune
		if it.Hotkey != "" {
			hk = []rune(it.Hotkey)[0]
		}
		right := ""
		if it.Submenu != "" {
			right = "▸"
		}
		items = append(items, ListItem{Label: it.Title, Hotkey: hk, Right: right, Data: i})
	}
	d := a.list(m.Name, items, func(_ int, li ListItem) {
		it := m.Items[li.Data.(int)]
		if it.Submenu != "" {
			sub := a.cfg.menu(it.Submenu)
			if sub == nil {
				a.setErr(fmt.Errorf("no menu named %q", it.Submenu))
				return
			}
			a.openMenu(sub)
			return
		}
		if it.Command == "" {
			return
		}
		run := func() {
			cwd := a.tab().Path
			if it.Cwd != "" {
				cwd = a.expandPlaceholders(it.Cwd)
			}
			a.runShell(a.expandPlaceholders(it.Command), cwd, !it.NoWait, it.Detach)
		}
		if it.Confirm {
			a.confirm(it.Title, []string{fitLeft(a.expandPlaceholders(it.Command), a.w-10)}, []string{"Run", "Cancel"}, func(r int) {
				if r == 0 {
					run()
				}
			})
			return
		}
		run()
	})
	d.Hint = "Enter: run   Esc: close"
}

func (a *App) showMenuList() {
	if len(a.cfg.Menus) == 0 {
		a.setStatus("No menus defined in " + a.cfg.path)
		return
	}
	var items []ListItem
	for i, m := range a.cfg.Menus {
		items = append(items, ListItem{Label: m.Name, Right: normKey(m.Key), Data: i})
	}
	d := a.list("Menus", items, func(_ int, li ListItem) {
		a.openMenu(&a.cfg.Menus[li.Data.(int)])
	})
	d.Searchable = true
	d.Hint = "Menus are defined in " + fitRight(a.cfg.path, 50)
}

func (a *App) showSortMenu() {
	t := a.tab()
	var items []ListItem
	for i, n := range sortNames {
		r := ""
		if SortMode(i) == t.Sort {
			r = "•"
		}
		items = append(items, ListItem{Label: n, Hotkey: []rune(n)[0], Right: r, Data: i})
	}
	items = append(items, ListItem{Sep: true})
	rev := ""
	if t.SortRev {
		rev = "•"
	}
	items = append(items, ListItem{Label: "Reverse order", Hotkey: 'r', Right: rev, Data: -1})
	d := a.list("Sort by", items, func(_ int, li ListItem) {
		if i := li.Data.(int); i < 0 {
			t.SortRev = !t.SortRev
		} else {
			t.Sort = SortMode(i)
		}
		a.loadTab(t)
	})
	d.Cursor = int(t.Sort)
}

func (a *App) showLocations() {
	var items []ListItem
	for _, l := range listLocations() {
		items = append(items, ListItem{Label: l.Name, Right: "", Data: l.Path})
	}
	home, _ := os.UserHomeDir()
	items = append(items, ListItem{Sep: true}, ListItem{Label: "Home", Hotkey: '~', Right: fitRight(home, 30), Data: home})
	for _, b := range a.cfg.Bookmarks {
		p := expandPath(b.Path)
		if p == home {
			continue
		}
		items = append(items, ListItem{Label: b.Name, Right: fitRight(p, 30), Data: p})
	}
	items = append(items, ListItem{Sep: true})
	for i, p := range a.panels {
		for j, tab := range p.Tabs {
			side := "left"
			if i == 1 {
				side = "right"
			}
			items = append(items, ListItem{Label: fmt.Sprintf("%s tab %d", side, j+1), Right: fitRight(tab.Path, 36), Data: tab.Path})
		}
	}
	d := a.list("Locations", items, func(_ int, li ListItem) {
		if err := a.navigate(a.tab(), li.Data.(string), ""); err != nil {
			a.setErr(err)
		}
	})
	d.Searchable = true
	d.Width = clampInt(a.w-10, 40, 78)
}

// ---------------------------------------------------------------- view / edit

func (a *App) viewCurrent() {
	t := a.tab()
	e := t.Current()
	if e == nil || e.IsUp || e.IsDir {
		return
	}
	a.viewFile(filepath.Join(t.Path, e.Name))
}

func (a *App) viewFile(full string) {
	v, err := newFileViewer(full)
	if err != nil {
		a.setErr(err)
		return
	}
	a.fileHistory = pushHistory(a.fileHistory, full, historyMax)
	a.push(v)
}

func (a *App) editCurrent() {
	t := a.tab()
	e := t.Current()
	if e == nil || e.IsUp || e.IsDir {
		return
	}
	a.editFile(filepath.Join(t.Path, e.Name))
}
