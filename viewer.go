package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// Variables rather than constants so that tests can lower them instead of
// writing files of tens of megabytes.
var (
	// viewerMaxBytes is how much of a file the viewer reads. A bigger file
	// is not refused: the beginning is shown and the viewer says so.
	viewerMaxBytes int64 = 64 << 20
	// A hex dump is about four times the size of what it shows, so less of
	// a binary file is read.
	viewerHexMaxBytes int64 = 4 << 20
)

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
	size     int64 // the whole file, which raw may be only the start of
	cut      bool  // the file was too big to read in full
	query    string
	cursor   int
	note     string // what the last search did, shown in the footer
}

func newViewer(title, text string, hex bool) *Viewer {
	v := &Viewer{Title: title, hex: hex, Wrap: !hex, WordWrap: true, size: int64(len(text))}
	v.Lines = strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	return v
}

func newFileViewer(path string) (*Viewer, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Decide from the first few KB how much is worth reading: a huge file
	// is shown from the beginning rather than refused.
	probe := make([]byte, 8192)
	n, err := io.ReadFull(f, probe)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	probe = probe[:n]
	binary := bytes.IndexByte(probe, 0) >= 0 || !utf8.Valid(probe[:len(probe)-utf8.UTFMax*boolInt(len(probe) == 8192)])
	limit := int64(viewerMaxBytes)
	if binary {
		limit = viewerHexMaxBytes
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, limit))
	if err != nil {
		return nil, err
	}
	v := &Viewer{Title: path, raw: data, size: st.Size(), Wrap: !binary, WordWrap: true}
	v.cut = int64(len(data)) < st.Size()
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

// maxTop is the last line that may stand at the top with the screen still
// full. Wrapped lines take several rows each, so it is not simply the line
// count less the row count; getting that wrong put the end of a wrapped
// file out of reach.
func (v *Viewer) maxTop(a *App) int {
	rows := v.rows(a)
	if !v.Wrap {
		return maxInt(0, len(v.Lines)-rows)
	}
	used := 0
	for i := len(v.Lines) - 1; i >= 0; i-- {
		used += len(wrapText(strings.ReplaceAll(v.Lines[i], "\t", "    "), a.w, v.WordWrap))
		if used >= rows {
			return i
		}
	}
	return 0
}

func (v *Viewer) find(a *App, from int) {
	if v.query == "" {
		return
	}
	q := strings.ToLower(v.query)
	hits := 0
	for _, l := range v.Lines {
		if strings.Contains(strings.ToLower(l), q) {
			hits++
		}
	}
	for i := 0; i < len(v.Lines); i++ {
		idx := (from + i) % len(v.Lines)
		if strings.Contains(strings.ToLower(v.Lines[idx]), q) {
			v.cursor = idx
			v.Top = idx
			// The status line is under the viewer, so say it here.
			v.note = fmt.Sprintf("%q on line %d of %d lines with it", v.query, idx+1, hits)
			return
		}
	}
	v.note = fmt.Sprintf("%q is not in this file", v.query)
}

func (v *Viewer) HandleKey(a *App, ev *tcell.EventKey) {
	rows := v.rows(a)
	maxTop := v.maxTop(a)
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
		d := a.input("Search", "Text:", v.query, func(s string) {
			v.query, a.lastContentSearch = s, s
			v.find(a, v.Top)
		})
		// What was searched for last is offered again, selected, so that
		// typing replaces it and Enter repeats it.
		d.Edit.Anchor, d.Edit.Cur = 0, len(d.Edit.Text)
	case "n", "N":
		v.find(a, v.cursor+1)
	}
}

// wrapText splits a line into rows of at most w cells; with words set it
// breaks after the last space that fits.
func wrapText(line string, w int, words bool) []string {
	if line == "" {
		return []string{""} // an empty line is still a line
	}
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

// markMatches paints every occurrence of the search text in a row that has
// just been drawn, so a hit can be seen rather than only jumped to.
func (v *Viewer) markMatches(a *App, row string, y int) {
	if v.query == "" || row == "" {
		return
	}
	low, q := strings.ToLower(row), strings.ToLower(v.query)
	if len(q) == 0 {
		return
	}
	for off := 0; off+len(q) <= len(low); {
		i := strings.Index(low[off:], q)
		if i < 0 {
			return
		}
		i += off
		if i+len(q) > len(row) {
			return
		}
		x := runewidth.StringWidth(row[:i])
		a.print(x, y, a.w-x, a.theme.Viewer.Reverse(true), row[i:i+len(q)])
		off = i + len(q)
	}
}

func (v *Viewer) Draw(a *App) {
	th := a.theme
	a.fill(0, 0, a.w, a.h, th.Viewer)
	rows := v.rows(a)
	y := 1
	// Driven by the screen row, not by a count of source lines: a wrapped
	// line fills several rows, so counting lines would stop short.
	for idx := v.Top; idx < len(v.Lines) && y <= rows; idx++ {
		line := strings.ReplaceAll(v.Lines[idx], "\t", "    ")
		if v.Wrap {
			for _, part := range wrapText(line, a.w, v.WordWrap) {
				if y > rows {
					break
				}
				a.print(0, y, a.w, th.Viewer, part)
				v.markMatches(a, part, y)
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
		v.markMatches(a, line, y)
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
	if m := v.maxTop(a); m > 0 {
		pct = clampInt(v.Top*100/m, 0, 100)
	}
	mode := "t"
	if v.hex {
		mode = "x"
	}
	size := humanSize(v.size)
	if v.cut {
		size = humanSize(int64(len(v.raw))) + " of " + size
	}
	right := fmt.Sprintf("%s │ UTF-8 │ %s │ Col %d │ %d%% ", mode, size, v.Left, pct)
	a.fill(0, 0, a.w, 1, th.KeyLabel)
	a.print(1, 0, maxInt(0, a.w-2-runewidth.StringWidth(right)), th.KeyLabel, fitRight(v.Title, maxInt(0, a.w-2-runewidth.StringWidth(right))))
	a.print(a.w-runewidth.StringWidth(right), 0, runewidth.StringWidth(right), th.KeyLabel, right)
	wrap := "off"
	if v.Wrap && v.WordWrap {
		wrap = "words"
	} else if v.Wrap {
		wrap = "on"
	}
	cut := ""
	if v.cut {
		cut = fmt.Sprintf("  only the first %s was read ", humanSize(int64(len(v.raw))))
	}
	foot := fmt.Sprintf(" line %d/%d%s  F2 wrap: %s  F7 search  N next  Esc close ", v.Top+1, len(v.Lines), cut, wrap)
	if v.note != "" {
		foot = " " + v.note + "   N: next   F7: search again   Esc: close "
	}
	a.fill(0, a.h-1, a.w, 1, th.KeyLabel)
	a.print(0, a.h-1, a.w, th.KeyLabel, fitLeft(foot, a.w))
}
