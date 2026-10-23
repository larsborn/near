package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// Console is the shell-like output area shown instead of the panels after a
// command was run from the command line. It keeps the output of every
// command (with its prompt echoed) and understands the common SGR colour
// escapes so that coloured tool output survives.
type Console struct {
	mu      sync.Mutex
	lines   []conLine
	partial conLine
	max     int
	scroll  int // visual rows scrolled up from the bottom
	st      tcell.Style
	base    tcell.Style
	esc     []byte // escape sequence being collected
	cr      bool   // a bare \r was seen: next text overwrites the line
}

type conSeg struct {
	text string
	st   tcell.Style
}

type conLine []conSeg

// EventConsole is posted when new output arrived.
type EventConsole struct{ tcell.EventTime }

// EventCommandDone is posted when the running command exited.
type EventCommandDone struct {
	tcell.EventTime
	err error
}

func newConsole(base tcell.Style, max int) *Console {
	if max <= 0 {
		max = 5000
	}
	return &Console{max: max, st: base, base: base}
}

// Println appends a complete line in the given style.
func (c *Console) Println(text string, st tcell.Style) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.flushLocked()
	c.lines = append(c.lines, conLine{{text, st}})
	c.trimLocked()
}

func (c *Console) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines, c.partial, c.scroll = nil, nil, 0
}

func (c *Console) trimLocked() {
	if len(c.lines) > c.max {
		c.lines = append([]conLine(nil), c.lines[len(c.lines)-c.max:]...)
	}
}

func (c *Console) flushLocked() {
	if len(c.partial) > 0 {
		c.lines = append(c.lines, c.partial)
		c.partial = nil
		c.trimLocked()
	}
}

// Feed consumes raw program output.
func (c *Console) Feed(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scroll = 0
	for len(data) > 0 {
		if c.esc != nil {
			c.esc = append(c.esc, data[0])
			data = data[1:]
			c.escapeLocked()
			continue
		}
		r, size := utf8.DecodeRune(data)
		data = data[size:]
		if c.cr && r != '\n' {
			c.partial = nil
			c.cr = false
		}
		switch {
		case r == 0x1b:
			c.esc = []byte{0x1b}
		case r == '\n':
			c.cr = false
			c.lines = append(c.lines, c.partial)
			c.partial = nil
			c.trimLocked()
		case r == '\r':
			c.cr = true
		case r == '\t':
			w := 0
			for _, s := range c.partial {
				w += runewidth.StringWidth(s.text)
			}
			c.appendLocked(strings.Repeat(" ", 8-w%8))
		case r < ' ' || r == 0x7f:
			// other control characters are dropped
		default:
			c.appendLocked(string(r))
		}
	}
}

func (c *Console) appendLocked(s string) {
	if n := len(c.partial); n > 0 && c.partial[n-1].st == c.st {
		c.partial[n-1].text += s
		return
	}
	c.partial = append(c.partial, conSeg{s, c.st})
}

// escapeLocked decides whether the collected escape sequence is complete
// and applies SGR colour sequences; everything else is ignored.
func (c *Console) escapeLocked() {
	e := c.esc
	if len(e) < 2 {
		return
	}
	switch e[1] {
	case '[': // CSI: parameters then a final byte 0x40-0x7e
		last := e[len(e)-1]
		if len(e) == 2 || last < 0x40 || last > 0x7e {
			if len(e) > 64 {
				c.esc = nil
			}
			return
		}
		if last == 'm' {
			c.sgrLocked(string(e[2 : len(e)-1]))
		}
		c.esc = nil
	case ']': // OSC: until BEL or ESC \
		if last := e[len(e)-1]; last == 0x07 || (len(e) > 2 && e[len(e)-2] == 0x1b && last == '\\') || len(e) > 512 {
			c.esc = nil
		}
	default: // two-byte escapes such as ESC = or ESC >
		if len(e) >= 2 {
			c.esc = nil
		}
	}
}

func (c *Console) sgrLocked(params string) {
	if params == "" {
		c.st = c.base
		return
	}
	ps := strings.Split(params, ";")
	for i := 0; i < len(ps); i++ {
		n, _ := strconv.Atoi(ps[i])
		switch {
		case n == 0:
			c.st = c.base
		case n == 1:
			c.st = c.st.Bold(true)
		case n == 2:
			c.st = c.st.Dim(true)
		case n == 3:
			c.st = c.st.Italic(true)
		case n == 4:
			c.st = c.st.Underline(true)
		case n == 7:
			c.st = c.st.Reverse(true)
		case n == 22:
			c.st = c.st.Bold(false).Dim(false)
		case n == 23:
			c.st = c.st.Italic(false)
		case n == 24:
			c.st = c.st.Underline(false)
		case n == 27:
			c.st = c.st.Reverse(false)
		case n >= 30 && n <= 37:
			c.st = c.st.Foreground(tcell.PaletteColor(n - 30))
		case n == 39:
			fg, _, _ := c.base.Decompose()
			c.st = c.st.Foreground(fg)
		case n >= 40 && n <= 47:
			c.st = c.st.Background(tcell.PaletteColor(n - 40))
		case n == 49:
			_, bg, _ := c.base.Decompose()
			c.st = c.st.Background(bg)
		case n >= 90 && n <= 97:
			c.st = c.st.Foreground(tcell.PaletteColor(n - 90 + 8))
		case n >= 100 && n <= 107:
			c.st = c.st.Background(tcell.PaletteColor(n - 100 + 8))
		case n == 38 || n == 48:
			color, used := extendedColor(ps[i+1:])
			if used == 0 {
				return
			}
			i += used
			if n == 38 {
				c.st = c.st.Foreground(color)
			} else {
				c.st = c.st.Background(color)
			}
		}
	}
}

func extendedColor(ps []string) (tcell.Color, int) {
	if len(ps) == 0 {
		return tcell.ColorDefault, 0
	}
	switch ps[0] {
	case "5":
		if len(ps) >= 2 {
			n, _ := strconv.Atoi(ps[1])
			return tcell.PaletteColor(n), 2
		}
	case "2":
		if len(ps) >= 4 {
			r, _ := strconv.Atoi(ps[1])
			g, _ := strconv.Atoi(ps[2])
			b, _ := strconv.Atoi(ps[3])
			return tcell.NewRGBColor(int32(r), int32(g), int32(b)), 4
		}
	}
	return tcell.ColorDefault, 0
}

// Text returns the console contents as plain text (used by tests).
func (c *Console) Text() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var b strings.Builder
	for _, l := range c.lines {
		for _, s := range l {
			b.WriteString(s.text)
		}
		b.WriteByte('\n')
	}
	for _, s := range c.partial {
		b.WriteString(s.text)
	}
	return b.String()
}

// visualRows wraps the last lines at width w and returns at most rows of
// them, honouring the scroll offset. It also reports whether more rows are
// available above.
func (c *Console) visualRows(w, rows int) (out []conLine, more bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if w < 1 {
		w = 1
	}
	all := c.lines
	if len(c.partial) > 0 {
		all = append(append([]conLine(nil), c.lines...), c.partial)
	}
	need := rows + c.scroll
	var collected []conLine // newest last
	for i := len(all) - 1; i >= 0 && len(collected) < need+1; i-- {
		wrapped := wrapLine(all[i], w)
		for j := len(wrapped) - 1; j >= 0 && len(collected) < need+1; j-- {
			collected = append(collected, wrapped[j])
		}
	}
	// collected is newest first
	more = len(collected) > need
	if len(collected) > need {
		collected = collected[:need]
	}
	// drop the scrolled-off newest rows
	if c.scroll > 0 && len(collected) > c.scroll {
		collected = collected[c.scroll:]
	} else if c.scroll > 0 {
		collected = nil
	}
	for i := len(collected) - 1; i >= 0; i-- {
		out = append(out, collected[i])
	}
	return out, more
}

// wrapLine splits a styled line into rows of at most w cells.
func wrapLine(l conLine, w int) []conLine {
	var rows []conLine
	var cur conLine
	used := 0
	for _, s := range l {
		text := s.text
		for text != "" {
			room := w - used
			if room <= 0 {
				rows = append(rows, cur)
				cur, used = nil, 0
				room = w
			}
			part := runewidth.Truncate(text, room, "")
			if part == "" { // a wide rune that does not fit
				rows = append(rows, cur)
				cur, used = nil, 0
				continue
			}
			cur = append(cur, conSeg{part, s.st})
			used += runewidth.StringWidth(part)
			text = text[len(part):]
		}
	}
	rows = append(rows, cur)
	return rows
}

func (c *Console) scrollBy(n, w, rows int) {
	c.mu.Lock()
	total := 0
	for _, l := range c.lines {
		total += len(wrapLine(l, w))
	}
	if len(c.partial) > 0 {
		total += len(wrapLine(c.partial, w))
	}
	c.scroll = clampInt(c.scroll+n, 0, maxInt(0, total-rows))
	c.mu.Unlock()
}

// ---------------------------------------------------------------- running

// drawConsole paints the console into the panel area.
func (a *App) drawConsole(ph int) {
	th := a.theme
	rows, more := a.console.visualRows(a.w, ph)
	y := ph - len(rows)
	for _, row := range rows {
		x := 0
		for _, s := range row {
			x += a.print(x, y, a.w-x, s.st, s.text)
		}
		y++
	}
	if more || a.console.scroll > 0 {
		tag := " PgUp/PgDn scroll "
		if a.console.scroll > 0 {
			tag = fmt.Sprintf(" ↑%d  PgDn/End back ", a.console.scroll)
		}
		a.print(a.w-runewidth.StringWidth(tag), 0, runewidth.StringWidth(tag), th.KeyLabel, tag)
	}
}

// isInteractive reports whether the command's program is listed as needing
// a real terminal.
func (a *App) isInteractive(cmdline string) bool {
	fields := strings.Fields(cmdline)
	if len(fields) == 0 {
		return false
	}
	prog := strings.TrimSuffix(strings.ToLower(fields[0]), ".exe")
	if i := strings.LastIndexAny(prog, `/\`); i >= 0 {
		prog = prog[i+1:]
	}
	for _, p := range a.cfg.InteractiveCommands {
		if strings.EqualFold(p, prog) {
			return true
		}
	}
	return false
}

// runInConsole starts cmdline with its output captured into the console
// and switches to the console view. The command line stays usable; Ctrl+C
// stops the command.
func (a *App) runInConsole(cmdline, cwd string) {
	if a.running != nil {
		a.setErr(fmt.Errorf("a command is still running (Ctrl+C stops it)"))
		return
	}
	cmd := shellCommand(a.shell(), cmdline)
	cmd.Dir = cwd
	cmd.Env = consoleEnv(a.w, a.h-bottomRows)
	setProcessGroup(cmd)
	pr, pw, err := os.Pipe()
	if err != nil {
		a.setErr(err)
		return
	}
	cmd.Stdout, cmd.Stderr = pw, pw
	a.console.Println(a.prompt()+cmdline, a.theme.CmdPrompt)
	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		a.console.Println(err.Error(), a.theme.StatusErr)
		a.hideAll = true
		return
	}
	pw.Close()
	a.running = cmd
	a.runStart = time.Now()
	a.hideAll = true
	screen := a.screen
	con := a.console
	go func() {
		buf := make([]byte, 32*1024)
		last := time.Time{}
		for {
			n, err := pr.Read(buf)
			if n > 0 {
				con.Feed(buf[:n])
				if time.Since(last) > 30*time.Millisecond {
					ev := &EventConsole{}
					ev.SetEventNow()
					_ = screen.PostEvent(ev)
					last = time.Now()
				}
			}
			if err != nil {
				break
			}
		}
		pr.Close()
		werr := cmd.Wait()
		ev := &EventCommandDone{err: werr}
		ev.SetEventNow()
		_ = screen.PostEvent(ev)
	}()
}

// consoleEnv asks tools for colour even though stdout is a pipe, and tells
// them the console size.
func consoleEnv(cols, rows int) []string {
	env := os.Environ()
	add := func(k, v string) {
		for i, e := range env {
			if strings.HasPrefix(e, k+"=") {
				env[i] = k + "=" + v
				return
			}
		}
		env = append(env, k+"="+v)
	}
	add("COLUMNS", strconv.Itoa(cols))
	add("LINES", strconv.Itoa(rows))
	add("CLICOLOR_FORCE", "1")
	add("FORCE_COLOR", "1")
	if os.Getenv("GIT_CONFIG_COUNT") == "" {
		add("GIT_CONFIG_COUNT", "1")
		add("GIT_CONFIG_KEY_0", "color.ui")
		add("GIT_CONFIG_VALUE_0", "always")
	}
	return env
}

func (a *App) commandDone(err error) {
	a.running = nil
	elapsed := time.Since(a.runStart).Round(10 * time.Millisecond)
	switch e := err.(type) {
	case nil:
		if elapsed > 2*time.Second {
			a.console.Println(fmt.Sprintf("[done in %s]", elapsed), a.theme.ListDim.Background(bgOf(a.theme.Cmd)))
		}
	case *exec.ExitError:
		a.console.Println(fmt.Sprintf("[exit %d after %s]", e.ExitCode(), elapsed), a.theme.StatusErr)
	default:
		a.console.Println("["+err.Error()+"]", a.theme.StatusErr)
	}
	a.reloadAll()
}

func (a *App) stopCommand() {
	if a.running == nil {
		return
	}
	a.console.Println("^C", a.theme.StatusErr)
	killTree(a.running)
}

func bgOf(st tcell.Style) tcell.Color {
	_, bg, _ := st.Decompose()
	return bg
}
