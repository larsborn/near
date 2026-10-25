package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// ctxMenu is a context menu for a selection: Explorer's shell menu on
// Windows, a desktop-application menu elsewhere, plus NEAR's own actions.
type ctxMenu interface {
	Entries() []menuEntry
	Expand(e menuEntry) []menuEntry
	Invoke(e menuEntry) error
	Close()
}

// menuEntry is one line of a (sub)menu.
type menuEntry struct {
	text    string
	hotkey  rune
	sep     bool
	dim     bool
	hasSub  bool
	id      uint32  // Windows: menu command id
	submenu uintptr // Windows: HMENU of the submenu
	index   int     // Windows: position in the parent menu
	run     func() error
}

// showContextMenu opens the context menu for the current entry (or the
// selection) of the active panel.
func (a *App) showContextMenu() {
	t := a.tab()
	cur := t.Current()
	if cur == nil || cur.IsUp {
		return
	}
	targets := t.Targets()
	var names []string
	for _, e := range targets {
		names = append(names, e.Name)
	}
	m, err := openShellMenu(t.Path, names)
	if err != nil {
		a.setErr(fmt.Errorf("context menu: %v", err))
		return
	}
	title := names[0]
	if len(names) > 1 {
		title = fmt.Sprintf("%d entries", len(names))
	}
	entries := append(m.Entries(), a.nearMenuEntries(targets)...)
	a.showMenuLevel(m, title, entries, nil)
}

// nearMenuEntries are NEAR's own actions, appended after a separator.
func (a *App) nearMenuEntries(targets []Entry) []menuEntry {
	act := func(text string, hot rune, action string) menuEntry {
		return menuEntry{text: text, hotkey: hot, run: func() error { a.doAction(action); return nil }}
	}
	entries := []menuEntry{{sep: true}}
	if len(targets) == 1 && !targets[0].IsDir {
		entries = append(entries, act("View (NEAR)", 'V', "view"), act("Edit (NEAR)", 'E', "edit"))
	}
	return append(entries,
		act("Copy to other panel…", 'C', "copy"),
		act("Move to other panel…", 'M', "move"),
		act("Rename…", 'R', "rename"),
		act("Mass rename…", 'a', "mass_rename"),
		act("Copy path to clipboard", 'p', "copy_path"),
		act("Note…", 'N', "notes"),
		act("Apply command…", 'A', "apply_command"),
		act("Delete", 'D', "delete"),
	)
}

// showMenuLevel shows one level of the menu; a submenu replaces its parent
// and Esc goes back to it, Esc at the root closes the menu.
func (a *App) showMenuLevel(m ctxMenu, title string, entries []menuEntry, parent func()) {
	var items []ListItem
	for _, e := range entries {
		if e.sep {
			items = append(items, ListItem{Sep: true})
			continue
		}
		right := ""
		if e.hasSub {
			right = "▸"
		}
		items = append(items, ListItem{Label: e.text, Hotkey: e.hotkey, Right: right, Dim: e.dim, Data: e})
	}
	d := a.list(title, items, func(_ int, li ListItem) {
		e := li.Data.(menuEntry)
		if e.hasSub {
			back := func() { a.showMenuLevel(m, title, entries, parent) }
			a.showMenuLevel(m, e.text, m.Expand(e), back)
			return
		}
		// Invoke needs the shell objects: run first, release afterwards.
		var err error
		if e.run != nil {
			err = e.run()
		} else {
			err = m.Invoke(e)
		}
		m.Close()
		if err != nil {
			a.setErr(fmt.Errorf("%s: %v", e.text, err))
		}
		a.reloadAll()
	})
	d.Menu = true
	d.Width = clampInt(a.w-20, 36, 60)
	d.OnClose = func() {
		if parent != nil {
			parent()
		} else {
			m.Close()
		}
	}
	d.OnKey = func(a *App, d *ListDialog, ev *tcell.EventKey) bool {
		// Right opens a submenu, Left goes back, like in Windows menus.
		switch keyName(ev) {
		case "Right":
			if d.Cursor < len(d.Items) {
				if e, ok := d.Items[d.Cursor].Data.(menuEntry); ok && e.hasSub {
					d.select_(a)
					return true
				}
			}
		case "Left":
			if parent != nil {
				a.pop()
				parent()
				return true
			}
		}
		return false
	}
}

// contextMenuTitle is used by the non-Windows menus for a sensible caption.
func contextMenuTitle(dir string, names []string) string {
	if len(names) == 1 {
		return filepath.Join(dir, names[0])
	}
	return fmt.Sprintf("%d entries in %s", len(names), dir)
}

// menuLabel strips the & mnemonic and a tab accelerator from a menu string.
func menuLabel(s string) (string, rune) {
	if i := strings.Index(s, "\t"); i >= 0 {
		s = s[:i]
	}
	var hot rune
	var b strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '&' && i+1 < len(rs) {
			if rs[i+1] == '&' {
				b.WriteRune('&')
				i++
				continue
			}
			if hot == 0 {
				hot = rs[i+1]
			}
			continue
		}
		b.WriteRune(rs[i])
	}
	return strings.TrimSpace(b.String()), hot
}
