package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

// Panel is one of the two sides, holding tabs.
type Panel struct {
	Tabs   []*Tab
	Active int
}

func (p *Panel) Tab() *Tab { return p.Tabs[p.Active] }

// App is the whole file manager.
type App struct {
	screen        tcell.Screen
	cfg           *Config
	keymap        map[string]string
	theme         *Theme
	panels        [2]*Panel
	active        int
	dialogs       []Dialog
	status        string
	statusErr     bool
	index         *Index
	quit          bool
	qsActive      bool
	qsText        []rune
	w, h          int
	showHidden    bool
	cmd           *LineEdit // the command line at the bottom
	history       []string  // command history, oldest first
	histPos       int
	dirHistory    []string              // directories visited, oldest first
	fileHistory   []string              // files viewed, edited or opened, oldest first
	hidden        [2]bool               // individual panels hidden (Ctrl+F1 / Ctrl+P)
	hideAll       bool                  // both panels hidden (Ctrl+O)
	heights       [2]int                // rows taken off a panel's height (Ctrl+Up/Down), 0 = full
	split         int                   // shift of the divider from the middle (Ctrl+Left/Right)
	clipboard     string                // last text copied inside NEAR
	readClip      func() (string, bool) // system clipboard reader; nil in tests
	tabPending    string                // command line text at an ambiguous Tab; a second Tab lists
	lastFind      findSpec              // what the Find file dialog asked for last time
	console       *Console              // output of commands run from the command line
	running       *exec.Cmd
	runStart      time.Time
	restorePanels bool // panels were visible before the command: show them again when it ends

	lastProgress     time.Time // last progress box redraw
	lastProgressItem string
	spinner          int
	transfers        *transferQueue  // copies and moves, run one after another
	slowBoxShown     bool            // the slow directory box was drawn (tests)
	lastApply        string          // last Apply command template
	cmdQueue         []queuedCommand // console commands waiting to run one after another
	panelRect        [2]rect         // where the panels were drawn, for the mouse
	lastClick        time.Time
	lastClickAt      [2]int
	lastButtons      tcell.ButtonMask
}

// rect is a screen rectangle.
type rect struct{ x, y, w, h int }

// panelVisible reports whether panel i is drawn.
func (a *App) panelVisible(i int) bool { return !a.hideAll && !a.hidden[i] }

// minPanelHeight is the frame plus one entry row.
const minPanelHeight = panelChrome + 1

// consoleBand is the number of rows under the panels that show the tail of
// the console: whatever the panels leave free once a command has produced
// output. ensureConsoleBand opens console_rows when output arrives; Ctrl+Up
// and Ctrl+Down then trade rows between the panels and the band.
func (a *App) consoleBand() int {
	if !a.console.HasContent() {
		return 0
	}
	tallest := 0
	for i := range a.panels {
		if a.panelVisible(i) && a.panelHeight(i) > tallest {
			tallest = a.panelHeight(i)
		}
	}
	return maxInt(0, a.panelArea()-tallest)
}

// ensureConsoleBand shortens the panels so that at least console_rows of
// output are visible under them (never making them taller).
func (a *App) ensureConsoleBand() {
	rows := clampInt(a.cfg.ConsoleRows, 0, maxInt(0, a.panelArea()-minPanelHeight))
	for i := range a.heights {
		if a.heights[i] < rows {
			a.heights[i] = rows
		}
	}
}

// panelArea is the height available to the panels.
func (a *App) panelArea() int { return a.h - bottomRows }

// panelHeight returns the drawn height of panel i.
func (a *App) panelHeight(i int) int {
	full := a.panelArea()
	return clampInt(full-a.heights[i], minPanelHeight, maxInt(full, minPanelHeight))
}

// panelSplit returns the column where the right panel starts.
func (a *App) panelSplit() int {
	const minWidth = 20
	if a.w < 2*minWidth {
		return a.w / 2
	}
	return clampInt(a.w/2+a.split, minWidth, a.w-minWidth)
}

// resizePanels changes the height of both panels (which < 0) or of the
// active one by delta rows; a positive delta makes the panel taller.
func (a *App) resizePanels(which, delta int) {
	full := a.panelArea()
	for i := range a.heights {
		if which >= 0 && i != which {
			continue
		}
		a.heights[i] = clampInt(a.heights[i]-delta, 0, maxInt(0, full-minPanelHeight))
	}
}

func newApp(cfg *Config, s tcell.Screen) *App {
	a := &App{screen: s, cfg: cfg, showHidden: cfg.ShowHidden, cmd: newLineEdit(""), readClip: readClipboard}
	a.theme = themeByName(cfg.Theme)
	a.keymap = buildKeymap(cfg)
	a.index = newIndex()
	a.console = newConsole(a.theme.Cmd, cfg.ConsoleLines)
	a.transfers = newTransferQueue(s)
	for i := range a.panels {
		a.panels[i] = &Panel{}
	}
	return a
}

func buildKeymap(cfg *Config) map[string]string {
	km := map[string]string{}
	for action, keys := range cfg.Keys {
		for _, k := range keys {
			if n := normKey(k); n != "" {
				km[n] = action
			}
		}
	}
	for _, m := range cfg.Menus {
		if m.Key != "" {
			km[normKey(m.Key)] = "menu:" + m.Name
		}
	}
	return km
}

// setup restores the previous session (or defaults) and applies CLI dirs.
func (a *App) setup(leftArg, rightArg string) {
	st := loadState(a.cfg.dir)
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	for i := 0; i < 2; i++ {
		p := a.panels[i]
		if st != nil && i < len(st.Panels) {
			for _, path := range st.Panels[i].Tabs {
				if isDir(path) {
					p.Tabs = append(p.Tabs, newTab(path))
				}
			}
			if len(p.Tabs) > 0 {
				p.Active = clampInt(st.Panels[i].Active, 0, len(p.Tabs)-1)
			}
		}
		if len(p.Tabs) == 0 {
			def := cwd
			if i == 1 {
				def = home
			}
			if def == "" {
				def = string(filepath.Separator)
			}
			p.Tabs = []*Tab{newTab(def)}
		}
	}
	if st != nil {
		a.active = clampInt(st.Active, 0, 1)
		a.history = st.History
		a.histPos = len(a.history)
		a.dirHistory = st.DirHistory
		a.fileHistory = st.FileHistory
		a.split = st.Split
		for i := range a.heights {
			if i < len(st.Heights) {
				a.heights[i] = maxInt(0, st.Heights[i])
			}
		}
	}
	if leftArg != "" {
		a.panels[0].Tab().Path = absPath(leftArg)
	}
	if rightArg != "" {
		a.panels[1].Tab().Path = absPath(rightArg)
	}
	for _, p := range a.panels {
		a.loadTab(p.Tab())
	}
}

func (a *App) saveSession() {
	st := &State{Active: a.active, History: a.history, DirHistory: a.dirHistory, FileHistory: a.fileHistory, Split: a.split, Heights: a.heights[:]}
	for _, p := range a.panels {
		ps := PanelState{Active: p.Active}
		for _, t := range p.Tabs {
			ps.Tabs = append(ps.Tabs, t.Path)
		}
		st.Panels = append(st.Panels, ps)
	}
	_ = saveState(a.cfg.dir, st)
}

func (a *App) startIndex() {
	a.index.Configure(a.cfg, a.screen)
	if err := a.index.LoadCache(); err == nil {
		if !a.cfg.SearchIndexOnStart && a.cfg.SearchWatch {
			a.index.StartWatch()
		}
	}
	if a.cfg.SearchIndexOnStart {
		a.index.RebuildAsync()
	}
}

func (a *App) rebuildIndex() {
	if a.index.Building() {
		return
	}
	a.index.RebuildAsync()
}

// run is the main event loop.
func (a *App) run() {
	for !a.quit {
		a.draw()
		ev := a.screen.PollEvent()
		if ev == nil {
			return
		}
		a.handleEvent(ev)
	}
	a.saveSession()
	a.index.Close()
}

func (a *App) handleEvent(ev tcell.Event) {
	switch ev := ev.(type) {
	case *tcell.EventKey:
		a.handleKey(ev)
	case *tcell.EventResize:
		a.screen.Sync()
	case *EventIndexDone:
		// The search dialog shows the index state; only failures are reported.
		if err := a.index.Err(); err != nil {
			a.setErr(fmt.Errorf("search index: %v", err))
		}
	case *EventIndexChanged, *EventIndexProgress:
		// The index was updated by the watcher or is being built; redraw picks it up.
	case *tcell.EventClipboard:
		if len(a.dialogs) > 0 {
			if e, ok := a.top().(*Editor); ok {
				e.pasteText(string(ev.Data()))
			}
		} else if len(ev.Data()) > 0 {
			a.cmd.insert(firstLine(string(ev.Data())))
		}
	case *EventConsole:
		// new output arrived; the redraw shows it
	case *EventCommandDone:
		a.commandDone(ev.err)
	case *EventTransfer:
		// progress changed; the redraw shows it
	case *EventTransferDone:
		a.transferDone(ev.t)
	case *tcell.EventMouse:
		a.handleMouse(ev)
	case *EventSearchResult:
		if len(a.dialogs) > 0 {
			if d, ok := a.top().(*SearchDialog); ok {
				d.apply(a, ev)
			}
		}
	}
}

func (a *App) handleKey(ev *tcell.EventKey) {
	if len(a.dialogs) > 0 {
		a.top().HandleKey(a, ev)
		return
	}
	a.status, a.statusErr = "", false
	if a.qsActive && a.handleQuickSearch(ev) {
		return
	}
	name := keyName(ev)
	// While a console command runs there is no prompt, like in a shell:
	// only stopping it, scrolling, peeking at the panels and quitting work.
	if a.running != nil {
		switch {
		case name == "Ctrl+C":
			a.stopCommand()
		case a.keymap[name] == "toggle_panels" || a.keymap[name] == "quit":
			a.restorePanels = false // the user took over; leave the view as they set it
			a.doAction(a.keymap[name])
		case a.hideAll && (name == "PgUp" || name == "PgDn" || name == "Home" || name == "End"):
			rows := a.h - bottomRows + 1
			delta := map[string]int{"PgUp": rows - 1, "PgDn": -(rows - 1), "Home": 1 << 30, "End": -(1 << 30)}[name]
			a.console.scrollBy(delta, a.w, rows)
		}
		return
	}
	// In the console view the keys behave like in a shell: nothing reaches
	// the invisible panels, Up/Down walk the history, paging keys scroll.
	if a.hideAll {
		rows := a.h - bottomRows + 1 // the console also uses the (cleared) status row
		switch name {
		case "Tab":
			a.completeCmd()
			return
		case "Shift+Tab", "Shift+Up", "Shift+Down", "Shift+Home", "Shift+End", "Shift+PgUp", "Shift+PgDn":
			return
		case "Up":
			a.historyStep(-1)
			return
		case "Down":
			a.historyStep(1)
			return
		case "Enter":
			if len(a.cmd.Text) == 0 {
				a.console.Println(a.prompt(), a.theme.CmdPrompt)
				return
			}
		case "PgUp":
			a.console.scrollBy(rows-1, a.w, rows)
			return
		case "PgDn":
			a.console.scrollBy(-(rows - 1), a.w, rows)
			return
		case "Home":
			if len(a.cmd.Text) == 0 {
				a.console.scrollBy(1<<30, a.w, rows)
				return
			}
		case "End":
			if len(a.cmd.Text) == 0 {
				a.console.scrollBy(-(1 << 30), a.w, rows)
				return
			}
		}
	}
	action, bound := a.keymap[name]
	typing := len(a.cmd.Text) > 0
	plainRune := ev.Key() == tcell.KeyRune && ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt|tcell.ModMeta) == 0
	altRune := ev.Key() == tcell.KeyRune && ev.Modifiers()&(tcell.ModAlt|tcell.ModMeta) != 0 && ev.Modifiers()&tcell.ModCtrl == 0

	// Alt+<character> that is not bound starts a quick search in the panel.
	if altRune && !bound {
		a.qsActive = true
		a.qsText = append(a.qsText[:0], ev.Rune())
		a.quickSearchJump(0)
		return
	}

	// The command line has the focus: it takes printable characters, and
	// while it holds text it also owns Enter, Esc and the editing keys.
	if typing {
		switch name {
		case "Enter":
			a.runCommandLine(false)
			return
		case "Shift+Enter":
			a.runCommandLine(true)
			return
		case "Esc":
			a.cmd.Set("")
			a.histPos = len(a.history)
			return
		case "Left", "Right", "Home", "End", "Ctrl+Left", "Ctrl+Right",
			"Shift+Left", "Shift+Right", "Shift+Home", "Shift+End", "Ctrl+Shift+Left", "Ctrl+Shift+Right",
			"Backspace", "Delete", "Ctrl+K", "Alt+Backspace", "Ctrl+Backspace", "Ctrl+C", "Ctrl+V", "Ctrl+Insert":
			a.cmd.HandleKey(ev, a)
			return
		}
	}
	if plainRune && (typing || !bound) {
		a.cmd.HandleKey(ev, a)
		a.histPos = len(a.history)
		return
	}
	// Pasting works on an empty command line too.
	if name == "Ctrl+V" || name == "Shift+Insert" {
		a.cmd.HandleKey(ev, a)
		a.histPos = len(a.history)
		return
	}
	if !bound {
		return
	}
	a.doAction(action)
}

// ---------------------------------------------------------------- helpers

func (a *App) push(d Dialog) { a.dialogs = append(a.dialogs, d) }

func (a *App) pop() {
	if len(a.dialogs) > 0 {
		a.dialogs = a.dialogs[:len(a.dialogs)-1]
	}
}

func (a *App) top() Dialog { return a.dialogs[len(a.dialogs)-1] }

func (a *App) setStatus(s string) { a.status, a.statusErr = s, false }

func (a *App) setErr(err error) {
	if err == nil {
		return
	}
	a.status, a.statusErr = err.Error(), true
}

func (a *App) panel() *Panel { return a.panels[a.active] }
func (a *App) tab() *Tab     { return a.panels[a.active].Tab() }
func (a *App) other() *Tab   { return a.panels[1-a.active].Tab() }

// loadTab re-reads a tab's directory with the slow-read box; giving up
// keeps the previous listing.
func (a *App) loadTab(t *Tab) {
	if t.Find != nil {
		a.applyFind(t)
		return
	}
	entries, notes, err := a.listDirWait(t.Path)
	if errors.Is(err, errDirCancelled) {
		a.setStatus("Directory read abandoned; showing the previous listing")
		return
	}
	t.Apply(entries, notes, err)
	if t.Err != nil {
		a.setErr(t.Err)
	}
}

func (a *App) reloadAll() {
	for _, p := range a.panels {
		a.loadTab(p.Tab())
	}
}

// navigate changes a tab's directory, placing the cursor on cursorName.
func (a *App) navigate(t *Tab, path, cursorName string) error {
	path = absPath(path)
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		cursorName = filepath.Base(path)
		path = filepath.Dir(path)
	}
	// Read the new directory first: if that is slow and the user gives up,
	// the tab stays where it was.
	entries, notes, err := a.listDirWait(path)
	if errors.Is(err, errDirCancelled) {
		a.setStatus("Directory read abandoned; staying in " + fitRight(t.Path, 40))
		return nil
	}
	if cur := t.Current(); cur != nil {
		t.posMem[t.Path] = cur.Name
	}
	t.Path = path
	t.Find = nil
	t.Cursor, t.Top = 0, 0
	t.Selected = map[string]bool{}
	t.Apply(entries, notes, err)
	if t.Err != nil {
		return t.Err
	}
	if cursorName == "" {
		cursorName = t.posMem[path]
	}
	if cursorName != "" {
		t.SetCursorOn(cursorName)
	}
	a.dirHistory = pushHistory(a.dirHistory, path, historyMax)
	return nil
}

func (a *App) enterCurrent() {
	t := a.tab()
	e := t.Current()
	if e == nil {
		return
	}
	if e.IsUp {
		a.goUp()
		return
	}
	full := filepath.Join(t.Path, e.Name)
	if e.IsDir {
		if err := a.navigate(t, full, ""); err != nil {
			a.setErr(err)
		}
		return
	}
	a.openWithOS(full)
}

func (a *App) goUp() {
	t := a.tab()
	if t.Find != nil {
		// Leaving the search results shows the directory itself again.
		a.leaveFind(t)
		return
	}
	parent := filepath.Dir(t.Path)
	if parent == t.Path {
		a.showLocations()
		return
	}
	base := filepath.Base(t.Path)
	if err := a.navigate(t, parent, base); err != nil {
		a.setErr(err)
	}
}

// ---------------------------------------------------------------- quick search

func (a *App) handleQuickSearch(ev *tcell.EventKey) bool {
	switch keyName(ev) {
	case "Esc":
		a.qsActive = false
		return true
	case "Enter":
		// Enter ends the search and opens the match right away.
		a.qsActive = false
		a.enterCurrent()
		return true
	case "Backspace":
		if len(a.qsText) > 0 {
			a.qsText = a.qsText[:len(a.qsText)-1]
			a.quickSearchJump(0)
		}
		return true
	case "Down", "Ctrl+S":
		a.quickSearchJump(1)
		return true
	case "Up":
		a.quickSearchJump(-1)
		return true
	}
	// Plain and Alt+character keys both extend the search, so the search
	// can be typed with Alt held down throughout.
	if ev.Key() == tcell.KeyRune && ev.Modifiers()&tcell.ModCtrl == 0 {
		a.qsText = append(a.qsText, ev.Rune())
		a.quickSearchJump(0)
		return true
	}
	a.qsActive = false
	return false
}

// quickSearchJump moves to the next (dir>0), previous (dir<0) or first
// matching entry starting at the cursor (dir==0).
func (a *App) quickSearchJump(dir int) {
	t := a.tab()
	n := len(t.Entries)
	if n == 0 || len(a.qsText) == 0 {
		return
	}
	q := strings.ToLower(string(a.qsText))
	match := func(i int, prefix bool) bool {
		name := strings.ToLower(t.Entries[i].Name)
		if prefix {
			return strings.HasPrefix(name, q)
		}
		return strings.Contains(name, q)
	}
	start := t.Cursor
	if dir != 0 {
		start = (t.Cursor + dir + n) % n
	}
	for _, prefix := range []bool{true, false} {
		for k := 0; k < n; k++ {
			i := ((start+k*step(dir))%n + n) % n
			if match(i, prefix) {
				t.Cursor = i
				return
			}
		}
	}
}

func step(dir int) int {
	if dir < 0 {
		return -1
	}
	return 1
}

// ---------------------------------------------------------------- actions

func (a *App) doAction(action string) {
	t := a.tab()
	switch action {
	case "quit":
		if a.transfers.active() {
			a.confirm("Transfers are running", []string{"Quit anyway? Running transfers are cancelled."}, []string{"Quit", "Cancel"}, func(r int) {
				if r == 0 {
					for _, t := range a.transfers.pending() {
						a.transfers.remove(t)
					}
					a.quit = true
				}
			})
			return
		}
		if a.running != nil {
			a.stopCommand()
		}
		a.quit = true
	case "transfers":
		a.showTransfers()
	case "apply_command":
		a.applyCommand()
	case "context_menu":
		a.showContextMenu()
	case "help":
		a.showHelp()
	case "switch_panel":
		if a.hideAll || a.panelVisible(1-a.active) {
			a.active = 1 - a.active
		}
	case "toggle_panels":
		a.hideAll = !a.hideAll
	case "toggle_inactive_panel":
		a.togglePanel(1 - a.active)
	case "toggle_left_panel":
		a.togglePanel(0)
	case "toggle_right_panel":
		a.togglePanel(1)
	case "panels_taller":
		a.resizePanels(-1, 1)
	case "panels_shorter":
		a.resizePanels(-1, -1)
	case "panel_taller":
		a.resizePanels(a.active, 1)
	case "panel_shorter":
		a.resizePanels(a.active, -1)
	case "split_left":
		a.split--
		a.split = a.panelSplit() - a.w/2
	case "split_right":
		a.split++
		a.split = a.panelSplit() - a.w/2
	case "panels_default_width":
		a.split = 0
	case "panels_default_height":
		a.heights = [2]int{}
	case "up":
		t.MoveCursor(-1)
	case "down":
		t.MoveCursor(1)
	case "page_up":
		t.MoveCursor(-a.listRows(a.active))
	case "page_down":
		t.MoveCursor(a.listRows(a.active))
	case "home":
		t.Cursor = 0
	case "end":
		t.Cursor = len(t.Entries) - 1
		t.clamp()
	case "enter":
		a.enterCurrent()
	case "up_dir":
		a.goUp()
	case "select":
		t.ToggleSelect()
		t.MoveCursor(1)
	case "select_down":
		t.ToggleSelect()
		t.MoveCursor(1)
	case "select_up":
		t.ToggleSelect()
		t.MoveCursor(-1)
	case "select_to_home":
		t.SelectRange(0)
	case "select_to_end":
		t.SelectRange(len(t.Entries) - 1)
	case "select_page_up":
		t.SelectRange(maxInt(0, t.Cursor-a.listRows(a.active)))
	case "select_page_down":
		t.SelectRange(clampInt(t.Cursor+a.listRows(a.active), 0, len(t.Entries)-1))
	case "select_pattern":
		a.selectPattern(true)
	case "unselect_pattern":
		a.selectPattern(false)
	case "invert_selection":
		for _, e := range t.Entries {
			if e.IsUp {
				continue
			}
			if t.Selected[e.Name] {
				delete(t.Selected, e.Name)
			} else {
				t.Selected[e.Name] = true
			}
		}
	case "select_all":
		for _, e := range t.Entries {
			if !e.IsUp {
				t.Selected[e.Name] = true
			}
		}
	case "unselect_all":
		t.Selected = map[string]bool{}
	case "new_tab":
		nt := newTab(t.Path)
		nt.Sort, nt.SortRev = t.Sort, t.SortRev
		p := a.panel()
		p.Tabs = append(p.Tabs[:p.Active+1], append([]*Tab{nt}, p.Tabs[p.Active+1:]...)...)
		p.Active++
		a.loadTab(nt)
		nt.Cursor = t.Cursor
	case "close_tab":
		p := a.panel()
		if len(p.Tabs) == 1 {
			a.setStatus("Cannot close the last tab")
			return
		}
		p.Tabs = append(p.Tabs[:p.Active], p.Tabs[p.Active+1:]...)
		if p.Active >= len(p.Tabs) {
			p.Active = len(p.Tabs) - 1
		}
		a.loadTab(p.Tab())
	case "next_tab":
		p := a.panel()
		p.Active = (p.Active + 1) % len(p.Tabs)
		a.loadTab(p.Tab())
	case "prev_tab":
		p := a.panel()
		p.Active = (p.Active + len(p.Tabs) - 1) % len(p.Tabs)
		a.loadTab(p.Tab())
	case "view":
		a.viewCurrent()
	case "edit":
		a.editCurrent()
	case "edit_new":
		a.input("Edit new file", "File name:", "", func(name string) {
			if name = strings.TrimSpace(name); name == "" {
				return
			}
			if !filepath.IsAbs(expandPath(name)) {
				name = filepath.Join(a.tab().Path, name)
			}
			a.editFileThen(absPath(name), false, func() { a.reloadAll(); a.tab().SetCursorOn(filepath.Base(name)) })
		})
	case "edit_external":
		if e := t.Current(); e != nil && !e.IsUp && !e.IsDir {
			a.editFileThen(filepath.Join(t.Path, e.Name), true, nil)
		}
	case "copy_path":
		p := t.Path
		if e := t.Current(); e != nil && !e.IsUp {
			p = filepath.Join(t.Path, e.Name)
		}
		a.setClipboard(p)
		a.setStatus("Copied " + p)
	case "copy":
		a.copyOrMove(false)
	case "move":
		a.copyOrMove(true)
	case "rename":
		a.renameCurrent()
	case "mass_rename":
		a.massRename()
	case "mkdir":
		a.makeDir()
	case "delete":
		a.deleteSelected(!a.cfg.DeleteToTrash)
	case "delete_permanent":
		a.deleteSelected(true)
	case "quick_launch":
		if len(a.cfg.Menus) == 0 {
			a.setStatus("No menus defined in " + a.cfg.path)
			return
		}
		a.openMenu(&a.cfg.Menus[0])
	case "menus":
		a.showMenuList()
	case "notes":
		a.editNote()
	case "search":
		a.showSearch()
	case "find_file":
		a.showFindFile()
	case "quick_search":
		a.qsActive = true
		a.qsText = a.qsText[:0]
	case "sort":
		a.showSortMenu()
	case "toggle_hidden":
		a.showHidden = !a.showHidden
		a.reloadAll()
	case "refresh":
		a.reloadAll()
		a.setStatus("Reloaded")
	case "swap_panels":
		a.panels[0], a.panels[1] = a.panels[1], a.panels[0]
		a.active = 1 - a.active
	case "sync_other":
		o := a.other()
		if err := a.navigate(o, t.Path, ""); err != nil {
			a.setErr(err)
		}
	case "locations":
		a.showLocations()
	case "goto_path":
		a.input("Go to directory", "Path:", t.Path, func(s string) {
			if strings.TrimSpace(s) == "" {
				return
			}
			if err := a.navigate(a.tab(), s, ""); err != nil {
				a.setErr(err)
			}
		})
	case "run_command":
		a.input("Run command in "+fitRight(t.Path, 40), "Command (placeholders: {file} {files} {dir} {other} …):", "", func(s string) {
			if strings.TrimSpace(s) == "" {
				return
			}
			a.runShell(a.expandPlaceholders(s), a.tab().Path, true, false)
		})
	case "insert_name":
		if e := t.Current(); e != nil && !e.IsUp {
			a.insertIntoCmd(e.Name)
		}
	case "insert_path":
		if e := t.Current(); e != nil && !e.IsUp {
			a.insertIntoCmd(filepath.Join(t.Path, e.Name))
		}
	case "history_prev":
		a.historyStep(-1)
	case "history_next":
		a.historyStep(1)
	case "history":
		a.showHistory()
	case "folder_history":
		a.showDirHistory()
	case "file_history":
		a.showFileHistory()
	case "compare":
		a.showCompareMenu()
	default:
		switch {
		case strings.HasPrefix(action, "menu:"):
			m := a.cfg.menu(strings.TrimPrefix(action, "menu:"))
			if m == nil {
				a.setErr(fmt.Errorf("no menu named %q", strings.TrimPrefix(action, "menu:")))
				return
			}
			a.openMenu(m)
		case strings.HasPrefix(action, "tab_"):
			n, err := strconv.Atoi(strings.TrimPrefix(action, "tab_"))
			p := a.panel()
			if err == nil && n >= 1 && n <= len(p.Tabs) {
				p.Active = n - 1
				a.loadTab(p.Tab())
			}
		default:
			a.setErr(fmt.Errorf("unknown action %q", action))
		}
	}
}

// togglePanel hides or shows one panel; the other one takes the full
// width. The active panel is never left hidden.
func (a *App) togglePanel(i int) {
	a.hidden[i] = !a.hidden[i]
	if a.hidden[0] && a.hidden[1] {
		a.hidden[1-i] = false
	}
	if a.hidden[a.active] {
		a.active = 1 - a.active
	}
}

// ---------------------------------------------------------------- help

func (a *App) showHelp() {
	byAction := map[string][]string{}
	for key, action := range a.keymap {
		byAction[action] = append(byAction[action], key)
	}
	var lines []string
	lines = append(lines, "NEAR - keyboard reference (from "+a.cfg.path+")", "")
	for _, h := range actionHelp {
		keys := byAction[h.Action]
		sortStrings(keys)
		lines = append(lines, fmt.Sprintf("  %-22s %-24s %s", h.Action, strings.Join(keys, ", "), h.Help))
	}
	lines = append(lines, "")
	for _, m := range a.cfg.Menus {
		if m.Key != "" {
			lines = append(lines, fmt.Sprintf("  %-22s %-24s open menu %q", "menu:"+m.Name, normKey(m.Key), m.Name))
		}
	}
	lines = append(lines, "",
		"Command line: always focused. Printable keys type into it; while it has text,",
		"              Enter runs it in the active panel's directory, Esc clears it, and",
		"              Left/Right/Home/End/Backspace/Delete edit it. Keys bound to actions",
		"              (+ - * Space) only act while it is empty. \"cd <dir>\" moves the panel.",
		"Panels: * marks an entry with a note, @ a symbolic link.",
		"Quick search: Alt+<character> (or Ctrl+S) jumps to a matching name in the panel;",
		"              keep typing with or without Alt, Up/Down next/previous, Esc to leave.",
		"Find file (Alt+F7): FAR's dialog; masks like *.go;*.md, text or hex content, the",
		"               results replace the panel (Esc or .. returns to the directory).",
		"Search dialog (Ctrl+F): words match the file name, a term with / matches the path,",
		"               * and ? are wildcards. Enter opens, Ctrl+T opens in a new tab.",
		"Mass rename: regex Find/Replace with $1 groups and {n} {name} {ext} {date}",
		"             {parent} tokens; F4 edits the name list in your editor instead.",
	)
	a.push(newViewer("Help", strings.Join(lines, "\n"), false))
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && lessKey(s[j], s[j-1]); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func lessKey(a, b string) bool {
	// Function keys and short keys first, then alphabetical.
	ra, rb := []rune(a), []rune(b)
	if len(ra) != len(rb) && (len(ra) <= 2 || len(rb) <= 2) {
		return len(ra) < len(rb)
	}
	return strings.ToLower(a) < strings.ToLower(b)
}
