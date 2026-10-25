package main

import (
	"os/exec"
	"runtime"
	"strings"
)

// setClipboard stores text in NEAR's own clipboard and pushes it to the
// system clipboard through the terminal (OSC 52) and, where one exists, a
// native helper program.
func (a *App) setClipboard(text string) {
	a.clipboard = text
	a.screen.SetClipboard([]byte(text))
	if cmd := clipboardWriter(); cmd != nil {
		cmd.Stdin = strings.NewReader(text)
		go func() { _ = cmd.Run() }()
	}
}

// clipboardWriter returns a command that copies stdin to the system
// clipboard, or nil if none is available.
func clipboardWriter() *exec.Cmd {
	var candidates [][]string
	switch runtime.GOOS {
	case "windows":
		candidates = [][]string{{"clip.exe"}}
	case "darwin":
		candidates = [][]string{{"pbcopy"}}
	default:
		candidates = [][]string{{"wl-copy"}, {"xclip", "-selection", "clipboard"}, {"xsel", "--clipboard", "--input"}}
	}
	for _, c := range candidates {
		if p, err := exec.LookPath(c[0]); err == nil {
			return exec.Command(p, c[1:]...)
		}
	}
	return nil
}

// pasteText returns what Ctrl+V should insert: the system clipboard when it
// can be read and holds text, otherwise NEAR's own clipboard.
func (a *App) pasteText() string {
	if a.readClip != nil {
		if s, ok := a.readClip(); ok && s != "" {
			return s
		}
	}
	return a.clipboard
}

// firstLine returns text up to the first line break, without a carriage
// return, for pasting into single-line inputs.
func firstLine(s string) string {
	s = strings.SplitN(s, "\n", 2)[0]
	return strings.TrimRight(s, "\r")
}
