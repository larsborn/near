package main

import (
	"time"

	"github.com/gdamore/tcell/v2"
)

// doubleClick is the longest gap between two clicks counted as one.
const doubleClick = 400 * time.Millisecond

// panelAt returns the panel and entry index under a screen position, or
// -1 when the position is not on a panel's entry rows.
func (a *App) panelAt(x, y int) (panel, entry int) {
	for i, r := range a.panelRect {
		if r.w == 0 || !a.panelVisible(i) || x < r.x || x >= r.x+r.w || y < r.y || y >= r.y+r.h {
			continue
		}
		row := y - (r.y + 3) // rows above the entries: border, path, header
		t := a.panels[i].Tab()
		if row < 0 || row >= a.listRows(i) || t.Top+row >= len(t.Entries) {
			return i, -1
		}
		return i, t.Top + row
	}
	return -1, -1
}

// handleMouse: left click moves the cursor (and the active panel), a
// double click opens the entry, right click opens its context menu, the
// wheel scrolls. Dialogs are keyboard-only, so clicks are ignored there.
func (a *App) handleMouse(ev *tcell.EventMouse) {
	buttons := ev.Buttons()
	pressed := buttons &^ a.lastButtons // newly pressed buttons only
	a.lastButtons = buttons &^ (tcell.WheelUp | tcell.WheelDown | tcell.WheelLeft | tcell.WheelRight)
	x, y := ev.Position()
	if len(a.dialogs) > 0 {
		// Dialogs get presses (not drags or releases) and wheel motion.
		if md, ok := a.top().(mouseDialog); ok && (pressed != 0 || buttons&(tcell.WheelUp|tcell.WheelDown) != 0) {
			md.HandleMouse(a, tcell.NewEventMouse(x, y, pressed|buttons&(tcell.WheelUp|tcell.WheelDown), ev.Modifiers()))
		}
		return
	}
	if a.consoleMouse(ev, pressed, buttons) {
		return
	}
	if pressed != 0 {
		a.sel = conSelection{} // a click elsewhere drops the marking
	}
	if a.hideAll {
		return
	}
	panel, entry := a.panelAt(x, y)
	if panel < 0 {
		return
	}
	t := a.panels[panel].Tab()
	switch {
	case buttons&tcell.WheelUp != 0:
		t.MoveCursor(-3)
	case buttons&tcell.WheelDown != 0:
		t.MoveCursor(3)
	case pressed&tcell.ButtonPrimary != 0:
		a.active = panel
		a.qsActive = false
		if entry >= 0 {
			t.Cursor = entry
			now := time.Now()
			if now.Sub(a.lastClick) < doubleClick && a.lastClickAt == [2]int{panel, entry} {
				a.lastClick = time.Time{}
				a.enterCurrent()
				return
			}
			a.lastClick, a.lastClickAt = now, [2]int{panel, entry}
		}
	case pressed&tcell.ButtonSecondary != 0:
		a.active = panel
		if entry >= 0 {
			t.Cursor = entry
			a.showContextMenu()
		}
	}
}
