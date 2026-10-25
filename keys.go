package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// keyName turns a tcell key event into a canonical name such as "F5",
// "Ctrl+F", "Alt+Enter", "Shift+F6" or "a". Canonical names are what the
// [keys] section of the config is normalised to, see normKey.
func keyName(ev *tcell.EventKey) string {
	mods := ev.Modifiers()
	var name string
	k := ev.Key()
	switch {
	case k == tcell.KeyRune:
		r := ev.Rune()
		if r == ' ' {
			name = "Space"
		} else {
			name = string(r)
		}
		mods &^= tcell.ModShift
	case k == tcell.KeyEnter:
		name = "Enter"
	case k == tcell.KeyEscape:
		name = "Esc"
	case k == tcell.KeyTab:
		name = "Tab"
	case k == tcell.KeyBacktab:
		name = "Tab"
		mods |= tcell.ModShift
	case k == tcell.KeyBackspace || k == tcell.KeyBackspace2:
		name = "Backspace"
	case k == tcell.KeyDelete:
		name = "Delete"
	case k == tcell.KeyInsert:
		name = "Insert"
	case k == tcell.KeyUp:
		name = "Up"
	case k == tcell.KeyDown:
		name = "Down"
	case k == tcell.KeyLeft:
		name = "Left"
	case k == tcell.KeyRight:
		name = "Right"
	case k == tcell.KeyHome:
		name = "Home"
	case k == tcell.KeyEnd:
		name = "End"
	case k == tcell.KeyPgUp:
		name = "PgUp"
	case k == tcell.KeyPgDn:
		name = "PgDn"
	case k >= tcell.KeyF1 && k <= tcell.KeyF64:
		name = fmt.Sprintf("F%d", int(k-tcell.KeyF1)+1)
	case k >= tcell.KeyCtrlA && k <= tcell.KeyCtrlZ:
		name = string(rune('A' + int(k-tcell.KeyCtrlA)))
		mods |= tcell.ModCtrl
	case k == tcell.KeyCtrlSpace:
		name = "Space"
		mods |= tcell.ModCtrl
	default:
		name = tcell.KeyNames[k]
		if name == "" {
			name = fmt.Sprintf("Key%d", int(k))
		}
	}
	return joinKey(mods&tcell.ModCtrl != 0, mods&(tcell.ModAlt|tcell.ModMeta) != 0, mods&tcell.ModShift != 0, name)
}

func joinKey(ctrl, alt, shift bool, name string) string {
	var b strings.Builder
	if ctrl {
		b.WriteString("Ctrl+")
	}
	if alt {
		b.WriteString("Alt+")
	}
	if shift {
		b.WriteString("Shift+")
	}
	if b.Len() > 0 && len([]rune(name)) == 1 {
		name = strings.ToUpper(name)
	}
	b.WriteString(name)
	return b.String()
}

// normKey normalises a key description from the config file into the
// canonical form produced by keyName. It accepts common aliases
// ("Escape", "Return", "PageUp", "Del", "control+x", ...).
func normKey(s string) string {
	s = strings.TrimSpace(s)
	if s == "+" || s == "" {
		return s
	}
	parts := strings.Split(s, "+")
	// "Ctrl++" splits into ["Ctrl", "", ""]: the key is a literal plus.
	name := parts[len(parts)-1]
	if name == "" && len(parts) >= 2 {
		name = "+"
		parts = parts[:len(parts)-1]
	}
	var ctrl, alt, shift bool
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "ctrl", "control", "c":
			ctrl = true
		case "alt", "meta", "m", "option":
			alt = true
		case "shift", "s":
			shift = true
		}
	}
	name = strings.TrimSpace(name)
	lower := strings.ToLower(name)
	switch lower {
	case "esc", "escape":
		name = "Esc"
	case "return", "enter", "cr":
		name = "Enter"
	case "space", "spc":
		name = "Space"
	case "bs", "backspace":
		name = "Backspace"
	case "del", "delete":
		name = "Delete"
	case "ins", "insert":
		name = "Insert"
	case "pgup", "pageup", "prior":
		name = "PgUp"
	case "pgdn", "pagedown", "next":
		name = "PgDn"
	case "up", "down", "left", "right", "home", "end", "tab":
		name = strings.ToUpper(lower[:1]) + lower[1:]
	case "backtab":
		name = "Tab"
		shift = true
	default:
		if len(lower) > 1 && lower[0] == 'f' && allDigits(lower[1:]) {
			name = "F" + lower[1:]
		}
	}
	return joinKey(ctrl, alt, shift, name)
}

func allDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

// actionHelp documents every built-in action, in display order.
var actionHelp = []struct{ Action, Help string }{
	{"help", "Show this help"},
	{"quit", "Quit"},
	{"switch_panel", "Switch active panel"},
	{"up", "Cursor up"},
	{"down", "Cursor down"},
	{"page_up", "Page up"},
	{"page_down", "Page down"},
	{"home", "First entry"},
	{"end", "Last entry"},
	{"enter", "Enter directory / open file"},
	{"up_dir", "Parent directory"},
	{"select", "Toggle selection of current entry"},
	{"select_down", "Toggle selection and move down"},
	{"select_up", "Toggle selection and move up"},
	{"select_to_home", "Select (or unselect) from the cursor to the first entry"},
	{"select_to_end", "Select (or unselect) from the cursor to the last entry"},
	{"select_page_up", "Select (or unselect) a page upwards"},
	{"select_page_down", "Select (or unselect) a page downwards"},
	{"select_pattern", "Select by pattern"},
	{"unselect_pattern", "Unselect by pattern"},
	{"invert_selection", "Invert selection"},
	{"select_all", "Select all"},
	{"unselect_all", "Unselect all"},
	{"new_tab", "New tab (copy of current)"},
	{"close_tab", "Close tab"},
	{"next_tab", "Next tab"},
	{"prev_tab", "Previous tab"},
	{"tab_1", "Go to tab 1 (tab_2 .. tab_9 likewise)"},
	{"view", "View file"},
	{"edit", "Edit file (built-in editor, F1 inside lists its keys)"},
	{"edit_new", "Edit a new file"},
	{"edit_external", "Edit file in the external editor"},
	{"copy_path", "Copy the full path of the current entry (or the directory) to the clipboard"},
	{"copy", "Copy to other panel"},
	{"move", "Move to other panel"},
	{"rename", "Rename current entry"},
	{"mass_rename", "Mass rename selected entries"},
	{"mkdir", "Create directory"},
	{"delete", "Move selected entries to the Recycle Bin / trash"},
	{"delete_permanent", "Delete selected entries permanently"},
	{"quick_launch", "Quick launch menu"},
	{"menus", "List all custom menus"},
	{"notes", "Edit note for current entry"},
	{"search", "Search all indexed file names"},
	{"quick_search", "Quick search in current directory"},
	{"sort", "Choose sort order"},
	{"toggle_hidden", "Show / hide hidden files"},
	{"refresh", "Reload both panels"},
	{"swap_panels", "Swap left and right panel"},
	{"toggle_panels", "Hide / show both panels"},
	{"toggle_inactive_panel", "Hide / show the inactive panel"},
	{"toggle_left_panel", "Hide / show the left panel"},
	{"toggle_right_panel", "Hide / show the right panel"},
	{"panels_taller", "Both panels one row taller"},
	{"panels_shorter", "Both panels one row shorter"},
	{"panel_taller", "Current panel one row taller"},
	{"panel_shorter", "Current panel one row shorter"},
	{"split_left", "Move the panel divider left"},
	{"split_right", "Move the panel divider right"},
	{"panels_default_width", "Restore the default panel widths"},
	{"panels_default_height", "Restore the default panel heights"},
	{"sync_other", "Other panel: go to this directory"},
	{"locations", "Drives, mounts and bookmarks"},
	{"goto_path", "Go to path"},
	{"apply_command", "Apply a shell command to every selected entry (!.! ! !~ !\\ !@! …)"},
	{"run_command", "Run a shell command (dialog)"},
	{"insert_name", "Command line: insert current entry name"},
	{"insert_path", "Command line: insert current entry's full path"},
	{"history_prev", "Command line: previous history entry"},
	{"history_next", "Command line: next history entry"},
	{"history", "Command history (Enter runs, Shift+Enter edits, Delete removes)"},
	{"folder_history", "Folders history (Enter goes, Ctrl+T new tab, Delete removes)"},
	{"file_history", "View / edit history (Enter views, F4 edits, Ctrl+Enter goes to file)"},
	{"compare", "Compare directories (mark differences) or files side by side"},
	{"transfers", "Background transfers (Enter shows progress, Delete cancels)"},
}
