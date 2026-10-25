package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// FAR's user menu editing: inside a menu Ins inserts a command or a
// submenu, Del deletes, F4 edits, Alt+F4 edits the menu file, Ctrl+Up and
// Ctrl+Down move items. Menus changed this way live in menus.toml next to
// the config; once that file exists it replaces the [[menus]] of config.toml.

const menusFileName = "menus.toml"

// menuSeparatorHotkey is what FAR uses as the hot key of a separator.
const menuSeparatorHotkey = "--"

func (a *App) menusFile() string { return filepath.Join(a.cfg.dir, menusFileName) }

// loadMenusFile reads menus.toml; a missing file is reported as such.
func loadMenusFile(path string) ([]Menu, error) {
	var doc struct {
		Menus []Menu `toml:"menus"`
	}
	if _, err := toml.DecodeFile(path, &doc); err != nil {
		return nil, err
	}
	return doc.Menus, nil
}

// saveMenus writes the current menus to menus.toml.
func (a *App) saveMenus() error {
	var buf bytes.Buffer
	buf.WriteString("# Menus edited inside near (F2, then Ins / Del / F4 / Alt+F4 / Ctrl+Up / Ctrl+Down).\n")
	buf.WriteString("# While this file exists the [[menus]] of config.toml are ignored.\n\n")
	doc := struct {
		Menus []Menu `toml:"menus"`
	}{a.cfg.Menus}
	if err := toml.NewEncoder(&buf).Encode(doc); err != nil {
		return err
	}
	if err := os.MkdirAll(a.cfg.dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(a.menusFile(), buf.Bytes(), 0o644)
}

// menusChanged saves the menus, rebinds menu keys and reopens menu m with
// the cursor on item index cursor.
func (a *App) menusChanged(m *Menu, cursor int) {
	if err := a.saveMenus(); err != nil {
		a.setErr(err)
	}
	a.keymap = buildKeymap(a.cfg)
	if len(a.dialogs) > 0 {
		if _, ok := a.top().(*ListDialog); ok {
			a.pop()
		}
	}
	d := a.openMenu(m)
	for i, it := range d.Items {
		if idx, ok := it.Data.(int); ok && idx == cursor {
			d.Cursor = i
		}
	}
}

// userMenuKey handles the editing keys of a menu list.
func (a *App) userMenuKey(m *Menu, d *ListDialog, ev *tcell.EventKey) bool {
	idx := -1
	if d.Cursor >= 0 && d.Cursor < len(d.Items) {
		if i, ok := d.Items[d.Cursor].Data.(int); ok {
			idx = i
		}
	}
	switch keyName(ev) {
	case "Insert":
		a.insertMenuItem(m, maxInt(idx, 0))
	case "Delete":
		if idx >= 0 {
			a.deleteMenuItem(m, idx)
		}
	case "F4":
		if idx >= 0 {
			a.editMenuItem(m, idx)
		}
	case "Alt+F4":
		a.editMenusFile(m)
	case "Ctrl+Up":
		if idx > 0 {
			m.Items[idx], m.Items[idx-1] = m.Items[idx-1], m.Items[idx]
			a.menusChanged(m, idx-1)
		}
	case "Ctrl+Down":
		if idx >= 0 && idx < len(m.Items)-1 {
			m.Items[idx], m.Items[idx+1] = m.Items[idx+1], m.Items[idx]
			a.menusChanged(m, idx+1)
		}
	default:
		return false
	}
	return true
}

// insertMenuItem asks what to insert and puts it at position at.
func (a *App) insertMenuItem(m *Menu, at int) {
	a.confirm("User menu", []string{"Do you wish to insert a new command or a new menu?"},
		[]string{"Insert command", "Insert menu"}, func(r int) {
			if r < 0 {
				return
			}
			submenu := r == 1
			a.menuItemDialog(MenuItem{}, submenu, func(it MenuItem) {
				if submenu {
					it.Submenu = a.newSubmenu(it.Title)
				}
				at = clampInt(at, 0, len(m.Items))
				m.Items = append(m.Items[:at], append([]MenuItem{it}, m.Items[at:]...)...)
				a.menusChanged(m, at)
			})
		})
}

// newSubmenu creates an empty menu with a unique name derived from title.
func (a *App) newSubmenu(title string) string {
	name := strings.TrimSpace(title)
	if name == "" {
		name = "Submenu"
	}
	for n := 2; a.cfg.menu(name) != nil; n++ {
		name = fmt.Sprintf("%s %d", strings.TrimSpace(title), n)
	}
	a.cfg.Menus = append(a.cfg.Menus, Menu{Name: name})
	return name
}

func (a *App) deleteMenuItem(m *Menu, idx int) {
	it := m.Items[idx]
	what := fmt.Sprintf("Delete menu item %q?", it.Title)
	if it.Submenu != "" {
		what = fmt.Sprintf("Delete submenu %q?", it.Title)
	}
	if it.Title == "-" || it.Title == "---" {
		what = "Delete the separator?"
	}
	a.confirm("User menu", []string{what}, []string{"Delete", "Cancel"}, func(r int) {
		if r != 0 {
			return
		}
		m.Items = append(m.Items[:idx], m.Items[idx+1:]...)
		if it.Submenu != "" && !a.menuReferenced(it.Submenu) {
			for i := range a.cfg.Menus {
				if strings.EqualFold(a.cfg.Menus[i].Name, it.Submenu) {
					a.cfg.Menus = append(a.cfg.Menus[:i], a.cfg.Menus[i+1:]...)
					break
				}
			}
		}
		a.menusChanged(m, clampInt(idx, 0, len(m.Items)-1))
	})
}

// menuReferenced reports whether any item still points at the named menu.
func (a *App) menuReferenced(name string) bool {
	for _, m := range a.cfg.Menus {
		for _, it := range m.Items {
			if strings.EqualFold(it.Submenu, name) {
				return true
			}
		}
	}
	return false
}

func (a *App) editMenuItem(m *Menu, idx int) {
	it := m.Items[idx]
	a.menuItemDialog(it, it.Submenu != "", func(edited MenuItem) {
		if it.Submenu != "" {
			edited.Submenu = it.Submenu
			if sub := a.cfg.menu(it.Submenu); sub != nil && edited.Title != it.Title {
				sub.Name = a.renameSubmenu(sub.Name, edited.Title)
				edited.Submenu = sub.Name
			}
		}
		m.Items[idx] = edited
		a.menusChanged(m, idx)
	})
}

// renameSubmenu renames a menu (keeping the name unique) and updates every
// item that points at it.
func (a *App) renameSubmenu(old, title string) string {
	name := strings.TrimSpace(title)
	if name == "" || strings.EqualFold(name, old) {
		return old
	}
	for n := 2; a.cfg.menu(name) != nil; n++ {
		name = fmt.Sprintf("%s %d", strings.TrimSpace(title), n)
	}
	for i := range a.cfg.Menus {
		for j := range a.cfg.Menus[i].Items {
			if strings.EqualFold(a.cfg.Menus[i].Items[j].Submenu, old) {
				a.cfg.Menus[i].Items[j].Submenu = name
			}
		}
	}
	return name
}

// editMenusFile opens menus.toml in the editor and reloads the menus when
// the editor closes.
func (a *App) editMenusFile(m *Menu) {
	if err := a.saveMenus(); err != nil {
		a.setErr(err)
		return
	}
	name := m.Name
	if _, ok := a.top().(*ListDialog); ok {
		a.pop()
	}
	a.editFileThen(a.menusFile(), false, func() {
		menus, err := loadMenusFile(a.menusFile())
		if err != nil {
			a.setErr(err)
			return
		}
		a.cfg.Menus = menus
		a.keymap = buildKeymap(a.cfg)
		if again := a.cfg.menu(name); again != nil {
			a.openMenu(again)
		}
	})
}

// ---------------------------------------------------------------- edit dialog

// MenuItemDialog is FAR's "Edit user menu" dialog: hot key, label and, for
// commands, the command lines.
type MenuItemDialog struct {
	hot   *LineEdit
	label *LineEdit
	cmds  *TextArea // nil for a submenu
	focus int       // 0 hot key, 1 label, 2 commands, 3 OK, 4 Cancel
	OnOK  func(MenuItem)
}

func (a *App) menuItemDialog(it MenuItem, submenu bool, onOK func(MenuItem)) {
	hot := it.Hotkey
	if it.Title == "-" || it.Title == "---" {
		hot = menuSeparatorHotkey
	}
	d := &MenuItemDialog{hot: newLineEdit(hot), label: newLineEdit(it.Title), OnOK: onOK}
	if !submenu {
		d.cmds = newTextArea(it.Command)
	}
	a.push(d)
}

func (d *MenuItemDialog) count() int {
	if d.cmds == nil {
		return 4
	}
	return 5
}

// index maps a logical field (0 hot, 1 label, 2 commands, 3 OK, 4 Cancel)
// to the focus number, skipping the commands box of a submenu.
func (d *MenuItemDialog) field() int {
	if d.cmds == nil && d.focus >= 2 {
		return d.focus + 1
	}
	return d.focus
}

func (d *MenuItemDialog) accept(a *App) {
	it := MenuItem{Title: strings.TrimSpace(d.label.String()), Hotkey: strings.TrimSpace(d.hot.String())}
	if d.cmds != nil {
		it.Command = strings.TrimRight(d.cmds.String(), "\n")
	}
	if it.Hotkey == menuSeparatorHotkey {
		it = MenuItem{Title: "-"}
	} else if it.Title == "" {
		a.setErr(fmt.Errorf("the label must not be empty"))
		return
	}
	a.pop()
	if d.OnOK != nil {
		d.OnOK(it)
	}
}

func (d *MenuItemDialog) HandleKey(a *App, ev *tcell.EventKey) {
	name := keyName(ev)
	switch name {
	case "Esc":
		a.pop()
		return
	case "Tab":
		d.focus = (d.focus + 1) % d.count()
		return
	case "Shift+Tab":
		d.focus = (d.focus + d.count() - 1) % d.count()
		return
	case "Ctrl+Enter", "Ctrl+J":
		d.accept(a)
		return
	case "Enter":
		switch d.field() {
		case 2:
			d.cmds.HandleKey(ev, a)
		case 4:
			a.pop()
		default:
			d.accept(a)
		}
		return
	}
	if ev.Key() == tcell.KeyRune && ev.Modifiers()&tcell.ModAlt != 0 {
		switch ev.Rune() {
		case 'h', 'H':
			d.focus = 0
			return
		case 'l', 'L':
			d.focus = 1
			return
		case 'c', 'C':
			if d.cmds != nil {
				d.focus = 2
			}
			return
		}
	}
	switch d.field() {
	case 0:
		if ev.Key() == tcell.KeyRune && ev.Modifiers() == 0 && len(d.hot.Text) >= 2 && d.hot.Anchor < 0 {
			return // a hot key is one character, or "--" for a separator
		}
		d.hot.HandleKey(ev, a)
	case 1:
		d.label.HandleKey(ev, a)
	case 2:
		d.cmds.HandleKey(ev, a)
	default:
		switch name {
		case "Left", "Right", "Up", "Down":
			if d.field() == 3 {
				d.focus++
			} else {
				d.focus--
			}
		case "Space":
			if d.field() == 4 {
				a.pop()
			} else {
				d.accept(a)
			}
		}
	}
}

func (d *MenuItemDialog) Draw(a *App) {
	th := a.theme
	w := clampInt(a.w-8, 50, 100)
	h := clampInt(a.h-6, 12, 24)
	if d.cmds == nil {
		h = 9
	}
	x, y := a.center(w, h)
	a.drawBox(x, y, w, h, th.Dialog, "Edit user menu")
	inner := w - 4
	field := func(row, fw int, e *LineEdit, focused bool) {
		st := th.Input
		if focused {
			st = th.InputFocus
		}
		e.Draw(a, x+2, row, fw, st, focused)
	}
	a.printHot(x+2, y+1, inner, th.Dialog, th.Hotkey, "Hot key:", 0)
	field(y+2, 4, d.hot, d.field() == 0)
	a.printHot(x+2, y+3, inner, th.Dialog, th.Hotkey, "Label:", 0)
	field(y+4, inner, d.label, d.field() == 1)
	a.separator(x, y+5, w, th.Dialog)
	brow := y + h - 2
	if d.cmds != nil {
		a.printHot(x+2, y+6, inner, th.Dialog, th.Hotkey, "Commands:", 0)
		st := th.Input
		if d.field() == 2 {
			st = th.InputFocus
		}
		d.cmds.Draw(a, x+2, y+7, inner, brow-2-(y+7), st, d.field() == 2)
		a.separator(x, brow-1, w, th.Dialog)
	}
	ok := "[ OK ]"
	if d.field() != 4 {
		ok = "{ OK }"
	}
	cancel := "[ Cancel ]"
	bw := runewidth.StringWidth(ok) + 1 + runewidth.StringWidth(cancel)
	bx := x + (w-bw)/2
	okSt, cSt := th.Button, th.Button
	if d.field() == 3 {
		okSt = th.ButtonActive
	}
	if d.field() == 4 {
		cSt = th.ButtonActive
	}
	a.print(bx, brow, w, okSt, ok)
	a.print(bx+runewidth.StringWidth(ok)+1, brow, w, cSt, cancel)
}

// ---------------------------------------------------------------- running

// usesFileSymbols reports whether a command refers to a single file, so
// that it must be run once per selected entry: FAR's !, !.!, !~, !` or
// near's {file} {path} {name} {ext}.
func usesFileSymbols(cmd string) bool {
	for _, p := range []string{"{file}", "{path}", "{name}", "{ext}"} {
		if strings.Contains(cmd, p) {
			return true
		}
	}
	for i := 0; i < len(cmd); i++ {
		if cmd[i] != '!' {
			continue
		}
		if i+1 >= len(cmd) {
			return true
		}
		switch cmd[i+1] {
		case '!':
			i++ // a literal !
		case '?':
			// a prompt: skip to its closing !
			if end := strings.IndexByte(cmd[i+2:], '!'); end >= 0 {
				i += 2 + end
			} else {
				return false
			}
		case '@', '$', '&', ':', '\\', '/', '#', '^', '[', ']', '=':
			// whole-selection, directory and panel symbols
		default:
			return true
		}
	}
	return false
}

// commandSeparator joins several commands into one shell line.
func commandSeparator() string {
	if runtime.GOOS == "windows" {
		return " & "
	}
	return " ; "
}

// menuCommandLine expands a menu item's command lines for the targets: once
// per selected entry when the command names a file, once otherwise. The
// lines are joined into one shell command.
func (a *App) menuCommandLine(it MenuItem, targets []Entry, listFile string) string {
	t := a.tab()
	var all []string
	for _, e := range targets {
		all = append(all, shQuote(e.Name))
	}
	runs := []Entry{}
	if cur := t.Current(); cur != nil && !cur.IsUp {
		runs = append(runs, *cur)
	}
	if usesFileSymbols(it.Command) && len(targets) > 1 {
		runs = targets
	}
	if len(runs) == 0 {
		runs = []Entry{{}}
	}
	var cmds []string
	for _, e := range runs {
		for _, line := range strings.Split(it.Command, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			cmds = append(cmds, a.expandFor(expandApply(line, t.Path, e, listFile, strings.Join(all, " ")), &e))
		}
	}
	return strings.Join(cmds, commandSeparator())
}

// runMenuItem runs a command item: prompts first, then one shell run for
// the whole selection.
func (a *App) runMenuItem(it MenuItem) {
	a.askPrompts(it.Command, it.Title, func(cmd string) {
		it.Command = cmd
		t := a.tab()
		targets := t.Targets()
		listFile := ""
		if strings.Contains(cmd, "!@!") {
			f, err := os.CreateTemp("", "near-list-*.txt")
			if err != nil {
				a.setErr(err)
				return
			}
			for _, e := range targets {
				fmt.Fprintln(f, filepath.Join(t.Path, e.Name))
			}
			f.Close()
			listFile = f.Name()
		}
		line := a.menuCommandLine(it, targets, listFile)
		run := func() {
			cwd := t.Path
			if it.Cwd != "" {
				cwd = a.expandPlaceholders(it.Cwd)
			}
			a.runShell(line, cwd, !it.NoWait, it.Detach)
		}
		if it.Confirm {
			a.confirm(it.Title, []string{fitLeft(line, a.w-10)}, []string{"Run", "Cancel"}, func(r int) {
				if r == 0 {
					run()
				}
			})
			return
		}
		run()
	})
}
