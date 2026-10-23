package main

import "github.com/gdamore/tcell/v2"

// Theme holds every style used by the UI.
type Theme struct {
	Panel, Border, BorderActive     tcell.Style
	TabBar, Tab, TabActive          tcell.Style
	Path, Header                    tcell.Style
	Dir, File, Hidden, Link         tcell.Style
	Selected, Cursor, CursorSel     tcell.Style
	Info, Note                      tcell.Style
	Status, StatusErr               tcell.Style
	KeyNum, KeyLabel                tcell.Style
	Dialog, DialogTitle, DialogHint tcell.Style
	Input, InputFocus               tcell.Style
	Button, ButtonActive            tcell.Style
	ListCursor, Hotkey, ListDim     tcell.Style
	Conflict                        tcell.Style
	Viewer                          tcell.Style
	Cmd, CmdPrompt                  tcell.Style
	DiffAdd, DiffDel, DiffChange    tcell.Style
}

func themeByName(name string) *Theme {
	if name == "plain" {
		return plainTheme()
	}
	return farTheme()
}

func farTheme() *Theme {
	bg := tcell.ColorNavy
	panel := tcell.StyleDefault.Background(bg).Foreground(tcell.ColorSilver)
	dlg := tcell.StyleDefault.Background(tcell.ColorSilver).Foreground(tcell.ColorBlack)
	return &Theme{
		Panel:        panel,
		Border:       panel.Foreground(tcell.ColorAqua),
		BorderActive: panel.Foreground(tcell.ColorAqua),
		TabBar:       panel.Foreground(tcell.ColorAqua),
		Tab:          panel.Foreground(tcell.ColorSilver),
		TabActive:    tcell.StyleDefault.Background(tcell.ColorAqua).Foreground(tcell.ColorBlack),
		Path:         panel.Foreground(tcell.ColorWhite).Bold(true),
		Header:       panel.Foreground(tcell.ColorYellow),
		Dir:          panel.Foreground(tcell.ColorWhite),
		File:         panel.Foreground(tcell.ColorAqua),
		Hidden:       panel.Foreground(tcell.ColorTeal),
		Link:         panel.Foreground(tcell.ColorLime),
		Selected:     panel.Foreground(tcell.ColorYellow).Bold(true),
		Cursor:       tcell.StyleDefault.Background(tcell.ColorTeal).Foreground(tcell.ColorBlack),
		CursorSel:    tcell.StyleDefault.Background(tcell.ColorTeal).Foreground(tcell.ColorYellow).Bold(true),
		Info:         panel.Foreground(tcell.ColorSilver),
		Note:         panel.Foreground(tcell.ColorYellow),
		Status:       tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorSilver),
		StatusErr:    tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorRed).Bold(true),
		KeyNum:       tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorSilver),
		KeyLabel:     tcell.StyleDefault.Background(tcell.ColorTeal).Foreground(tcell.ColorBlack),
		Dialog:       dlg,
		DialogTitle:  dlg.Foreground(tcell.ColorNavy).Bold(true),
		DialogHint:   dlg.Foreground(tcell.ColorGray),
		Input:        tcell.StyleDefault.Background(tcell.ColorTeal).Foreground(tcell.ColorBlack),
		InputFocus:   tcell.StyleDefault.Background(tcell.ColorTeal).Foreground(tcell.ColorWhite).Bold(true),
		Button:       dlg,
		ButtonActive: tcell.StyleDefault.Background(tcell.ColorTeal).Foreground(tcell.ColorBlack).Bold(true),
		ListCursor:   tcell.StyleDefault.Background(tcell.ColorTeal).Foreground(tcell.ColorBlack),
		Hotkey:       dlg.Foreground(tcell.ColorYellow).Bold(true),
		ListDim:      dlg.Foreground(tcell.ColorGray),
		Conflict:     dlg.Foreground(tcell.ColorRed).Bold(true),
		Viewer:       tcell.StyleDefault.Background(bg).Foreground(tcell.ColorSilver),
		Cmd:          tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite),
		CmdPrompt:    tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorSilver).Bold(true),
		DiffAdd:      tcell.StyleDefault.Background(bg).Foreground(tcell.ColorLime),
		DiffDel:      tcell.StyleDefault.Background(bg).Foreground(tcell.ColorRed).Bold(true),
		DiffChange:   tcell.StyleDefault.Background(bg).Foreground(tcell.ColorYellow),
	}
}

func plainTheme() *Theme {
	d := tcell.StyleDefault
	return &Theme{
		Panel:        d,
		Border:       d,
		BorderActive: d.Bold(true),
		TabBar:       d,
		Tab:          d,
		TabActive:    d.Reverse(true),
		Path:         d.Bold(true),
		Header:       d.Underline(true),
		Dir:          d.Bold(true),
		File:         d,
		Hidden:       d.Dim(true),
		Link:         d.Italic(true),
		Selected:     d.Foreground(tcell.ColorYellow).Bold(true),
		Cursor:       d.Reverse(true),
		CursorSel:    d.Reverse(true).Foreground(tcell.ColorYellow).Bold(true),
		Info:         d,
		Note:         d.Foreground(tcell.ColorYellow),
		Status:       d,
		StatusErr:    d.Foreground(tcell.ColorRed).Bold(true),
		KeyNum:       d,
		KeyLabel:     d.Reverse(true),
		Dialog:       d,
		DialogTitle:  d.Bold(true),
		DialogHint:   d.Dim(true),
		Input:        d.Underline(true),
		InputFocus:   d.Underline(true).Bold(true),
		Button:       d,
		ButtonActive: d.Reverse(true),
		ListCursor:   d.Reverse(true),
		Hotkey:       d.Foreground(tcell.ColorYellow).Bold(true),
		ListDim:      d.Dim(true),
		Conflict:     d.Foreground(tcell.ColorRed).Bold(true),
		Viewer:       d,
		Cmd:          d,
		CmdPrompt:    d.Bold(true),
		DiffAdd:      d.Foreground(tcell.ColorGreen),
		DiffDel:      d.Foreground(tcell.ColorRed).Bold(true),
		DiffChange:   d.Foreground(tcell.ColorYellow),
	}
}
