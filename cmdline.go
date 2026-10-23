package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const historyMax = 200

// prompt is the command line prompt for the active panel's directory.
func (a *App) prompt() string {
	p := a.tab().Path
	if home, _ := os.UserHomeDir(); home != "" && (p == home || strings.HasPrefix(p, home+string(filepath.Separator))) {
		p = "~" + p[len(home):]
	}
	max := a.w / 3
	if max < 12 {
		max = 12
	}
	return fitRight(p, max) + ">"
}

func (a *App) drawCmdLine() {
	th := a.theme
	y := a.h - 2
	a.fill(0, y, a.w, 1, th.Cmd)
	if a.running != nil {
		return // no prompt until the command has terminated
	}
	pr := a.prompt()
	pw := a.print(0, y, a.w-4, th.CmdPrompt, pr)
	focused := len(a.dialogs) == 0 && !a.qsActive
	a.cmd.Draw(a, pw, y, a.w-pw, th.Cmd, focused)
}

// insertIntoCmd appends a name (quoted when needed) and a space.
func (a *App) insertIntoCmd(s string) {
	if strings.ContainsAny(s, " \t'\"$&|<>();*?[]{}\\") {
		s = shQuote(s)
	}
	text := a.cmd.String()
	if text != "" && !strings.HasSuffix(text, " ") {
		text += " "
	}
	a.cmd.Set(text + s + " ")
	a.histPos = len(a.history)
}

func (a *App) addHistory(line string) {
	a.history = pushHistory(a.history, line, historyMax)
	a.histPos = len(a.history)
}

func (a *App) historyStep(delta int) {
	if len(a.history) == 0 {
		return
	}
	pos := clampInt(a.histPos+delta, 0, len(a.history))
	a.histPos = pos
	if pos == len(a.history) {
		a.cmd.Set("")
	} else {
		a.cmd.Set(a.history[pos])
	}
}

// runCommandLine executes the command line in the active panel's directory.
// Output is captured into the console view unless the program needs a real
// terminal (interactive_commands) or terminal is set (Shift+Enter).
func (a *App) runCommandLine(terminal bool) {
	line := strings.TrimSpace(a.cmd.String())
	a.cmd.Set("")
	if line == "" {
		return
	}
	a.addHistory(line)
	if a.handleCd(line) {
		if a.hideAll {
			a.console.Println(a.prompt()+line, a.theme.CmdPrompt)
		}
		return
	}
	expanded := a.expandPlaceholders(line)
	if terminal || a.isInteractive(line) {
		a.runShell(expanded, a.tab().Path, a.cfg.CommandWait, false)
		return
	}
	a.runInConsole(expanded, a.tab().Path)
}

// handleCd implements "cd", "cd <dir>" and (on Windows) "d:" without a
// shell, moving the active panel instead.
func (a *App) handleCd(line string) bool {
	t := a.tab()
	var target string
	switch {
	case line == "cd" || line == "cd ~":
		target, _ = os.UserHomeDir()
	case strings.HasPrefix(line, "cd ") || strings.HasPrefix(line, "cd\t"):
		target = strings.Trim(strings.TrimSpace(line[3:]), `"'`)
		if target == "" {
			return false
		}
	case runtime.GOOS == "windows" && len(line) == 2 && line[1] == ':':
		target = strings.ToUpper(line) + `\`
		for _, p := range a.panels {
			for _, tab := range p.Tabs {
				if strings.EqualFold(filepath.VolumeName(tab.Path), line) {
					target = tab.Path
				}
			}
		}
	default:
		return false
	}
	if target == "-" {
		return false
	}
	target = expandPath(target)
	if !filepath.IsAbs(target) {
		target = filepath.Join(t.Path, target)
	}
	if err := a.navigate(t, target, ""); err != nil {
		a.setErr(err)
	}
	return true
}
