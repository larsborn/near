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
	if t.Zip != nil {
		if move {
			a.setErr(fmt.Errorf("moving out of an archive is not possible; F5 extracts a copy"))
			return
		}
		a.copyMoveDialog(false, targets, a.other().diskDir(), func(dst string, _ copyOptions) {
			dst = strings.TrimSpace(dst)
			if dst == "" {
				return
			}
			if !filepath.IsAbs(expandPath(dst)) {
				dst = filepath.Join(a.other().diskDir(), dst)
			}
			if err := os.MkdirAll(absPath(dst), 0o755); err != nil {
				a.setErr(err)
				return
			}
			a.extractToDir(t, targets, absPath(dst))
		})
		return
	}
	a.copyMoveDialog(move, targets, a.other().diskDir(), func(dst string, opts copyOptions) {
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
			// Search results are named by their relative path; copy the
			// entry itself, not the directories above it.
			d = filepath.Join(dst, filepath.Base(e.Name))
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
		if !opts.KeepSelection {
			tab.Selected = map[string]bool{}
		}
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

// copyAsCurrent is FAR's Shift+F5: copy the entry under the cursor to
// another name beside it. The counterpart of Shift+F6, which renames it
// instead, so it works the same way — the cursor entry whatever is
// selected, and the selection is left alone afterwards. A path may be
// typed instead of a bare name, relative to the entry's own directory.
func (a *App) copyAsCurrent() {
	if a.inArchive("copying to a new name") {
		return
	}
	t := a.tab()
	e := t.Current()
	if e == nil || e.IsUp {
		return
	}
	src := filepath.Join(t.Path, e.Name)
	dir, old := t.Path, e.Name
	if t.Find != nil {
		// A search result is named by its relative path; copy it beside itself.
		dir, old = filepath.Join(t.Path, filepath.Dir(e.Name)), filepath.Base(e.Name)
	}
	title := "Copy"
	if e.IsDir {
		title = "Copy directory"
	}
	a.input(title, "Copy "+fitRight(old, 40)+" to:", old, func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		dst := expandPath(name)
		if !filepath.IsAbs(dst) {
			dst = filepath.Join(dir, dst)
		}
		dst = absPath(dst)
		// Copying a file onto itself would truncate it. An existing
		// directory is fine: the entry is then copied into it, as with F5.
		if sameFile(src, dst) && !isDir(dst) {
			a.setErr(fmt.Errorf("%s is the entry being copied", name))
			return
		}
		a.doCopyOrMove(t, []Entry{*e}, dst, false, copyOptions{KeepTimes: true, KeepSelection: true})
	})
}

func (a *App) renameCurrent() {
	if a.inArchive("renaming") {
		return
	}
	t := a.tab()
	e := t.Current()
	if e == nil || e.IsUp {
		return
	}
	dir, old := t.Path, e.Name
	if t.Find != nil {
		// A search result is named by its relative path; rename it in place.
		dir, old = filepath.Join(t.Path, filepath.Dir(e.Name)), filepath.Base(e.Name)
	}
	a.input("Rename", "New name:", old, func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || name == old {
			return
		}
		if strings.ContainsAny(name, `/\`) {
			a.setErr(fmt.Errorf("name must not contain path separators"))
			return
		}
		if _, err := os.Lstat(filepath.Join(dir, name)); err == nil && !strings.EqualFold(name, old) {
			a.setErr(fmt.Errorf("%s already exists", name))
			return
		}
		if err := renameMany(dir, []renamePair{{Old: old, New: name}}, a.cfg.NotesFile); err != nil {
			a.setErr(err)
		}
		a.reloadAll()
		t.SetCursorOn(name)
	})
}

func (a *App) makeDir() {
	if a.inArchive("creating a directory") {
		return
	}
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

func (a *App) deleteSelected(permanent bool) {
	if a.inArchive("deleting") {
		return
	}
	t := a.tab()
	targets := t.Targets()
	if len(targets) == 0 {
		return
	}
	run := func() {
		var err error
		done := 0
		if permanent {
			for _, e := range targets {
				full := filepath.Join(t.Path, e.Name)
				if err = deleteTree(full, func(item string, _, _ int64) bool { return a.progress("Deleting", item, -1, -1) }); err != nil {
					break
				}
				deleteNote(t.Path, e.Name, a.cfg.NotesFile)
				done++
			}
		} else {
			var paths []string
			for _, e := range targets {
				paths = append(paths, filepath.Join(t.Path, e.Name))
			}
			a.progress("Moving to the "+trashName, fitLeft(targets[0].Name, 60), -1, -1)
			if err = trashPaths(paths); err == nil {
				for _, e := range targets {
					deleteNote(t.Path, e.Name, a.cfg.NotesFile)
				}
				done = len(targets)
			}
		}
		t.Selected = map[string]bool{}
		a.reloadAll()
		switch {
		case err != nil:
			a.setErr(fmt.Errorf("delete: %v (%d done)", err, done))
		case permanent:
			a.setStatus(fmt.Sprintf("Deleted %d entries", done))
		default:
			a.setStatus(fmt.Sprintf("Moved %d entries to the %s", done, trashName))
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
	title := fmt.Sprintf("Move %d entries to the %s?", len(targets), trashName)
	button := "Move"
	if permanent {
		title = fmt.Sprintf("Delete %d entries permanently?", len(targets))
		button = "Delete"
	}
	a.confirm(title, lines, []string{button, "Cancel"}, func(r int) {
		if r == 0 {
			run()
		}
	})
}

// ---------------------------------------------------------------- notes

func (a *App) editNote() {
	if a.inArchive("editing a note") {
		return
	}
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

// openMenu shows a menu at the top level; Esc closes it.
func (a *App) openMenu(m *Menu) *ListDialog { return a.openMenuLevel(m, nil) }

// openMenuLevel shows one level of a menu. back, when set, reopens the
// level above, so that Esc (and Left) step out one menu at a time instead
// of closing the lot.
func (a *App) openMenuLevel(m *Menu, back func()) *ListDialog {
	items := make([]ListItem, 0, len(m.Items))
	for i, it := range m.Items {
		if it.Title == "-" || it.Title == "---" {
			items = append(items, ListItem{Sep: true, Data: i})
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
		items = append(items, ListItem{Label: it.Title, Hotkey: hk, Right: right, Sub: it.Submenu != "", Data: i})
	}
	name := m.Name // by name: editing may reallocate a.cfg.Menus, making m stale
	// Reopening this very level, for a submenu to come back to and for the
	// editing keys to use after they changed something.
	here := func() {
		if again := a.cfg.menu(name); again != nil {
			a.openMenuLevel(again, back)
		}
	}
	d := a.list(name, items, func(_ int, li ListItem) {
		it := m.Items[li.Data.(int)]
		if it.Submenu != "" {
			sub := a.cfg.menu(it.Submenu)
			if sub == nil {
				a.setErr(fmt.Errorf("no menu named %q", it.Submenu))
				return
			}
			a.openMenuLevel(sub, here)
			return
		}
		if it.Command == "" {
			return
		}
		a.runMenuItem(it)
	})
	d.Menu = true
	d.Hint = "Ins Del F4 Alt+F4 Ctrl+Up Ctrl+Down"
	if back != nil {
		d.Hint += "   Esc: back"
		d.OnClose = back
	}
	d.OnKey = func(a *App, d *ListDialog, ev *tcell.EventKey) bool { return a.userMenuKey(name, back, d, ev) }
	return d
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
	where := a.cfg.path
	if _, err := os.Stat(a.menusFile()); err == nil {
		where = a.menusFile()
	}
	d.Hint = "Menus are defined in " + fitRight(where, 50)
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
	if t.Zip != nil {
		tmp, err := a.zipCopyOut(t.Zip, t.Zip.inner(e.Name))
		if err != nil {
			a.setErr(err)
			return
		}
		a.viewFile(tmp)
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
	// Carry over what was last looked for inside files: the hits are marked
	// at once after a Find file content search, the view opens on the first
	// of them, and F7 offers the same text again.
	if v.query = a.lastContentSearch; v.query != "" {
		v.find(a, 0)
	}
	a.fileHistory = pushHistory(a.fileHistory, full, historyMax)
	a.push(v)
}

func (a *App) editCurrent() {
	if a.inArchive("editing") {
		return
	}
	t := a.tab()
	e := t.Current()
	if e == nil || e.IsUp || e.IsDir {
		return
	}
	a.editFile(filepath.Join(t.Path, e.Name))
}
