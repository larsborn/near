package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// Existing-files policies of the copy/move dialog, in FAR's wording.
var existingPolicies = []string{"Ask", "Overwrite", "Skip", "Only newer"}

// copyOptions are the choices made in the copy/move dialog.
type copyOptions struct {
	Policy     string // one of existingPolicies
	KeepTimes  bool   // preserve modification times
	Deref      bool   // copy the contents of symbolic links instead of the links
	Background bool   // queue in the background instead of a modal progress box
}

// CopyMoveDialog is FAR's Copy / Rename-Move dialog.
type CopyMoveDialog struct {
	move    bool
	targets []Entry
	dest    *LineEdit
	policy  int
	keep    bool
	deref   bool
	focus   int // 0 destination, 1 policy, 2 keep times, 3 symlinks, 4 OK, 5 Background, 6 Cancel
	OnOK    func(dest string, opts copyOptions)
}

const copyFocusCount = 7

func (a *App) copyMoveDialog(move bool, targets []Entry, dest string, onOK func(string, copyOptions)) {
	d := &CopyMoveDialog{move: move, targets: targets, dest: newLineEdit(dest), keep: true, OnOK: onOK}
	a.push(d)
}

func (d *CopyMoveDialog) title() string {
	if d.move {
		return "Rename/Move"
	}
	return "Copy"
}

func (d *CopyMoveDialog) question(width int) string {
	verb := "Copy"
	if d.move {
		verb = "Rename or move"
	}
	if len(d.targets) == 1 {
		room := width - runewidth.StringWidth(verb) - len(" to") - 2
		return fmt.Sprintf("%s %s to", verb, fitLeft(d.targets[0].Name, room))
	}
	files, dirs := 0, 0
	for _, e := range d.targets {
		if e.IsDir {
			dirs++
		} else {
			files++
		}
	}
	var what []string
	if files > 0 {
		what = append(what, plural(files, "file"))
	}
	if dirs > 0 {
		what = append(what, plural(dirs, "folder"))
	}
	return fmt.Sprintf("%s %s to", verb, strings.Join(what, " and "))
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func (d *CopyMoveDialog) okLabel() string {
	if d.move {
		return "Rename"
	}
	return "Copy"
}

func (d *CopyMoveDialog) accept(a *App, background bool) {
	dest := strings.TrimSpace(d.dest.String())
	if dest == "" {
		return
	}
	a.pop()
	if d.OnOK != nil {
		d.OnOK(dest, copyOptions{Policy: existingPolicies[d.policy], KeepTimes: d.keep, Deref: d.deref, Background: background})
	}
}

func (d *CopyMoveDialog) cyclePolicy(delta int) {
	d.policy = (d.policy + delta + len(existingPolicies)) % len(existingPolicies)
}

func (d *CopyMoveDialog) HandleKey(a *App, ev *tcell.EventKey) {
	name := keyName(ev)
	switch name {
	case "Esc":
		a.pop()
		return
	case "Tab", "Down":
		d.focus = (d.focus + 1) % copyFocusCount
		return
	case "Shift+Tab", "Up":
		d.focus = (d.focus + copyFocusCount - 1) % copyFocusCount
		return
	case "Enter":
		switch d.focus {
		case 6:
			a.pop()
		case 5:
			d.accept(a, true)
		default:
			d.accept(a, false)
		}
		return
	}
	// Hotkeys (FAR highlights one letter per control); Alt+letter works
	// everywhere, the bare letter outside the text field.
	if ev.Key() == tcell.KeyRune && (ev.Modifiers()&tcell.ModAlt != 0 || d.focus != 0) {
		switch ev.Rune() {
		case 'x', 'X':
			d.focus = 1
			d.cyclePolicy(1)
			return
		case 'p', 'P':
			d.focus = 2
			d.keep = !d.keep
			return
		case 'l', 'L':
			d.focus = 3
			d.deref = !d.deref
			return
		case 'b', 'B':
			d.accept(a, true)
			return
		case 'c', 'C':
			if ev.Modifiers()&tcell.ModAlt != 0 || d.focus >= 4 {
				a.pop()
				return
			}
		}
	}
	switch d.focus {
	case 0:
		d.dest.HandleKey(ev, a)
	case 1:
		switch name {
		case "Left":
			d.cyclePolicy(-1)
		case "Right", "Space":
			d.cyclePolicy(1)
		}
	case 2:
		if name == "Space" {
			d.keep = !d.keep
		}
	case 3:
		if name == "Space" {
			d.deref = !d.deref
		}
	default:
		switch name {
		case "Left":
			d.focus = clampInt(d.focus-1, 4, 6)
		case "Right":
			d.focus = clampInt(d.focus+1, 4, 6)
		case "Space":
			switch d.focus {
			case 6:
				a.pop()
			case 5:
				d.accept(a, true)
			default:
				d.accept(a, false)
			}
		}
	}
}

// printHot prints text with the character at hot highlighted.
func (a *App) printHot(x, y, w int, st, hot tcell.Style, text string, hotIdx int) {
	cx := x
	for i, r := range []rune(text) {
		s := st
		if i == hotIdx {
			s = hot
		}
		if cx-x >= w {
			break
		}
		cx += a.print(cx, y, w-(cx-x), s, string(r))
	}
}

func (d *CopyMoveDialog) Draw(a *App) {
	th := a.theme
	w := clampInt(a.w-10, 50, 76)
	h := 10
	x, y := a.center(w, h)
	a.drawBox(x, y, w, h, th.Dialog, d.title())
	inner := w - 4
	fieldW := inner - 1 // leave room for the ↓ marker

	a.print(x+2, y+1, inner, th.Dialog, d.question(inner))
	st := th.Input
	if d.focus == 0 {
		st = th.InputFocus
	}
	d.dest.Draw(a, x+2, y+2, fieldW, st, d.focus == 0)
	a.print(x+2+fieldW, y+2, 1, th.Dialog, "↓")
	a.separator(x, y+3, w, th.Dialog)

	label := "Already existing files: "
	a.printHot(x+2, y+4, inner, th.Dialog, th.Hotkey, label, 9) // x
	fx := x + 2 + len(label)
	fw := x + 2 + fieldW - fx
	pst := th.Input
	if d.focus == 1 {
		pst = th.InputFocus
	}
	a.print(fx, y+4, fw, pst, padRight(existingPolicies[d.policy], fw))
	a.print(fx+fw, y+4, 1, th.Dialog, "↓")

	check := func(row int, on bool, focused bool, text string, hot int) {
		box := "[ ] "
		if on {
			box = "[x] "
		}
		bst := th.Dialog
		if focused {
			bst = th.ButtonActive
		}
		a.print(x+2, row, 4, bst, box)
		a.printHot(x+6, row, inner-4, th.Dialog, th.Hotkey, text, hot)
	}
	check(y+5, d.keep, d.focus == 2, "Preserve timestamps", 0)
	check(y+6, d.deref, d.focus == 3, "Copy contents of symbolic links", 17)

	a.separator(x, y+7, w, th.Dialog)
	ok := "[ " + d.okLabel() + " ]"
	if d.focus < 5 {
		ok = "{ " + d.okLabel() + " }" // the default button, FAR style
	}
	bg := "[ Background ]"
	cancel := "[ Cancel ]"
	bw := runewidth.StringWidth(ok) + 1 + runewidth.StringWidth(bg) + 1 + runewidth.StringWidth(cancel)
	bx := x + (w-bw)/2
	okSt, bSt, cSt := th.Button, th.Button, th.Button
	switch d.focus {
	case 4:
		okSt = th.ButtonActive
	case 5:
		bSt = th.ButtonActive
	case 6:
		cSt = th.ButtonActive
	}
	a.printHot(bx, y+8, w, okSt, th.Hotkey.Background(bgOf(okSt)), ok, 2)
	bx += runewidth.StringWidth(ok) + 1
	a.printHot(bx, y+8, w, bSt, th.Hotkey.Background(bgOf(bSt)), bg, 2)
	bx += runewidth.StringWidth(bg) + 1
	a.printHot(bx, y+8, w, cSt, th.Hotkey.Background(bgOf(cSt)), cancel, 2)
}
