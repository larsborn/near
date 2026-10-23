package main

import (
	"os"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
)

// pushHistory appends item as the most recent entry, dropping an earlier
// occurrence and keeping at most max entries.
func pushHistory(h []string, item string, max int) []string {
	for i, x := range h {
		if x == item {
			h = append(h[:i], h[i+1:]...)
			break
		}
	}
	h = append(h, item)
	if len(h) > max {
		h = h[len(h)-max:]
	}
	return h
}

func removeHistory(h []string, item string) []string {
	out := h[:0]
	for _, x := range h {
		if x != item {
			out = append(out, x)
		}
	}
	return out
}

// historyItems lists a history most recent first.
func historyItems(h []string, right func(string) string) []ListItem {
	items := make([]ListItem, 0, len(h))
	for i := len(h) - 1; i >= 0; i-- {
		it := ListItem{Label: h[i], Data: h[i]}
		if right != nil {
			it.Right = right(h[i])
		}
		items = append(items, it)
	}
	return items
}

// deleteFromList removes the item under the cursor from a history dialog.
func deleteFromList(d *ListDialog, remove func(string)) {
	if d.Cursor < 0 || d.Cursor >= len(d.Items) {
		return
	}
	remove(d.Items[d.Cursor].Data.(string))
	d.Items = append(d.Items[:d.Cursor], d.Items[d.Cursor+1:]...)
	if d.Cursor >= len(d.Items) {
		d.Cursor = len(d.Items) - 1
	}
}

// ---------------------------------------------------------------- command history

func (a *App) showHistory() {
	if len(a.history) == 0 {
		a.setStatus("Command history is empty")
		return
	}
	d := a.list("Command history", historyItems(a.history, nil), func(_ int, li ListItem) {
		a.cmd.Set(li.Data.(string))
		a.runCommandLine()
	})
	d.Searchable = true
	d.Width = clampInt(a.w-10, 40, 100)
	d.Hint = "Enter: run   Shift+Enter / Insert: edit in command line   Delete: remove   Esc: close"
	d.OnKey = func(a *App, d *ListDialog, ev *tcell.EventKey) bool {
		switch keyName(ev) {
		case "Shift+Enter", "Ctrl+Enter", "Insert":
			if d.Cursor < len(d.Items) {
				a.pop()
				a.cmd.Set(d.Items[d.Cursor].Data.(string))
				a.histPos = len(a.history)
			}
			return true
		case "Delete":
			deleteFromList(d, func(s string) {
				a.history = removeHistory(a.history, s)
				a.histPos = len(a.history)
			})
			if len(d.Items) == 0 {
				a.pop()
			}
			return true
		}
		return false
	}
}

// ---------------------------------------------------------------- folders history

func (a *App) showDirHistory() {
	if len(a.dirHistory) == 0 {
		a.setStatus("Folders history is empty")
		return
	}
	items := historyItems(a.dirHistory, func(p string) string {
		if !isDir(p) {
			return "missing"
		}
		return ""
	})
	d := a.list("Folders history", items, func(_ int, li ListItem) {
		if err := a.navigate(a.tab(), li.Data.(string), ""); err != nil {
			a.setErr(err)
		}
	})
	d.Searchable = true
	d.Width = clampInt(a.w-10, 40, 100)
	d.Hint = "Enter: go   Ctrl+T: open in new tab   Shift+Enter: other panel   Delete: remove   Esc: close"
	d.OnKey = func(a *App, d *ListDialog, ev *tcell.EventKey) bool {
		if d.Cursor >= len(d.Items) {
			return false
		}
		path := d.Items[d.Cursor].Data.(string)
		switch keyName(ev) {
		case "Ctrl+T":
			a.pop()
			a.doAction("new_tab")
			if err := a.navigate(a.tab(), path, ""); err != nil {
				a.setErr(err)
			}
			return true
		case "Shift+Enter", "Ctrl+Enter":
			a.pop()
			if err := a.navigate(a.other(), path, ""); err != nil {
				a.setErr(err)
			}
			return true
		case "Delete":
			deleteFromList(d, func(s string) { a.dirHistory = removeHistory(a.dirHistory, s) })
			if len(d.Items) == 0 {
				a.pop()
			}
			return true
		}
		return false
	}
}

// ---------------------------------------------------------------- view / edit history

func (a *App) showFileHistory() {
	if len(a.fileHistory) == 0 {
		a.setStatus("View and edit history is empty")
		return
	}
	items := historyItems(a.fileHistory, func(p string) string {
		if _, err := os.Stat(p); err != nil {
			return "missing"
		}
		return ""
	})
	d := a.list("View and edit history", items, func(_ int, li ListItem) {
		a.viewFile(li.Data.(string))
	})
	d.Searchable = true
	d.Width = clampInt(a.w-10, 40, 100)
	d.Hint = "Enter: view   F4: edit   Ctrl+Enter: go to file   Delete: remove   Esc: close"
	d.OnKey = func(a *App, d *ListDialog, ev *tcell.EventKey) bool {
		if d.Cursor >= len(d.Items) {
			return false
		}
		path := d.Items[d.Cursor].Data.(string)
		switch keyName(ev) {
		case "F4":
			a.pop()
			a.editFile(path)
			return true
		case "Ctrl+Enter", "Shift+Enter":
			a.pop()
			if err := a.navigate(a.tab(), filepath.Dir(path), filepath.Base(path)); err != nil {
				a.setErr(err)
			}
			return true
		case "Delete":
			deleteFromList(d, func(s string) { a.fileHistory = removeHistory(a.fileHistory, s) })
			if len(d.Items) == 0 {
				a.pop()
			}
			return true
		}
		return false
	}
}
