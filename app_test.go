package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/gdamore/tcell/v2"
)

func testApp(t *testing.T) (*App, string) {
	t.Helper()
	root := t.TempDir()
	cfgDir := filepath.Join(root, "cfg")
	data := filepath.Join(root, "data")
	for _, d := range []string{"alpha", "beta", "gamma/inner"} {
		if err := os.MkdirAll(filepath.Join(data, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"file1.txt", "file2.txt", "file10.txt", "photo.jpg", "gamma/inner/deep.md"} {
		if err := os.WriteFile(filepath.Join(data, f), []byte("hello "+f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := loadConfig(filepath.Join(cfgDir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.SearchIndexOnStart = false
	cfg.SearchRoots = []string{data}
	s := tcell.NewSimulationScreen("")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	s.SetSize(120, 40)
	a := newApp(cfg, s)
	a.readClip = nil // tests must not see the developer's clipboard
	a.setup(data, root)
	return a, data
}

func key(a *App, name string) {
	ev := eventFor(name)
	a.handleEvent(ev)
	a.draw()
}

func typeText(a *App, s string) {
	for _, r := range s {
		a.handleEvent(tcell.NewEventKey(tcell.KeyRune, r, 0))
	}
	a.draw()
}

func eventFor(name string) *tcell.EventKey {
	var mod tcell.ModMask
	for {
		switch {
		case strings.HasPrefix(name, "Ctrl+") && len(name) > 5:
			mod |= tcell.ModCtrl
			name = name[5:]
			continue
		case strings.HasPrefix(name, "Alt+") && len(name) > 4:
			mod |= tcell.ModAlt
			name = name[4:]
			continue
		case strings.HasPrefix(name, "Shift+") && len(name) > 6:
			mod |= tcell.ModShift
			name = name[6:]
			continue
		}
		break
	}
	switch name {
	case "Enter":
		return tcell.NewEventKey(tcell.KeyEnter, 0, mod)
	case "Esc":
		return tcell.NewEventKey(tcell.KeyEscape, 0, mod)
	case "Tab":
		if mod&tcell.ModShift != 0 {
			return tcell.NewEventKey(tcell.KeyBacktab, 0, 0)
		}
		return tcell.NewEventKey(tcell.KeyTab, 0, mod)
	case "Backspace":
		return tcell.NewEventKey(tcell.KeyBackspace2, 0, mod)
	case "Up":
		return tcell.NewEventKey(tcell.KeyUp, 0, mod)
	case "Down":
		return tcell.NewEventKey(tcell.KeyDown, 0, mod)
	case "Left":
		return tcell.NewEventKey(tcell.KeyLeft, 0, mod)
	case "Right":
		return tcell.NewEventKey(tcell.KeyRight, 0, mod)
	case "Insert":
		return tcell.NewEventKey(tcell.KeyInsert, 0, mod)
	case "Delete":
		return tcell.NewEventKey(tcell.KeyDelete, 0, mod)
	case "Home":
		return tcell.NewEventKey(tcell.KeyHome, 0, mod)
	case "End":
		return tcell.NewEventKey(tcell.KeyEnd, 0, mod)
	case "PgUp":
		return tcell.NewEventKey(tcell.KeyPgUp, 0, mod)
	case "PgDn":
		return tcell.NewEventKey(tcell.KeyPgDn, 0, mod)
	case "Space":
		return tcell.NewEventKey(tcell.KeyRune, ' ', mod)
	}
	if strings.HasPrefix(name, "F") && allDigits(name[1:]) {
		n := 0
		for _, c := range name[1:] {
			n = n*10 + int(c-'0')
		}
		return tcell.NewEventKey(tcell.KeyF1+tcell.Key(n-1), 0, mod)
	}
	r := []rune(name)[0]
	if mod&tcell.ModCtrl != 0 && r >= 'A' && r <= 'Z' {
		return tcell.NewEventKey(tcell.KeyCtrlA+tcell.Key(r-'A'), 0, mod)
	}
	return tcell.NewEventKey(tcell.KeyRune, r, mod)
}

func names(t *Tab) []string {
	var out []string
	for _, e := range t.Entries {
		out = append(out, e.Name)
	}
	return out
}

func screenText(a *App) string {
	cells, w, h := a.screen.(tcell.SimulationScreen).GetContents()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Runes) > 0 {
				b.WriteRune(c.Runes[0])
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func TestKeyNames(t *testing.T) {
	cases := map[string]string{
		"ctrl+f": "Ctrl+F", "Escape": "Esc", "shift+f6": "Shift+F6", "alt+right": "Alt+Right",
		"+": "+", "PageDown": "PgDn", "Ctrl+Alt+x": "Ctrl+Alt+X", "a": "a", "Return": "Enter",
	}
	for in, want := range cases {
		if got := normKey(in); got != want {
			t.Errorf("normKey(%q) = %q, want %q", in, got, want)
		}
	}
	evs := map[string]*tcell.EventKey{
		"Ctrl+F":   tcell.NewEventKey(tcell.KeyCtrlF, 0, tcell.ModCtrl),
		"Esc":      tcell.NewEventKey(tcell.KeyEscape, 0, 0),
		"Shift+F6": tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModShift),
		"Alt+R":    tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModAlt),
		"+":        tcell.NewEventKey(tcell.KeyRune, '+', 0),
		"Space":    tcell.NewEventKey(tcell.KeyRune, ' ', 0),
		"Enter":    tcell.NewEventKey(tcell.KeyEnter, 0, 0),
	}
	for want, ev := range evs {
		if got := keyName(ev); got != want {
			t.Errorf("keyName = %q, want %q", got, want)
		}
	}
}

func TestNaturalSort(t *testing.T) {
	a, _ := testApp(t)
	got := strings.Join(names(a.tab()), " ")
	want := ".. alpha beta gamma file1.txt file2.txt file10.txt photo.jpg"
	if got != want {
		t.Fatalf("listing = %q, want %q", got, want)
	}
}

func TestNavigateAndEscUp(t *testing.T) {
	a, data := testApp(t)
	a.draw()
	key(a, "Down")
	key(a, "Down")
	key(a, "Down") // gamma
	key(a, "Enter")
	if a.tab().Path != filepath.Join(data, "gamma") {
		t.Fatalf("path = %s", a.tab().Path)
	}
	key(a, "Down")
	key(a, "Enter")
	if filepath.Base(a.tab().Path) != "inner" {
		t.Fatalf("path = %s", a.tab().Path)
	}
	key(a, "Esc")
	key(a, "Esc")
	if a.tab().Path != data {
		t.Fatalf("after Esc Esc path = %s", a.tab().Path)
	}
	if cur := a.tab().Current(); cur == nil || cur.Name != "gamma" {
		t.Fatalf("cursor should return to gamma, got %+v", cur)
	}
	if !strings.Contains(screenText(a), "gamma") {
		t.Fatal("gamma not drawn")
	}
}

func TestTabs(t *testing.T) {
	a, data := testApp(t)
	key(a, "Ctrl+T")
	if len(a.panel().Tabs) != 2 || a.panel().Active != 1 {
		t.Fatalf("tabs=%d active=%d", len(a.panel().Tabs), a.panel().Active)
	}
	key(a, "Down")
	key(a, "Enter") // alpha in tab 2
	key(a, "Alt+1")
	if a.tab().Path != data {
		t.Fatalf("tab 1 path = %s", a.tab().Path)
	}
	key(a, "Ctrl+PgDn")
	if filepath.Base(a.tab().Path) != "alpha" {
		t.Fatalf("tab 2 path = %s", a.tab().Path)
	}
	if !strings.Contains(screenText(a), "2:alpha") {
		t.Fatal("tab label missing")
	}
	key(a, "Ctrl+W")
	if len(a.panel().Tabs) != 1 {
		t.Fatal("tab not closed")
	}
	key(a, "Ctrl+W")
	if len(a.panel().Tabs) != 1 {
		t.Fatal("last tab must survive")
	}
	key(a, "Tab")
	if a.active != 1 {
		t.Fatal("panel switch failed")
	}
}

func TestSelectAndMassRename(t *testing.T) {
	a, data := testApp(t)
	key(a, "+")
	a.top().(*InputDialog).Edit.Set("*.txt")
	key(a, "Enter")
	if n, _ := a.tab().SelectedStats(); n != 3 {
		t.Fatalf("selected %d, want 3", n)
	}
	key(a, "Ctrl+F6")
	d, ok := a.top().(*MassRenameDialog)
	if !ok {
		t.Fatalf("expected rename dialog, got %T", a.top())
	}
	typeText(a, `file(\d+)`)
	key(a, "Tab")
	typeText(a, `doc_{n}_$1`)
	key(a, "Tab")
	key(a, "Backspace")
	typeText(a, "5")
	key(a, "Tab")
	key(a, "Tab")
	key(a, "Backspace")
	typeText(a, "2")
	if d.errs != 0 {
		t.Fatalf("unexpected conflicts: %+v", d.pairs)
	}
	key(a, "Enter")
	got := strings.Join(names(a.tab()), " ")
	want := ".. alpha beta gamma doc_05_1.txt doc_06_2.txt doc_07_10.txt photo.jpg"
	if got != want {
		t.Fatalf("after rename: %q\nwant %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(data, "doc_07_10.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestMassRenameConflict(t *testing.T) {
	a, _ := testApp(t)
	key(a, "Ctrl+A")
	key(a, "Ctrl+F6")
	d := a.top().(*MassRenameDialog)
	key(a, "Tab")
	typeText(a, "same")
	if d.errs == 0 {
		t.Fatal("expected duplicate conflicts")
	}
	key(a, "Enter")
	if _, still := a.top().(*MassRenameDialog); !still {
		t.Fatal("dialog must stay open on conflicts")
	}
	if !a.statusErr {
		t.Fatal("expected error status")
	}
}

func TestNotes(t *testing.T) {
	a, data := testApp(t)
	key(a, "Down") // alpha
	key(a, "Ctrl+N")
	typeText(a, "project notes")
	key(a, "Enter")
	typeText(a, "second line")
	key(a, "Ctrl+S")
	notes := loadNotes(data, a.cfg.NotesFile)
	if notes["alpha"] != "project notes\nsecond line" {
		t.Fatalf("note = %q", notes["alpha"])
	}
	if _, err := os.Stat(filepath.Join(data, ".near-notes")); err != nil {
		t.Fatal(err)
	}
	a.draw()
	txt := screenText(a)
	if !strings.Contains(txt, "✎ project notes …") {
		t.Fatalf("note line not shown:\n%s", txt)
	}
	marked := false
	for _, line := range strings.Split(txt, "\n") {
		if strings.Contains(line, "alpha") && strings.Contains(line, "*│") {
			marked = true
		}
	}
	if !marked {
		t.Fatalf("note marker missing:\n%s", txt)
	}
	for _, n := range names(a.tab()) {
		if n == ".near-notes" {
			t.Fatal("notes file must be hidden from listing")
		}
	}
	// Rename keeps the note.
	key(a, "Shift+F6")
	a.top().(*InputDialog).Edit.Set("alpha2")
	key(a, "Enter")
	notes = loadNotes(data, a.cfg.NotesFile)
	if notes["alpha2"] == "" || notes["alpha"] != "" {
		t.Fatalf("note did not follow rename: %v", notes)
	}
	// Empty note removes it and the file.
	key(a, "Ctrl+N")
	d := a.top().(*TextDialog)
	d.Lines = [][]rune{{}}
	d.Row, d.Col = 0, 0
	key(a, "Ctrl+S")
	if _, err := os.Stat(filepath.Join(data, ".near-notes")); !os.IsNotExist(err) {
		t.Fatal("notes file should be gone")
	}
}

func TestSearchIndex(t *testing.T) {
	a, data := testApp(t)
	a.index.Configure(a.cfg, nil)
	a.index.Build()
	if a.index.Count() != 9 {
		t.Fatalf("indexed %d, want 9", a.index.Count())
	}
	key(a, "Alt+F7")
	typeText(a, "deep")
	d := a.top().(*SearchDialog)
	waitSearch(t, a)
	if len(d.results) != 1 {
		t.Fatalf("results = %v", d.results)
	}
	key(a, "Enter")
	if a.tab().Path != filepath.Join(data, "gamma", "inner") || a.tab().Current().Name != "deep.md" {
		t.Fatalf("path=%s cur=%v", a.tab().Path, a.tab().Current())
	}
	key(a, "Ctrl+F")
	typeText(a, "file*.txt")
	waitSearch(t, a)
	if d := a.top().(*SearchDialog); len(d.results) != 3 {
		t.Fatalf("glob results = %v", d.results)
	}
	key(a, "Ctrl+U")
	typeText(a, "inner/")
	waitSearch(t, a)
	if d := a.top().(*SearchDialog); len(d.results) != 1 {
		t.Fatalf("path results = %v", d.results)
	}
	key(a, "Esc")
}

func TestQuickSearchAndMkdirDelete(t *testing.T) {
	a, data := testApp(t)
	key(a, "Ctrl+S")
	typeText(a, "pho")
	if a.tab().Current().Name != "photo.jpg" {
		t.Fatalf("quick search cursor on %s", a.tab().Current().Name)
	}
	key(a, "Esc")
	if a.qsActive {
		t.Fatal("quick search should be closed")
	}
	key(a, "F7")
	typeText(a, "newdir")
	key(a, "Enter")
	if !isDir(filepath.Join(data, "newdir")) || a.tab().Current().Name != "newdir" {
		t.Fatal("mkdir failed")
	}
	key(a, "F8")
	key(a, "Enter")
	if isDir(filepath.Join(data, "newdir")) {
		t.Fatal("delete failed")
	}
}

func TestCopyMove(t *testing.T) {
	a, data := testApp(t)
	other := filepath.Join(filepath.Dir(data), "dest")
	os.MkdirAll(other, 0o755)
	if err := a.navigate(a.other(), other, ""); err != nil {
		t.Fatal(err)
	}
	a.tab().SetCursorOn("gamma")
	key(a, "Insert")
	a.tab().SetCursorOn("photo.jpg")
	key(a, "Insert")
	key(a, "F5")
	key(a, "Enter")
	waitTransfers(t, a)
	if _, err := os.Stat(filepath.Join(other, "gamma", "inner", "deep.md")); err != nil {
		t.Fatal("copy of tree failed:", err)
	}
	if _, err := os.Stat(filepath.Join(other, "photo.jpg")); err != nil {
		t.Fatal("copy of file failed:", err)
	}
	a.tab().SetCursorOn("beta")
	key(a, "F6")
	key(a, "Enter")
	waitTransfers(t, a)
	if isDir(filepath.Join(data, "beta")) || !isDir(filepath.Join(other, "beta")) {
		t.Fatal("move failed")
	}
}

func TestMenusAndPlaceholders(t *testing.T) {
	a, data := testApp(t)
	a.tab().SetCursorOn("photo.jpg")
	got := a.expandPlaceholders("{file} {name} {ext} {dir}")
	if !strings.Contains(got, "photo.jpg") || !strings.Contains(got, "photo") || !strings.Contains(got, "jpg") || !strings.Contains(got, data) {
		t.Fatalf("placeholders = %s", got)
	}
	key(a, "F2")
	d, ok := a.top().(*ListDialog)
	if !ok || d.Title != "Quick launch" {
		t.Fatalf("quick launch menu not open: %T", a.top())
	}
	key(a, "t") // Tools submenu hotkey
	if d2, ok := a.top().(*ListDialog); !ok || d2.Title != "Tools" {
		t.Fatalf("submenu not opened: %T", a.top())
	}
	key(a, "Esc")
	key(a, "F9")
	if d3, ok := a.top().(*ListDialog); !ok || d3.Title != "Menus" {
		t.Fatalf("menus list not opened")
	}
	key(a, "Esc")
	key(a, "Shift+F2")
	if d4, ok := a.top().(*ListDialog); !ok || d4.Title != "Tools" {
		t.Fatalf("menu key binding failed")
	}
}

func TestHelpAndViewer(t *testing.T) {
	a, _ := testApp(t)
	key(a, "F1")
	if _, ok := a.top().(*Viewer); !ok {
		t.Fatal("help viewer not open")
	}
	if !strings.Contains(screenText(a), "up_dir") {
		t.Fatal("help content missing")
	}
	key(a, "Esc")
	a.tab().SetCursorOn("photo.jpg")
	key(a, "F3")
	if !strings.Contains(screenText(a), "hello photo.jpg") {
		t.Fatal("viewer content missing")
	}
	key(a, "Esc")
}

func TestDefaultConfigParsesOnEveryPlatform(t *testing.T) {
	for _, goos := range []string{"windows", "linux", "darwin"} {
		var cfg Config
		if _, err := toml.Decode(defaultConfigTextFor(goos), &cfg); err != nil {
			t.Fatalf("%s default config: %v", goos, err)
		}
		if len(cfg.Menus) != 2 || cfg.Menus[0].Name != "Quick launch" || len(cfg.Keys) < 40 {
			t.Fatalf("%s default config incomplete: %d menus, %d keys", goos, len(cfg.Menus), len(cfg.Keys))
		}
	}
}

func TestCommandLineFocus(t *testing.T) {
	a, data := testApp(t)
	a.draw()
	// Printable keys go into the command line; the panel cursor stays put.
	typeText(a, "echo hi")
	if a.cmd.String() != "echo hi" || a.tab().Cursor != 0 {
		t.Fatalf("cmd=%q cursor=%d", a.cmd.String(), a.tab().Cursor)
	}
	// Space while typing is a character, not "select".
	typeText(a, " there")
	if a.cmd.String() != "echo hi there" {
		t.Fatalf("cmd=%q", a.cmd.String())
	}
	if n, _ := a.tab().SelectedStats(); n != 0 {
		t.Fatal("space must not select while typing")
	}
	// Tab still switches panels while text is present; Up/Down still navigate.
	key(a, "Tab")
	if a.active != 1 {
		t.Fatal("Tab did not switch panels")
	}
	key(a, "Tab")
	key(a, "Down")
	if a.tab().Cursor != 1 {
		t.Fatal("Down must still move the panel cursor")
	}
	// Esc clears the line instead of going up.
	key(a, "Esc")
	if a.cmd.String() != "" || a.tab().Path != data {
		t.Fatalf("Esc: cmd=%q path=%s", a.cmd.String(), a.tab().Path)
	}
	// Prompt and cursor are drawn.
	if !strings.Contains(screenText(a), ">") {
		t.Fatal("prompt missing")
	}
	// Bound characters act when the line is empty.
	key(a, "+")
	if _, ok := a.top().(*InputDialog); !ok {
		t.Fatal("+ on empty line should open select dialog")
	}
	key(a, "Esc")
}

func TestCommandLineInsertAndCd(t *testing.T) {
	a, data := testApp(t)
	a.tab().SetCursorOn("photo.jpg")
	typeText(a, "cat")
	key(a, "Ctrl+Enter")
	if a.cmd.String() != "cat photo.jpg " {
		t.Fatalf("insert_name: %q", a.cmd.String())
	}
	key(a, "Ctrl+J")
	if a.cmd.String() != "cat photo.jpg photo.jpg " {
		t.Fatalf("Ctrl+J fallback: %q", a.cmd.String())
	}
	key(a, "Esc")
	key(a, "Alt+Enter")
	if !strings.Contains(a.cmd.String(), filepath.Join(data, "photo.jpg")) {
		t.Fatalf("insert_path: %q", a.cmd.String())
	}
	key(a, "Esc")
	a.insertIntoCmd("with space.txt")
	if !strings.Contains(a.cmd.String(), `with space.txt`) || a.cmd.String() == "with space.txt " {
		t.Fatalf("names with spaces must be quoted: %q", a.cmd.String())
	}
	key(a, "Esc")

	typeText(a, "cd gamma")
	key(a, "Enter")
	if a.tab().Path != filepath.Join(data, "gamma") || a.cmd.String() != "" {
		t.Fatalf("cd: path=%s cmd=%q", a.tab().Path, a.cmd.String())
	}
	typeText(a, "cd ..")
	key(a, "Enter")
	if a.tab().Path != data {
		t.Fatalf("cd ..: path=%s", a.tab().Path)
	}
	if len(a.history) != 2 || a.history[1] != "cd .." {
		t.Fatalf("history = %v", a.history)
	}
	key(a, "Ctrl+E")
	if a.cmd.String() != "cd .." {
		t.Fatalf("history_prev: %q", a.cmd.String())
	}
	key(a, "Ctrl+E")
	if a.cmd.String() != "cd gamma" {
		t.Fatalf("history_prev twice: %q", a.cmd.String())
	}
	key(a, "Ctrl+X")
	key(a, "Ctrl+X")
	if a.cmd.String() != "" {
		t.Fatalf("history_next past end: %q", a.cmd.String())
	}
	key(a, "Alt+F8")
	if d, ok := a.top().(*ListDialog); !ok || d.Title != "Command history" {
		t.Fatal("history dialog missing")
	}
	key(a, "Shift+Enter")
	if a.cmd.String() != "cd .." {
		t.Fatalf("history pick: %q", a.cmd.String())
	}
	// Prompt follows the active panel.
	key(a, "Esc")
	key(a, "Tab")
	if !strings.HasPrefix(a.prompt(), fitRight(a.tab().Path, a.w/3)) {
		t.Fatalf("prompt %q should show %s", a.prompt(), a.tab().Path)
	}
}

func TestAltTypingQuickSearch(t *testing.T) {
	a, _ := testApp(t)
	// Alt+p jumps to photo.jpg, further Alt+letters refine, Esc leaves.
	key(a, "Alt+f")
	if !a.qsActive || a.tab().Current().Name != "file1.txt" {
		t.Fatalf("Alt+f: active=%v cur=%s", a.qsActive, a.tab().Current().Name)
	}
	key(a, "Alt+i")
	key(a, "Alt+l")
	key(a, "Alt+e")
	key(a, "1")
	key(a, "0")
	if a.tab().Current().Name != "file10.txt" || string(a.qsText) != "file10" {
		t.Fatalf("mixed Alt/plain typing: cur=%s qs=%q", a.tab().Current().Name, string(a.qsText))
	}
	if a.cmd.String() != "" {
		t.Fatalf("quick search must not leak into the command line: %q", a.cmd.String())
	}
	key(a, "Esc")
	if a.qsActive {
		t.Fatal("Esc should end the quick search")
	}
	// Bound Alt combinations still act (Alt+1 is tab 1).
	key(a, "Ctrl+T")
	key(a, "Alt+1")
	if a.panel().Active != 0 || a.qsActive {
		t.Fatal("Alt+1 must switch tabs, not search")
	}
	// Alt typing works while the command line holds text, and leaves it alone.
	typeText(a, "cat ")
	key(a, "Alt+g")
	if a.tab().Current().Name != "gamma" || a.cmd.String() != "cat " {
		t.Fatalf("Alt+g with command text: cur=%s cmd=%q", a.tab().Current().Name, a.cmd.String())
	}
}

func TestHidePanels(t *testing.T) {
	a, data := testApp(t)
	a.draw()
	key(a, "Ctrl+O")
	if !a.hideAll || strings.Contains(screenText(a), "Date") {
		t.Fatal("Ctrl+O should hide both panels")
	}
	if !strings.Contains(screenText(a), ">") {
		t.Fatal("command line must stay visible")
	}
	key(a, "Ctrl+O")
	if a.hideAll || !strings.Contains(screenText(a), "1:data") {
		t.Fatal("Ctrl+O again should show the panels")
	}
	key(a, "Ctrl+P") // hides the inactive (right) panel
	txt := screenText(a)
	if !a.hidden[1] || a.active != 0 {
		t.Fatalf("Ctrl+P: hidden=%v active=%d", a.hidden, a.active)
	}
	if strings.Count(txt, "│ Size ") != 1 || !strings.Contains(txt, "1:data") {
		t.Fatalf("right panel still drawn:\n%s", txt)
	}
	// The left panel keeps its half width; the right half is blank.
	firstLine := []rune(strings.SplitN(txt, "\n", 2)[0])
	if string(firstLine[a.w/2-1]) != "╗" || strings.TrimSpace(string(firstLine[a.w/2:])) != "" {
		t.Fatalf("left panel should keep half width with a blank right half: %q", string(firstLine))
	}
	key(a, "Tab")
	if a.active != 0 {
		t.Fatal("Tab must not switch to a hidden panel")
	}
	if a.tab().Path != data {
		t.Fatal("active tab should be the left one")
	}
	key(a, "Ctrl+P")
	if a.hidden[1] {
		t.Fatal("Ctrl+P again should show the right panel")
	}
	// From the right panel, Ctrl+P hides the left one and Ctrl+F2 the right one.
	key(a, "Tab")
	key(a, "Ctrl+P")
	if !a.hidden[0] || a.hidden[1] || a.active != 1 {
		t.Fatalf("Ctrl+P from the right: hidden=%v active=%d", a.hidden, a.active)
	}
	key(a, "Ctrl+P")
	key(a, "Ctrl+F2")
	if !a.hidden[1] || a.active != 0 {
		t.Fatalf("Ctrl+F2: hidden=%v active=%d", a.hidden, a.active)
	}
}

func TestQuickSearchEnterOpens(t *testing.T) {
	a, data := testApp(t)
	key(a, "Alt+g")
	key(a, "Alt+a")
	key(a, "Enter")
	if a.qsActive || a.tab().Path != filepath.Join(data, "gamma") {
		t.Fatalf("Enter should enter the matched directory: active=%v path=%s", a.qsActive, a.tab().Path)
	}
	// Esc still just leaves the search without opening anything.
	key(a, "Alt+i") // "inner" inside gamma
	key(a, "Esc")
	if a.qsActive || a.tab().Path != filepath.Join(data, "gamma") || a.tab().Current().Name != "inner" {
		t.Fatalf("Esc: active=%v path=%s cur=%v", a.qsActive, a.tab().Path, a.tab().Current())
	}
}

func TestPanelResize(t *testing.T) {
	a, _ := testApp(t)
	a.draw()
	full := a.h - bottomRows
	if a.panelHeight(0) != full || a.panelHeight(1) != full {
		t.Fatalf("initial heights %d %d, want %d", a.panelHeight(0), a.panelHeight(1), full)
	}
	// Ctrl+Up moves the bottom edge up (shorter), Ctrl+Down grows them back, never past full.
	key(a, "Ctrl+Up")
	key(a, "Ctrl+Up")
	if a.panelHeight(0) != full-2 || a.panelHeight(1) != full-2 {
		t.Fatalf("after 2x Ctrl+Up: %d %d", a.panelHeight(0), a.panelHeight(1))
	}
	txt := screenText(a)
	lines := strings.Split(txt, "\n")
	if !strings.HasPrefix(lines[full-3], "╚") || strings.TrimSpace(lines[full-2]) != "" || strings.TrimSpace(lines[full-1]) != "" {
		t.Fatalf("rows below shortened panels should be blank:\n%s", txt)
	}
	for i := 0; i < 5; i++ {
		key(a, "Ctrl+Down")
	}
	if a.panelHeight(0) != full || a.heights[0] != 0 {
		t.Fatalf("Ctrl+Down must stop at full height: %d", a.panelHeight(0))
	}
	// Ctrl+Shift+Up only affects the active (left) panel.
	key(a, "Ctrl+Shift+Up")
	if a.panelHeight(0) != full-1 || a.panelHeight(1) != full {
		t.Fatalf("Ctrl+Shift+Down: %d %d", a.panelHeight(0), a.panelHeight(1))
	}
	// Minimum height keeps the frame and one row; page size follows the height.
	for i := 0; i < 100; i++ {
		key(a, "Ctrl+Shift+Up")
	}
	if a.panelHeight(0) != minPanelHeight || a.listRows(0) != 1 {
		t.Fatalf("minimum: height=%d rows=%d", a.panelHeight(0), a.listRows(0))
	}
	// The divider moves with Ctrl+Left/Right while the command line is empty.
	mid := a.w / 2
	key(a, "Ctrl+Right")
	key(a, "Ctrl+Right")
	if a.panelSplit() != mid+2 {
		t.Fatalf("split = %d, want %d", a.panelSplit(), mid+2)
	}
	first := []rune(strings.SplitN(screenText(a), "\n", 2)[0])
	if string(first[mid+2]) != "╔" {
		t.Fatalf("right panel should start at column %d: %q", mid+2, string(first))
	}
	for i := 0; i < 200; i++ {
		key(a, "Ctrl+Left")
	}
	if a.panelSplit() != 20 {
		t.Fatalf("split should stop at the minimum width, got %d", a.panelSplit())
	}
	// With text in the command line, Ctrl+Left edits instead of resizing.
	before := a.panelSplit()
	typeText(a, "echo x")
	key(a, "Ctrl+Left")
	if a.panelSplit() != before || a.cmd.Cur != 5 {
		t.Fatalf("Ctrl+Left while typing: split=%d cur=%d", a.panelSplit(), a.cmd.Cur)
	}
	// Sizes survive a session round trip.
	key(a, "Esc")
	a.saveSession()
	st := loadState(a.cfg.dir)
	if st == nil || st.Split != a.split || len(st.Heights) != 2 || st.Heights[0] != a.heights[0] {
		t.Fatalf("state = %+v", st)
	}
}

func TestPanelDefaultSize(t *testing.T) {
	a, _ := testApp(t)
	a.draw()
	key(a, "Ctrl+Down")
	key(a, "Ctrl+Right")
	a.doAction("panels_default_width")
	a.doAction("panels_default_height")
	if a.split != 0 || a.heights != [2]int{} {
		t.Fatalf("defaults not restored: split=%d heights=%v", a.split, a.heights)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func runWatcherTest(t *testing.T, mk func() (fsWatcher, error)) {
	a, data := testApp(t)
	a.index.Configure(a.cfg, nil)
	a.index.newWatcher = mk
	a.index.Build()
	a.index.StartWatch()
	defer a.index.Close()
	if st := a.index.WatchStatus(); !strings.HasPrefix(st, "watching") {
		t.Fatalf("watch status = %q", st)
	}
	found := func(q string, n int) func() bool {
		return func() bool { _, total, _ := a.index.Search(q, 10); return total == n }
	}
	// New file.
	os.WriteFile(filepath.Join(data, "fresh.log"), []byte("x"), 0o644)
	waitFor(t, "fresh.log indexed", found("fresh.log", 1))
	// New directory tree created in one go.
	os.MkdirAll(filepath.Join(data, "newdir", "sub"), 0o755)
	os.WriteFile(filepath.Join(data, "newdir", "sub", "leaf.txt"), []byte("x"), 0o644)
	waitFor(t, "leaf.txt indexed", found("leaf.txt", 1))
	// A file inside a directory that appeared after the watch started
	// (exercises per-directory watch registration on the portable backend).
	time.Sleep(500 * time.Millisecond)
	os.WriteFile(filepath.Join(data, "newdir", "sub", "later.txt"), []byte("x"), 0o644)
	waitFor(t, "later.txt indexed", found("later.txt", 1))
	// Rename.
	os.Rename(filepath.Join(data, "fresh.log"), filepath.Join(data, "renamed.log"))
	waitFor(t, "rename applied", func() bool {
		return found("fresh.log", 0)() && found("renamed.log", 1)()
	})
	// Remove a whole tree.
	os.RemoveAll(filepath.Join(data, "newdir"))
	waitFor(t, "tree removed", func() bool {
		return found("leaf.txt", 0)() && found("later.txt", 0)() && found("newdir", 0)()
	})
	// Count stays consistent with a fresh build.
	live := a.index.Count()
	fresh := newIndex()
	fresh.Configure(a.cfg, nil)
	fresh.Build()
	if live != fresh.Count() {
		t.Fatalf("live count %d, fresh build %d", live, fresh.Count())
	}
	// Excluded directories are ignored.
	os.MkdirAll(filepath.Join(data, "node_modules", "pkg"), 0o755)
	os.WriteFile(filepath.Join(data, "node_modules", "pkg", "index.js"), []byte("x"), 0o644)
	time.Sleep(700 * time.Millisecond)
	if _, total, _ := a.index.Search("index.js", 10); total != 0 {
		t.Fatal("excluded directory content was indexed")
	}
}

func TestWatcherNative(t *testing.T) {
	runWatcherTest(t, newWatcher)
}

func TestWatcherPortable(t *testing.T) {
	runWatcherTest(t, newFsnotifyWatcher)
}

func TestIndexCacheRoundTrip(t *testing.T) {
	a, _ := testApp(t)
	a.index.Configure(a.cfg, nil)
	a.index.Build()
	if err := a.index.saveCache(); err != nil {
		t.Fatal(err)
	}
	other := newIndex()
	other.Configure(a.cfg, nil)
	if err := other.LoadCache(); err != nil {
		t.Fatal(err)
	}
	if other.Count() != a.index.Count() {
		t.Fatalf("cache count %d, want %d", other.Count(), a.index.Count())
	}
	if res, _, _ := other.Search("deep", 10); len(res) != 1 {
		t.Fatalf("search after cache load: %v", res)
	}
}

func applyOps(a, b []string, ops []diffOp) []string {
	var out []string
	for _, op := range ops {
		switch op.kind {
		case '=':
			out = append(out, a[op.a])
		case '+':
			out = append(out, b[op.b])
		}
	}
	return out
}

func TestMyersDiff(t *testing.T) {
	cases := [][2][]string{
		{{"a", "b", "c"}, {"a", "b", "c"}},
		{{}, {"x", "y"}},
		{{"x", "y"}, {}},
		{{"a", "b", "c", "d"}, {"a", "x", "c", "d", "e"}},
		{{"1", "2", "3", "4", "5"}, {"3", "4", "5", "1", "2"}},
		{{"same"}, {"other"}},
	}
	for _, c := range cases {
		ops := myersDiff(c[0], c[1])
		got := applyOps(c[0], c[1], ops)
		if strings.Join(got, "|") != strings.Join(c[1], "|") {
			t.Fatalf("diff %v -> %v produced %v (ops %v)", c[0], c[1], got, ops)
		}
	}
	rows := diffRows([]string{"a", "b", "c", "d"}, []string{"a", "x", "c", "d", "e"})
	kinds := ""
	for _, r := range rows {
		kinds += string(r.kind)
	}
	if kinds != "=~==+" {
		t.Fatalf("row kinds = %q", kinds)
	}
	if rows[1].left != "b" || rows[1].right != "x" || rows[1].lno != 2 || rows[1].rno != 2 {
		t.Fatalf("changed row = %+v", rows[1])
	}
}

func TestFolderHistory(t *testing.T) {
	a, data := testApp(t)
	key(a, "Down")
	key(a, "Enter") // alpha
	key(a, "Esc")
	a.tab().SetCursorOn("gamma")
	key(a, "Enter") // gamma
	key(a, "Esc")
	if n := len(a.dirHistory); n < 3 || a.dirHistory[n-1] != data {
		t.Fatalf("dir history = %v", a.dirHistory)
	}
	key(a, "Alt+F12")
	d, ok := a.top().(*ListDialog)
	if !ok || d.Title != "Folders history" {
		t.Fatalf("expected folders history, got %T", a.top())
	}
	if d.Items[0].Label != data || d.Items[1].Label != filepath.Join(data, "gamma") {
		t.Fatalf("history order: %s, %s", d.Items[0].Label, d.Items[1].Label)
	}
	key(a, "Down")
	key(a, "Delete")
	if len(d.Items) != len(a.dirHistory) || a.dirHistory[len(a.dirHistory)-1] != data {
		t.Fatalf("delete: items=%d history=%v", len(d.Items), a.dirHistory)
	}
	for i, h := range a.dirHistory {
		if h == filepath.Join(data, "gamma") {
			t.Fatalf("gamma still in history at %d", i)
		}
	}
	key(a, "Down") // alpha
	key(a, "Enter")
	if a.tab().Path != filepath.Join(data, "alpha") {
		t.Fatalf("history navigation: %s", a.tab().Path)
	}
	key(a, "Alt+F12")
	key(a, "Down")
	key(a, "Shift+Enter")
	if a.other().Path != data {
		t.Fatalf("Shift+Enter should move the other panel: %s", a.other().Path)
	}
	a.saveSession()
	if st := loadState(a.cfg.dir); st == nil || len(st.DirHistory) != len(a.dirHistory) {
		t.Fatal("dir history not persisted")
	}
}

func TestFileHistory(t *testing.T) {
	a, data := testApp(t)
	a.tab().SetCursorOn("photo.jpg")
	key(a, "F3")
	key(a, "Esc")
	if len(a.fileHistory) != 1 || a.fileHistory[0] != filepath.Join(data, "photo.jpg") {
		t.Fatalf("file history = %v", a.fileHistory)
	}
	key(a, "Alt+F11")
	if d, ok := a.top().(*ListDialog); !ok || d.Title != "View and edit history" {
		t.Fatalf("expected file history dialog, got %T", a.top())
	}
	key(a, "Enter")
	if _, ok := a.top().(*Viewer); !ok {
		t.Fatalf("Enter should open the viewer, got %T", a.top())
	}
	key(a, "Esc")
	key(a, "Alt+F11")
	key(a, "Ctrl+Enter")
	if a.tab().Current().Name != "photo.jpg" {
		t.Fatal("Ctrl+Enter should place the cursor on the file")
	}
}

func TestCommandHistoryDialog(t *testing.T) {
	a, data := testApp(t)
	typeText(a, "cd gamma")
	key(a, "Enter")
	typeText(a, "cd ..")
	key(a, "Enter")
	key(a, "Alt+F8")
	d, ok := a.top().(*ListDialog)
	if !ok || d.Title != "Command history" || d.Items[0].Label != "cd .." {
		t.Fatalf("history dialog: %T %+v", a.top(), d)
	}
	key(a, "Down") // cd gamma
	key(a, "Shift+Enter")
	if a.cmd.String() != "cd gamma" || len(a.dialogs) != 0 {
		t.Fatalf("Shift+Enter should edit: cmd=%q dialogs=%d", a.cmd.String(), len(a.dialogs))
	}
	key(a, "Esc")
	key(a, "Alt+F8")
	key(a, "Down")
	key(a, "Enter") // runs "cd gamma" (handled internally, no shell)
	if a.tab().Path != filepath.Join(data, "gamma") {
		t.Fatalf("Enter should run the command: %s", a.tab().Path)
	}
	key(a, "Alt+F8")
	key(a, "Delete")
	if len(a.history) != 1 {
		t.Fatalf("Delete should remove the entry: %v", a.history)
	}
	key(a, "Esc")
}

func TestCompareDirs(t *testing.T) {
	a, data := testApp(t)
	other := filepath.Join(filepath.Dir(data), "other")
	os.MkdirAll(filepath.Join(other, "alpha"), 0o755)
	os.WriteFile(filepath.Join(other, "file1.txt"), []byte("hello file1.txt"), 0o644) // same size, same content
	os.WriteFile(filepath.Join(other, "file2.txt"), []byte("hello file2.txt"), 0o644) // same size, content differs by date only? (identical)
	os.WriteFile(filepath.Join(other, "photo.jpg"), []byte("different size"), 0o644)  // size differs
	os.WriteFile(filepath.Join(other, "only-right.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(other, "file10.txt"), []byte("hello fileXX.txt"), 0o644) // same size, different content
	// Equalise times so only size/content decide.
	for _, n := range []string{"file1.txt", "file2.txt", "photo.jpg", "file10.txt"} {
		st, _ := os.Stat(filepath.Join(data, n))
		os.Chtimes(filepath.Join(other, n), st.ModTime(), st.ModTime())
	}
	if err := a.navigate(a.other(), other, ""); err != nil {
		t.Fatal(err)
	}
	key(a, "Shift+F3")
	if d, ok := a.top().(*ListDialog); !ok || d.Title != "Compare" {
		t.Fatalf("expected compare menu, got %T", a.top())
	}
	key(a, "d")
	left, right := a.tab().Selected, a.other().Selected
	wantLeft := map[string]bool{"beta": true, "gamma": true, "photo.jpg": true}
	wantRight := map[string]bool{"only-right.md": true, "photo.jpg": true}
	if len(left) != len(wantLeft) || len(right) != len(wantRight) {
		t.Fatalf("size/date compare: left=%v right=%v", left, right)
	}
	for n := range wantLeft {
		if !left[n] {
			t.Fatalf("left should mark %s", n)
		}
	}
	key(a, "Shift+F3")
	key(a, "c")
	if !a.tab().Selected["file10.txt"] || !a.other().Selected["file10.txt"] || a.tab().Selected["file1.txt"] {
		t.Fatalf("content compare: left=%v right=%v", a.tab().Selected, a.other().Selected)
	}
	// Newer file on one side is marked on that side only.
	a.tab().Selected, a.other().Selected = map[string]bool{}, map[string]bool{}
	os.Chtimes(filepath.Join(other, "file1.txt"), time.Now().Add(time.Hour), time.Now().Add(time.Hour))
	a.reloadAll()
	key(a, "Shift+F3")
	key(a, "d")
	if a.tab().Selected["file1.txt"] || !a.other().Selected["file1.txt"] {
		t.Fatal("newer file should be marked on its own side only")
	}
}

func TestDiffViewer(t *testing.T) {
	a, data := testApp(t)
	other := filepath.Join(filepath.Dir(data), "other")
	os.MkdirAll(other, 0o755)
	os.WriteFile(filepath.Join(data, "left.txt"), []byte("one\ntwo\nthree\nfour\n"), 0o644)
	os.WriteFile(filepath.Join(other, "right.txt"), []byte("one\n2\nthree\nfour\nfive\n"), 0o644)
	a.reloadAll()
	a.navigate(a.other(), other, "right.txt")
	a.tab().SetCursorOn("left.txt")
	key(a, "Shift+F3")
	key(a, "f")
	v, ok := a.top().(*DiffViewer)
	if !ok {
		t.Fatalf("expected diff viewer, got %T", a.top())
	}
	if v.changes != 2 {
		t.Fatalf("changes = %d", v.changes)
	}
	txt := screenText(a)
	if !strings.Contains(txt, "~ two") || !strings.Contains(txt, "~ 2") || !strings.Contains(txt, "+ five") {
		t.Fatalf("diff not rendered:\n%s", txt)
	}
	key(a, "n")
	if v.top != 1 {
		t.Fatalf("n should jump to the first difference, top=%d", v.top)
	}
	key(a, "n")
	if v.top != 4 {
		t.Fatalf("n should jump to the next difference, top=%d", v.top)
	}
	key(a, "Esc")
	if len(a.dialogs) != 0 {
		t.Fatal("viewer should close")
	}
}

func TestPanelLooksLikeFar(t *testing.T) {
	a, _ := testApp(t)
	a.draw()
	txt := screenText(a)
	lines := strings.Split(txt, "\n")
	hdr := lines[2]
	if !strings.Contains(hdr, "Name") || !strings.Contains(hdr, "│ Size ") || !strings.Contains(hdr, "│   Date   │") || !strings.HasPrefix(strings.TrimPrefix(hdr, "║"), "n") {
		t.Fatalf("header: %q", hdr)
	}
	if !strings.Contains(lines[3], "..") || !strings.Contains(lines[3], " Up│") {
		t.Fatalf("up row: %q", lines[3])
	}
	if !strings.Contains(lines[4], "Folder│") || !strings.HasPrefix(lines[4], "║alpha") {
		t.Fatalf("folder row should start right after the border and show Folder: %q", lines[4])
	}
	foot := lines[a.h-bottomRows-1]
	if !strings.Contains(foot, " Bytes: ") || !strings.Contains(foot, ", files: 4, folders: 3 ") {
		t.Fatalf("footer: %q", foot)
	}
	// Footer is centred: roughly as much border on both sides (count in
	// runes, the border characters are multi-byte).
	left := []rune(foot)[:a.w/2]
	l, r := 0, 0
	for i := 1; i < len(left) && left[i] == '═'; i++ {
		l++
	}
	for i := len(left) - 2; i > 0 && left[i] == '═'; i-- {
		r++
	}
	if l < 5 || r < 5 || abs(l-r) > 2 {
		t.Fatalf("footer not centred: left %d right %d in %q", l, r, string(left))
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func TestCopyPath(t *testing.T) {
	a, data := testApp(t)
	a.tab().SetCursorOn("photo.jpg")
	key(a, "Ctrl+Alt+Insert")
	if a.clipboard != filepath.Join(data, "photo.jpg") {
		t.Fatalf("clipboard = %q", a.clipboard)
	}
	a.tab().Cursor = 0 // ".."
	key(a, "Ctrl+Alt+Insert")
	if a.clipboard != data {
		t.Fatalf("clipboard on ..: %q", a.clipboard)
	}
}

func TestEditorBasics(t *testing.T) {
	a, data := testApp(t)
	path := filepath.Join(data, "notes.txt")
	os.WriteFile(path, []byte("first line\r\nsecond line\r\n"), 0o644)
	a.reloadAll()
	a.tab().SetCursorOn("notes.txt")
	key(a, "F4")
	e, ok := a.top().(*Editor)
	if !ok {
		t.Fatalf("expected the built-in editor, got %T", a.top())
	}
	if !e.crlf || len(e.lines) != 2 {
		t.Fatalf("crlf=%v lines=%d", e.crlf, len(e.lines))
	}
	// Type at the end of line 1, Enter keeps indentation, save with F2.
	key(a, "End")
	typeText(a, " edited")
	key(a, "Enter")
	typeText(a, "  indented")
	key(a, "Enter")
	typeText(a, "x")
	if string(e.lines[1]) != "  indented" || string(e.lines[2]) != "  x" {
		t.Fatalf("lines = %q", e.lines)
	}
	key(a, "F2")
	got, _ := os.ReadFile(path)
	want := "first line edited\r\n  indented\r\n  x\r\nsecond line\r\n"
	if string(got) != want {
		t.Fatalf("saved %q, want %q", got, want)
	}
	// Undo removes the typed text step by step; redo brings it back.
	key(a, "Ctrl+Z")
	if string(e.lines[2]) != "  " {
		t.Fatalf("after undo: %q", e.lines[2])
	}
	key(a, "Ctrl+Shift+Z")
	if string(e.lines[2]) != "  x" {
		t.Fatalf("after redo: %q", e.lines[2])
	}
	// Select with Shift+arrows, copy, paste.
	key(a, "Ctrl+Home")
	key(a, "Shift+End")
	key(a, "Ctrl+C")
	if a.clipboard != "first line edited" {
		t.Fatalf("copied %q", a.clipboard)
	}
	key(a, "Ctrl+End")
	key(a, "Enter")
	key(a, "Ctrl+V")
	if string(e.lines[len(e.lines)-1]) != "first line edited" {
		t.Fatalf("paste: %q", e.lines[len(e.lines)-1])
	}
	// Ctrl+Y deletes the line, Ctrl+K cuts to the end.
	key(a, "Ctrl+Y")
	if len(e.lines) != 4 {
		t.Fatalf("after Ctrl+Y: %d lines", len(e.lines))
	}
	key(a, "Ctrl+Home")
	for i := 0; i < 5; i++ {
		key(a, "Right")
	}
	key(a, "Ctrl+K")
	if string(e.lines[0]) != "first" {
		t.Fatalf("after Ctrl+K: %q", e.lines[0])
	}
	// Search and replace all.
	key(a, "Ctrl+F7")
	a.top().(*InputDialog).Edit.Set("line")
	key(a, "Enter")
	a.top().(*InputDialog).Edit.Set("LINE")
	key(a, "Enter")
	if _, ok := a.top().(*ConfirmDialog); !ok {
		t.Fatalf("expected replace prompt, got %T", a.top())
	}
	key(a, "a") // All
	if string(e.lines[3]) != "second LINE" {
		t.Fatalf("replace all: %q", e.lines)
	}
	// Leaving with changes asks; "Don't save" keeps the file as saved before.
	key(a, "Esc")
	if _, ok := a.top().(*ConfirmDialog); !ok {
		t.Fatalf("expected save prompt, got %T", a.top())
	}
	key(a, "d")
	if len(a.dialogs) != 0 {
		t.Fatal("editor should be closed")
	}
	got, _ = os.ReadFile(path)
	if string(got) != want {
		t.Fatal("file changed although Don't save was chosen")
	}
	if len(a.fileHistory) == 0 || a.fileHistory[len(a.fileHistory)-1] != path {
		t.Fatal("edit should be in the file history")
	}
}

func TestEditNewAndExternalFlag(t *testing.T) {
	a, data := testApp(t)
	key(a, "Shift+F4")
	a.top().(*InputDialog).Edit.Set("fresh.md")
	key(a, "Enter")
	e, ok := a.top().(*Editor)
	if !ok || !e.isNew {
		t.Fatalf("expected editor on a new file, got %T", a.top())
	}
	typeText(a, "# title")
	key(a, "F2")
	key(a, "Esc")
	if got, _ := os.ReadFile(filepath.Join(data, "fresh.md")); string(got) != "# title\n" {
		t.Fatalf("new file content %q", got)
	}
	if a.tab().Current().Name != "fresh.md" {
		t.Fatal("cursor should be on the new file")
	}
}

// waitSearch pumps events until the search dialog's results are current.
func waitSearch(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pump(a)
		if d, ok := a.top().(*SearchDialog); ok && !d.busy && !d.pending && d.last == d.edit.String() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("search did not settle")
}
