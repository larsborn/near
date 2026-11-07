package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/gdamore/tcell/v2"
)

// Folder shortcuts: a short list of directories opened with Ctrl+D, each
// numbered so one keystroke goes there. Ins adds the panel's directory,
// Del removes the one under the cursor. Edits are kept in shortcuts.toml
// next to the config; once that file exists the [[bookmarks]] of
// config.toml are ignored, as with menus.toml.

const shortcutsFileName = "shortcuts.toml"

func (a *App) shortcutsFile() string { return filepath.Join(a.cfg.dir, shortcutsFileName) }

// loadShortcutsFile reads shortcuts.toml; a missing file is reported as such.
func loadShortcutsFile(path string) ([]Bookmark, error) {
	var doc struct {
		Bookmarks []Bookmark `toml:"bookmarks"`
	}
	if _, err := toml.DecodeFile(path, &doc); err != nil {
		return nil, err
	}
	return doc.Bookmarks, nil
}

// saveShortcuts writes the current list to shortcuts.toml.
func (a *App) saveShortcuts() error {
	var buf bytes.Buffer
	buf.WriteString("# Folder shortcuts edited inside NEAR (Ctrl+D, then Ins / Del).\n")
	buf.WriteString("# While this file exists the [[bookmarks]] of config.toml are ignored.\n\n")
	doc := struct {
		Bookmarks []Bookmark `toml:"bookmarks"`
	}{a.cfg.Bookmarks}
	if err := toml.NewEncoder(&buf).Encode(doc); err != nil {
		return err
	}
	if err := os.MkdirAll(a.cfg.dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(a.shortcutsFile(), buf.Bytes(), 0o644)
}

// shortcutLabel names a shortcut after its directory; a drive or file
// system root has no base name worth showing, so it keeps the path.
func shortcutLabel(path string) string {
	base := filepath.Base(path)
	if base == "" || base == "." || base == string(filepath.Separator) || strings.HasSuffix(path, `:\`) {
		return path
	}
	return base
}

// showShortcuts opens the list, with the cursor on entry at.
func (a *App) showShortcuts(at int) {
	var items []ListItem
	for i, b := range a.cfg.Bookmarks {
		var hk rune
		if i < 9 {
			hk = rune('1' + i)
		}
		items = append(items, ListItem{Label: b.Name, Hotkey: hk, Right: fitRight(expandPath(b.Path), 44), Data: i})
	}
	if len(items) == 0 {
		items = append(items, ListItem{Label: "No shortcuts yet", Dim: true})
	}
	d := a.list("Folder shortcuts", items, func(_ int, li ListItem) {
		i, ok := li.Data.(int)
		if !ok || i >= len(a.cfg.Bookmarks) {
			return
		}
		if err := a.navigate(a.tab(), expandPath(a.cfg.Bookmarks[i].Path), ""); err != nil {
			a.setErr(err)
		}
	})
	d.Menu = true
	d.Width = clampInt(a.w-10, 44, 80)
	d.Hint = "Enter or 1-9: go there   Ins: add this directory   Del: remove"
	d.Cursor = clampInt(at, 0, maxInt(0, len(items)-1))
	d.OnKey = func(a *App, d *ListDialog, ev *tcell.EventKey) bool {
		switch keyName(ev) {
		case "Insert":
			a.addShortcut(a.tab().Path)
			return true
		case "Delete":
			if i, ok := d.Items[d.Cursor].Data.(int); ok {
				a.removeShortcut(i)
			}
			return true
		}
		return false
	}
}

// addShortcut remembers a directory, or moves to it if it is already there.
func (a *App) addShortcut(path string) {
	for i, b := range a.cfg.Bookmarks {
		if foldPath(expandPath(b.Path)) == foldPath(path) {
			a.reopenShortcuts(i, fmt.Sprintf("%s is already a shortcut", shortcutLabel(path)))
			return
		}
	}
	a.cfg.Bookmarks = append(a.cfg.Bookmarks, Bookmark{Name: shortcutLabel(path), Path: path})
	a.reopenShortcuts(len(a.cfg.Bookmarks)-1, "Added "+path)
}

// removeShortcut drops one entry.
func (a *App) removeShortcut(i int) {
	if i < 0 || i >= len(a.cfg.Bookmarks) {
		return
	}
	gone := a.cfg.Bookmarks[i].Name
	a.cfg.Bookmarks = append(a.cfg.Bookmarks[:i], a.cfg.Bookmarks[i+1:]...)
	a.reopenShortcuts(clampInt(i, 0, maxInt(0, len(a.cfg.Bookmarks)-1)), "Removed "+gone)
}

// reopenShortcuts saves the list and draws it again with the cursor on at.
func (a *App) reopenShortcuts(at int, note string) {
	if err := a.saveShortcuts(); err != nil {
		a.setErr(err)
	}
	if len(a.dialogs) > 0 {
		if _, ok := a.top().(*ListDialog); ok {
			a.pop()
		}
	}
	a.showShortcuts(at)
	if d, ok := a.top().(*ListDialog); ok && note != "" {
		d.Hint = note
	}
}
