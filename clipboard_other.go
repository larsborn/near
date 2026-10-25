//go:build !windows

package main

import (
	"os/exec"
	"runtime"
)

// readClipboard returns the system clipboard's text through a helper
// program (pbpaste, wl-paste, xclip or xsel), if one is available.
func readClipboard() (string, bool) {
	var candidates [][]string
	switch runtime.GOOS {
	case "darwin":
		candidates = [][]string{{"pbpaste"}}
	default:
		candidates = [][]string{{"wl-paste", "--no-newline"}, {"xclip", "-selection", "clipboard", "-o"}, {"xsel", "--clipboard", "--output"}}
	}
	for _, c := range candidates {
		p, err := exec.LookPath(c[0])
		if err != nil {
			continue
		}
		out, err := exec.Command(p, c[1:]...).Output()
		if err != nil {
			continue
		}
		return string(out), true
	}
	return "", false
}
