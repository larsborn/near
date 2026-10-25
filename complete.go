package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// completeCmd completes the path at the end of the command line, as Tab
// does in a shell: a unique match is filled in; with several matches the
// first Tab does nothing and a second Tab on the same text lists them in
// the console.
func (a *App) completeCmd() {
	text := a.cmd.String()
	start := strings.LastIndexAny(text, " \t") + 1
	tok := text[start:]
	quoted := strings.HasPrefix(tok, `"`)
	tok = strings.Trim(tok, `"`)
	cands := a.completions(tok)
	switch len(cands) {
	case 0:
		a.tabPending = ""
	case 1:
		c := cands[0].text
		if quoted || strings.ContainsAny(c, " \t") {
			c = `"` + c + `"`
		}
		a.cmd.Set(text[:start] + c)
		a.tabPending = ""
	default:
		if a.tabPending != text {
			a.tabPending = text
			return
		}
		a.tabPending = ""
		a.listCompletions(cands)
	}
}

type completion struct {
	text string // the completed token
	name string // the entry's name, with a separator after directories
}

// completions returns the entries whose name starts with the last path
// element of tok, each as the token it would complete to.
func (a *App) completions(tok string) []completion {
	i := strings.LastIndexAny(tok, `\/`)
	prefix, base := tok[:i+1], tok[i+1:]
	dir := prefix
	if dir == "" {
		dir = "."
	} else if strings.HasSuffix(dir, ":") {
		dir += string(filepath.Separator)
	}
	dir = expandPath(dir)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(a.tab().Path, dir)
	}
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	fold := pathsEqualFold()
	var out []completion
	for _, d := range des {
		n := d.Name()
		if !(strings.HasPrefix(n, base) || fold && strings.HasPrefix(strings.ToLower(n), strings.ToLower(base))) {
			continue
		}
		shown := n
		if d.IsDir() {
			shown += string(filepath.Separator)
		}
		out = append(out, completion{text: prefix + n, name: shown})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].name) < strings.ToLower(out[j].name) })
	return out
}

// listCompletions prints the candidates in columns, like a shell does.
func (a *App) listCompletions(cands []completion) {
	width := 0
	for _, c := range cands {
		if w := len([]rune(c.name)); w > width {
			width = w
		}
	}
	width += 2
	cols := maxInt(1, a.w/width)
	rows := (len(cands) + cols - 1) / cols
	for r := 0; r < rows; r++ {
		var sb strings.Builder
		for c := 0; c < cols; c++ {
			i := c*rows + r
			if i >= len(cands) {
				break
			}
			sb.WriteString(cands[i].name)
			if c < cols-1 && i+rows < len(cands) {
				sb.WriteString(strings.Repeat(" ", width-len([]rune(cands[i].name))))
			}
		}
		a.console.Println(sb.String(), a.theme.Cmd)
	}
}
