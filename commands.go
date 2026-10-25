package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// shQuote quotes a path for the platform shell.
func shQuote(s string) string {
	if runtime.GOOS == "windows" {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// expandPlaceholders replaces {file} {path} {files} {paths} {dir} {other}
// {name} {ext} with values from the active panel, quoted for the shell.
func (a *App) expandPlaceholders(cmd string) string {
	return a.expandFor(cmd, a.tab().Current())
}

// expandFor is expandPlaceholders with an explicit current entry, used when
// a command is run once per selected entry.
func (a *App) expandFor(cmd string, cur *Entry) string {
	t := a.tab()
	var file, name, ext string
	if cur != nil && !cur.IsUp {
		file = cur.Name
		name, ext = file, cur.Ext()
		if ext != "" {
			name = file[:len(file)-len(ext)-1]
		}
	}
	targets := t.Targets()
	var files, paths []string
	for _, e := range targets {
		files = append(files, shQuote(e.Name))
		paths = append(paths, shQuote(filepath.Join(t.Path, e.Name)))
	}
	r := strings.NewReplacer(
		"{file}", shQuote(file),
		"{path}", shQuote(filepath.Join(t.Path, file)),
		"{files}", strings.Join(files, " "),
		"{paths}", strings.Join(paths, " "),
		"{dir}", shQuote(t.Path),
		"{other}", shQuote(a.other().Path),
		"{name}", shQuote(name),
		"{ext}", shQuote(ext),
	)
	return r.Replace(cmd)
}

func (a *App) shell() string {
	if a.cfg.Shell != "" {
		return a.cfg.Shell
	}
	return defaultShell()
}

func (a *App) editor() string {
	for _, e := range []string{a.cfg.Editor, os.Getenv("VISUAL"), os.Getenv("EDITOR")} {
		if strings.TrimSpace(e) != "" {
			return e
		}
	}
	return defaultEditor()
}

// runShell runs a command line through the shell. Unless detach is set the
// terminal is handed over to the command; with wait the user must press
// Enter afterwards so output stays readable.
func (a *App) runShell(cmdline, cwd string, wait, detach bool) {
	cmd := shellCommand(a.shell(), cmdline)
	cmd.Dir = cwd
	if detach {
		if err := cmd.Start(); err != nil {
			a.setErr(err)
			return
		}
		go func() { _ = cmd.Wait() }()
		a.setStatus("Started: " + fitLeft(cmdline, 60))
		return
	}
	if err := a.screen.Suspend(); err != nil {
		a.setErr(err)
		return
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if runtime.GOOS != "windows" {
		fmt.Print("\x1b[0m\x1b[2J\x1b[H")
	}
	fmt.Printf("$ %s\n", cmdline)
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n[NEAR] %v\n", err)
	}
	if wait || err != nil {
		fmt.Print("\n[NEAR] Press Enter to return ")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}
	if err := a.screen.Resume(); err != nil {
		a.setErr(err)
	}
	a.screen.Sync()
	a.reloadAll()
}

// editFile opens path in the built-in editor, or the external one when
// internal_editor is off.
func (a *App) editFile(path string) { a.editFileThen(path, !a.cfg.InternalEditor, nil) }

// editFileThen edits path and runs done when the editor is left. The
// external editor blocks, the built-in one calls done from its close.
func (a *App) editFileThen(path string, external bool, done func()) {
	if !strings.HasPrefix(filepath.Base(path), "near-rename-") {
		a.fileHistory = pushHistory(a.fileHistory, path, historyMax)
	}
	if external {
		a.runShell(a.editor()+" "+shQuote(path), filepath.Dir(path), false, false)
		if done != nil {
			done()
		}
		return
	}
	e, err := newEditor(path, a.cfg.EditorTabSize)
	if err != nil {
		a.setErr(err)
		return
	}
	e.OnClose = func(saved bool) {
		if saved {
			a.reloadAll()
		}
		if done != nil {
			done()
		}
	}
	a.push(e)
}

func (a *App) openWithOS(path string) {
	if err := osOpen(path); err != nil {
		a.setErr(fmt.Errorf("open %s: %v", filepath.Base(path), err))
		return
	}
	a.fileHistory = pushHistory(a.fileHistory, path, historyMax)
	a.setStatus("Opened " + filepath.Base(path))
}
