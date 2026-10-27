package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

const viewerMaxBytes = 64 << 20

// Viewer shows text (or a hex dump for binary data) full screen.
type Viewer struct {
	Title    string
	Lines    []string
	Top      int
	Left     int
	Wrap     bool // lines longer than the screen continue on the next row (F2), as in FAR's viewer
	WordWrap bool // ... breaking at spaces rather than anywhere (Shift+F2)
	hex      bool
	raw      []byte
	query    string
	cursor   int
}

func newViewer(title, text string, hex bool) *Viewer {
	v := &Viewer{Title: title, hex: hex, Wrap: !hex, WordWrap: true}
	v.Lines = strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	return v
}

func newFileViewer(path string) (*Viewer, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.Size() > viewerMaxBytes {
		return nil, fmt.Errorf("%s is larger than %d MB", st.Name(), viewerMaxBytes>>20)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	probe := data
	if len(probe) > 8192 {
		probe = probe[:8192]
	}
	binary := bytes.IndexByte(probe, 0) >= 0 || !utf8.Valid(probe[:len(probe)-utf8.UTFMax*boolInt(len(probe) == 8192)])
	v := &Viewer{Title: path, raw: data, Wrap: !binary, WordWrap: true}
	if binary {
		v.hex = true
		v.Lines = hexDump(data)
	} else {
		v.Lines = strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	}
	return v, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func hexDump(data []byte) []string {
	var lines []string
	for off := 0; off < len(data); off += 16 {
		end := off + 16
		if end > len(data) {
			end = len(data)
		}
		chunk := data[off:end]
		var b strings.Builder
		fmt.Fprintf(&b, "%08x  ", off)
		for i := 0; i < 16; i++ {
			if i < len(chunk) {
				fmt.Fprintf(&b, "%02x ", chunk[i])
			} else {
				b.WriteString("   ")
			}
			if i == 7 {
				b.WriteByte(' ')
			}
		}
		b.WriteString(" |")
		for _, c := range chunk {
			if c >= 32 && c < 127 {
				b.WriteByte(c)
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteByte('|')
		lines = append(lines, b.String())
	}
	return lines
}

func (v *Viewer) rows(a *App) int { return maxInt(1, a.h-2) }

func (v *Viewer) find(a *App, from int) {
	if v.query == "" {
		return
	}
	q := strings.ToLower(v.query)
	for i := 0; i < len(v.Lines); i++ {
		idx := (from + i) % len(v.Lines)
		if strings.Contains(strings.ToLower(v.Lines[idx]), q) {
			v.cursor = idx
			v.Top = idx
			return
		}
	}
	a.setStatus("Not found: " + v.query)
}

func (v *Viewer) HandleKey(a *App, ev *tcell.EventKey) {
	rows := v.rows(a)
	maxTop := maxInt(0, len(v.Lines)-rows)
	switch keyName(ev) {
	case "Esc", "F3", "F10", "q", "Q":
		a.pop()
	case "Up":
		v.Top = clampInt(v.Top-1, 0, maxTop)
	case "Down":
		v.Top = clampInt(v.Top+1, 0, maxTop)
	case "PgUp", "Backspace":
		v.Top = clampInt(v.Top-rows, 0, maxTop)
	case "PgDn", "Space":
		v.Top = clampInt(v.Top+rows, 0, maxTop)
	case "Home", "Ctrl+Home":
		v.Top = 0
	case "End", "Ctrl+End":
		v.Top = maxTop
	case "Left":
		v.Left = maxInt(0, v.Left-8)
	case "Right":
		v.Left += 8
	case "F2", "W":
		v.Wrap = !v.Wrap
		v.Left = 0
	case "Shift+F2":
		v.WordWrap = !v.WordWrap
	case "F7", "/", "Ctrl+F":
		a.input("Search", "Text:", v.query, func(s string) {
			v.query = s
			v.find(a, v.Top)
		})
	case "n", "N":
		v.find(a, v.cursor+1)
	}
}

// wrapText splits a line into rows of at most w cells; with words set it
// breaks after the last space that fits.
func wrapText(line string, w int, words bool) []string {
	var out []string
	for line != "" {
		part := runewidth.Truncate(line, w, "")
		if part == "" {
			break
		}
		if words && len(part) < len(line) {
			if i := strings.LastIndex(part, " "); i > 0 {
				part = part[:i+1]
			}
		}
		out = append(out, part)
		line = line[len(part):]
	}
	return out
}

func (v *Viewer) Draw(a *App) {
	th := a.theme
	a.fill(0, 0, a.w, a.h, th.Viewer)
	rows := v.rows(a)
	y := 1
	for i := 0; i < rows; i++ {
		idx := v.Top + i
		if idx >= len(v.Lines) {
			break
		}
		line := strings.ReplaceAll(v.Lines[idx], "\t", "    ")
		if v.Wrap {
			for _, part := range wrapText(line, a.w, v.WordWrap) {
				if y > rows {
					break
				}
				a.print(0, y, a.w, th.Viewer, part)
				y++
			}
			if y > rows {
				break
			}
			continue
		}
		// Unwrapped: FAR's « » marks show that the line goes on beyond the
		// screen edge on that side.
		width := runewidth.StringWidth(line)
		if v.Left > 0 {
			line = runewidth.TruncateLeft(line, v.Left, "")
		}
		a.print(0, y, a.w, th.Viewer, line)
		if v.Left > 0 && width > v.Left {
			a.print(0, y, 1, th.Viewer, "«")
		}
		if width-v.Left > a.w {
			a.print(a.w-1, y, 1, th.Viewer, "»")
		}
		y++
	}
	// Header like FAR's viewer: path at the left; mode, encoding, size,
	// column and position at the right.
	pct := 100
	if len(v.Lines) > rows {
		pct = (v.Top + rows) * 100 / len(v.Lines)
	}
	mode := "t"
	if v.hex {
		mode = "x"
	}
	right := fmt.Sprintf("%s │ UTF-8 │ %d │ Col %d │ %d%% ", mode, len(v.raw), v.Left, pct)
	a.fill(0, 0, a.w, 1, th.KeyLabel)
	a.print(1, 0, maxInt(0, a.w-2-runewidth.StringWidth(right)), th.KeyLabel, fitRight(v.Title, maxInt(0, a.w-2-runewidth.StringWidth(right))))
	a.print(a.w-runewidth.StringWidth(right), 0, runewidth.StringWidth(right), th.KeyLabel, right)
	wrap := "off"
	if v.Wrap && v.WordWrap {
		wrap = "words"
	} else if v.Wrap {
		wrap = "on"
	}
	foot := fmt.Sprintf(" line %d/%d  F2 wrap: %s  Shift+F2 word wrap  F7 search  N next  Esc close ", v.Top+1, len(v.Lines), wrap)
	a.fill(0, a.h-1, a.w, 1, th.KeyLabel)
	a.print(0, a.h-1, a.w, th.KeyLabel, fitLeft(foot, a.w))
}
