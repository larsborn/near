package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// pump delivers queued screen events (console output, command done) to the app.
func pump(a *App) {
	for a.screen.HasPendingEvent() {
		a.handleEvent(a.screen.PollEvent())
	}
	a.draw()
}

func waitCommand(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		pump(a)
		if a.running == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("command did not finish")
}

func TestConsoleParser(t *testing.T) {
	c := newConsole(tcell.StyleDefault, 100)
	c.Feed([]byte("plain \x1b[31mred\x1b[0m end\r\nsecond\tcol\n"))
	c.Feed([]byte("progress 1\rprogress 2\n"))
	c.Feed([]byte("\x1b]0;title\x07after osc\n"))
	c.Feed([]byte("part"))
	got := c.Text()
	want := "plain red end\nsecond  col\nprogress 2\nafter osc\npart"
	if got != want {
		t.Fatalf("console text = %q, want %q", got, want)
	}
	if len(c.lines[0]) != 3 {
		t.Fatalf("expected 3 styled segments, got %d: %+v", len(c.lines[0]), c.lines[0])
	}
	fg, _, _ := c.lines[0][1].st.Decompose()
	if fg != tcell.PaletteColor(1) {
		t.Fatalf("red segment colour = %v", fg)
	}
	rows := wrapLine(conLine{{"abcdefghij", tcell.StyleDefault}}, 4)
	if len(rows) != 3 || rows[0][0].text != "abcd" || rows[2][0].text != "ij" {
		t.Fatalf("wrap = %+v", rows)
	}
}

func TestConsoleRunAndStay(t *testing.T) {
	a, _ := testApp(t)
	typeText(a, "echo hello console")
	key(a, "Enter")
	if !a.hideAll || a.running == nil {
		t.Fatalf("command should run in the console view: hideAll=%v running=%v", a.hideAll, a.running != nil)
	}
	waitCommand(t, a)
	out := a.console.Text()
	if !strings.Contains(out, "echo hello console") || !strings.Contains(out, "\nhello console") {
		t.Fatalf("console = %q", out)
	}
	// The panels were visible before, so they come back (FAR behaviour),
	// with the tail of the output in a band under them.
	txt0 := screenText(a)
	if a.hideAll || a.cmd.String() != "" || !strings.Contains(txt0, "Name") {
		t.Fatal("panels should be back after the command")
	}
	if !strings.Contains(txt0, "hello console") {
		t.Fatalf("last output should stay visible under the panels:\n%s", txt0)
	}
	if band := a.consoleBand(); band != a.cfg.ConsoleRows || a.panelHeight(0) != a.h-bottomRows-band {
		t.Fatalf("band=%d panel height=%d", band, a.panelHeight(0))
	}
	// Ctrl+Down reclaims band rows for the panels; Ctrl+Up gives them back.
	key(a, "Ctrl+Down")
	key(a, "Ctrl+Down")
	if band := a.consoleBand(); band != a.cfg.ConsoleRows-2 || a.panelHeight(0) != a.h-bottomRows-band {
		t.Fatalf("after Ctrl+Down: band=%d panel height=%d", band, a.panelHeight(0))
	}
	for i := 0; i < 10; i++ {
		key(a, "Ctrl+Down")
	}
	if a.consoleBand() != 0 || a.panelHeight(0) != a.h-bottomRows {
		t.Fatalf("panels should be able to take the whole height: band=%d", a.consoleBand())
	}
	key(a, "Ctrl+Up")
	if a.consoleBand() != 1 {
		t.Fatalf("Ctrl+Up should reopen the band: %d", a.consoleBand())
	}
	// ... and the output waits behind Ctrl+O, with no "press Enter" pause anywhere.
	key(a, "Ctrl+O")
	txt := screenText(a)
	if !a.hideAll || !strings.Contains(txt, "hello console") || strings.Contains(txt, "Press Enter") {
		t.Fatalf("screen:\n%s", txt)
	}
	// Started from the console view, a command leaves the console in place.
	typeText(a, "exit 3")
	key(a, "Enter")
	waitCommand(t, a)
	if !a.hideAll {
		t.Fatal("console view should stay when it was active before the command")
	}
	if !strings.Contains(a.console.Text(), "[exit 3") {
		t.Fatalf("exit status missing: %q", a.console.Text())
	}
	typeText(a, "cd alpha")
	key(a, "Enter")
	if !strings.HasSuffix(a.tab().Path, "alpha") || !strings.Contains(a.console.Text(), ">cd alpha") {
		t.Fatalf("cd in console view: path=%s console=%q", a.tab().Path, a.console.Text())
	}
	// Ctrl+O brings the panels back, and again returns to the console with its content.
	key(a, "Ctrl+O")
	if a.hideAll || !strings.Contains(screenText(a), "Name") {
		t.Fatal("Ctrl+O should show the panels")
	}
	key(a, "Ctrl+O")
	if !a.hideAll || !strings.Contains(screenText(a), "hello console") {
		t.Fatal("Ctrl+O again should show the console with its history")
	}
}

func TestConsoleCtrlC(t *testing.T) {
	a, _ := testApp(t)
	if runtime.GOOS == "windows" {
		typeText(a, "ping -n 30 127.0.0.1")
	} else {
		typeText(a, "sleep 30")
	}
	key(a, "Enter")
	if a.running == nil {
		t.Fatal("command should be running")
	}
	// No prompt and no input while the command runs.
	typeText(a, "ignored")
	key(a, "Enter")
	promptRow := strings.Split(screenText(a), "\n")[a.h-2]
	if a.cmd.String() != "" || strings.TrimSpace(promptRow) != "" {
		key(a, "Ctrl+C")
		waitCommand(t, a)
		t.Fatalf("input must be ignored and the prompt hidden while running: cmd=%q row=%q", a.cmd.String(), promptRow)
	}
	start := time.Now()
	key(a, "Ctrl+C")
	waitCommand(t, a)
	if time.Since(start) > 10*time.Second {
		t.Fatal("Ctrl+C took too long to stop the command")
	}
	if !strings.Contains(a.console.Text(), "^C") {
		t.Fatalf("console = %q", a.console.Text())
	}
}

func TestInteractiveDetection(t *testing.T) {
	a, _ := testApp(t)
	for cmd, want := range map[string]bool{
		"vim file.txt": true, "/usr/bin/vim x": true, "VIM.EXE x": true, "ssh host": true,
		"git status": false, "echo vim": false, "": false,
	} {
		if got := a.isInteractive(cmd); got != want {
			t.Errorf("isInteractive(%q) = %v, want %v", cmd, got, want)
		}
	}
}

func TestConsoleScroll(t *testing.T) {
	a, _ := testApp(t)
	for i := 0; i < 100; i++ {
		a.console.Println("line "+string(rune('A'+i%26)), tcell.StyleDefault)
	}
	a.hideAll = true
	a.draw()
	if !strings.Contains(screenText(a), "line V") { // last line: 99 % 26 = 21 -> V
		t.Fatalf("newest output should be visible:\n%s", screenText(a))
	}
	key(a, "PgUp")
	if a.console.scroll == 0 {
		t.Fatal("PgUp should scroll the console")
	}
	key(a, "End")
	if a.console.scroll != 0 {
		t.Fatal("End should scroll back to the bottom")
	}
}

// Aliases and functions from shell_rc are available to console commands.
func TestConsoleShellRC(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shells only")
	}
	a, root := testApp(t)
	rc := filepath.Join(root, "rc.sh")
	if err := os.WriteFile(rc, []byte("alias zzalias='echo via alias'\nzzfunc() { echo via function; }\ncd /\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.cfg.ShellRC = rc
	for _, sh := range []string{"sh", "bash"} {
		if _, err := exec.LookPath(sh); err != nil {
			continue
		}
		a.cfg.Shell = sh
		typeText(a, "zzalias; zzfunc; pwd")
		key(a, "Enter")
		waitCommand(t, a)
		out := a.console.Text()
		if !strings.Contains(out, "\nvia alias") || !strings.Contains(out, "\nvia function") {
			t.Fatalf("%s: console = %q", sh, out)
		}
		// The rc file's cd must not move the command out of the panel directory.
		if !strings.Contains(out, "\n"+a.tab().Path+"\n") && !strings.HasSuffix(out, "\n"+a.tab().Path) {
			t.Fatalf("%s: command did not run in the panel directory: %q", sh, out)
		}
	}
	// A missing file is reported once and the command still runs.
	a.cfg.ShellRC = filepath.Join(root, "missing.sh")
	typeText(a, "echo still runs")
	key(a, "Enter")
	waitCommand(t, a)
	if !strings.Contains(a.console.Text(), "\nstill runs") {
		t.Fatalf("console = %q", a.console.Text())
	}
}
