package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// FAR's "Apply command" (Ctrl+G): a command template is run once for every
// selected entry with FAR's substitutions.
// applyTokens documents FAR's substitutions; the dialog lays them out in
// two aligned columns.
var applyTokens = [][2]string{
	{"!.!", "name with extension"},
	{"!", "name without extension"},
	{"!~", "extension"},
	{"!\\", "directory with separator"},
	{"!/", "directory without separator"},
	{"!:", "drive"},
	{"!&", "all selected names"},
	{"!@!", "file listing all selected paths"},
	{"!!", "a literal !"},
	{"!?Title?Default!", "asks once, before running"},
}

// applyHelpLines formats the token table for a dialog of inner width w:
// the short tokens in two aligned columns, the prompt form on its own line.
func applyHelpLines(w int) []string {
	const tokW, descW = 5, 31
	short := applyTokens[:len(applyTokens)-1]
	perRow := maxInt(1, w/(tokW+descW))
	var lines []string
	for i := 0; i < len(short); i += perRow {
		var b strings.Builder
		for j := i; j < i+perRow && j < len(short); j++ {
			b.WriteString(padRight(short[j][0], tokW))
			b.WriteString(padRight(short[j][1], descW))
		}
		lines = append(lines, strings.TrimRight(b.String(), " "))
	}
	last := applyTokens[len(applyTokens)-1]
	lines = append(lines, last[0]+"  "+last[1])
	return append(lines, "Symbols are inserted as they are (quote them: \"!.!\"); NEAR's {file} {path} {dir} {other} are quoted.")
}

// ApplyDialog asks for the command template.
type ApplyDialog struct {
	edit    *LineEdit
	targets []Entry
	OnOK    func(template string)
}

func (a *App) applyCommand() {
	t := a.tab()
	targets := t.Targets()
	if len(targets) == 0 {
		return
	}
	d := &ApplyDialog{edit: newLineEdit(a.lastApply), targets: targets}
	d.OnOK = func(template string) {
		a.lastApply = template
		a.runApply(template, targets)
	}
	a.push(d)
}

func (d *ApplyDialog) HandleKey(a *App, ev *tcell.EventKey) {
	switch keyName(ev) {
	case "Esc":
		a.pop()
	case "Enter":
		s := strings.TrimSpace(d.edit.String())
		if s == "" {
			return
		}
		a.pop()
		d.OnOK(s)
	default:
		d.edit.HandleKey(ev, a)
	}
}

func (d *ApplyDialog) Draw(a *App) {
	th := a.theme
	w := clampInt(a.w-10, 60, 76)
	help := applyHelpLines(w - 4)
	h := 5 + len(help)
	x, y := a.center(w, h)
	a.drawBox(x, y, w, h, th.Dialog, "Apply command")
	a.print(x+2, y+1, w-4, th.Dialog, "Apply command to "+plural(len(d.targets), "selected entry")+":")
	d.edit.Draw(a, x+2, y+2, w-4, th.InputFocus, true)
	a.separator(x, y+3, w, th.Dialog)
	for i, l := range help {
		a.print(x+2, y+4+i, w-4, th.DialogHint, fitLeft(l, w-4))
	}
}

// runApply resolves !?prompts?, then queues one command per target.
func (a *App) runApply(template string, targets []Entry) {
	a.askPrompts(template, "Apply command", func(cmd string) { a.applyExpanded(cmd, targets) })
}

// askPrompts resolves FAR's !?Title?Default! prompts one after another,
// asking the user for each, then calls then with the completed template.
func (a *App) askPrompts(template, title string, then func(string)) {
	if i := strings.Index(template, "!?"); i >= 0 {
		rest := template[i+2:]
		if end := strings.Index(rest, "!"); end >= 0 {
			spec := rest[:end]
			prompt, def := spec, ""
			if q := strings.Index(spec, "?"); q >= 0 {
				prompt, def = spec[:q], spec[q+1:]
			}
			a.input(title, prompt+":", def, func(v string) {
				a.askPrompts(strings.Replace(template, "!?"+spec+"!", v, 1), title, then)
			})
			return
		}
	}
	then(template)
}

func (a *App) applyExpanded(template string, targets []Entry) {
	t := a.tab()
	var listFile string
	if strings.Contains(template, "!@!") {
		f, err := os.CreateTemp("", "near-list-*.txt")
		if err != nil {
			a.setErr(err)
			return
		}
		for _, e := range targets {
			fmt.Fprintln(f, filepath.Join(t.Path, e.Name))
		}
		f.Close()
		listFile = f.Name()
	}
	var all []string
	for _, e := range targets {
		all = append(all, shQuote(e.Name))
	}
	lines := make([]string, 0, len(targets))
	for _, e := range targets {
		lines = append(lines, a.expandPlaceholders(expandApply(template, t.Path, e, listFile, strings.Join(all, " "))))
	}
	t.Selected = map[string]bool{}
	a.queueCommands(lines, t.Path)
}

// expandApply applies FAR's substitutions for one entry.
func expandApply(tpl, dir string, e Entry, listFile, allNames string) string {
	name := e.Name
	ext := e.Ext()
	base := name
	if ext != "" {
		base = name[:len(name)-len(ext)-1]
	}
	drive := ""
	if runtime.GOOS == "windows" {
		drive = filepath.VolumeName(dir)
	}
	dirSlash := dir
	if !strings.HasSuffix(dirSlash, string(filepath.Separator)) {
		dirSlash += string(filepath.Separator)
	}
	// Longest tokens first so "!.!" is not read as "!" + ".!". As in FAR the
	// symbols are inserted as they are: quote them yourself ("!.!") where a
	// name with spaces matters. !& quotes each name.
	r := strings.NewReplacer(
		"!!", "\x00",
		"!@!", listFile,
		"!.!", name,
		"!&", allNames,
		"!~", ext,
		"!`", ext,
		"!\\", dirSlash,
		"!/", strings.TrimSuffix(dir, string(filepath.Separator)),
		"!:", drive,
		"!", base,
	)
	return strings.ReplaceAll(r.Replace(tpl), "\x00", "!")
}

// queueCommands runs several command lines one after another in the
// console view.
func (a *App) queueCommands(lines []string, cwd string) {
	for _, l := range lines {
		a.cmdQueue = append(a.cmdQueue, queuedCommand{line: l, cwd: cwd})
	}
	a.startQueued()
}

// startQueued starts the next queued command if none is running.
func (a *App) startQueued() {
	if a.running != nil || len(a.cmdQueue) == 0 {
		return
	}
	next := a.cmdQueue[0]
	a.cmdQueue = a.cmdQueue[1:]
	a.runInConsole(next.line, next.cwd)
}

type queuedCommand struct{ line, cwd string }
