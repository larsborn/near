package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// A transfer is one copy or move operation: a set of source/destination
// pairs with the options chosen in the dialog. Transfers are queued and run
// one after another by a single worker goroutine, in the foreground (modal
// progress box) or the background.
type transfer struct {
	id     int
	move   bool
	jobs   []transferJob
	flags  copyFlags
	policy string
	srcDir string
	notes  string
	cancel atomic.Bool

	mu         sync.Mutex
	item, dst  string
	done       int64 // bytes of the current file
	total      int64 // size of the current file, -1 for a folder
	filesDone  int
	bytesDone  int64 // bytes of finished files
	bytesTotal int64 // whole transfer, 0 when unknown
	start      time.Time
	itemStart  time.Time
	finished   bool
	err        error
	okCount    int
}

type transferJob struct{ src, dst, name string }

func (t *transfer) title() string {
	if t.move {
		return "Rename/Move"
	}
	return "Copy"
}

func (t *transfer) verb() string {
	if t.move {
		return "Moving"
	}
	return "Copying"
}

// snapshot copies the progress fields under the lock.
func (t *transfer) snapshot() transfer {
	t.mu.Lock()
	defer t.mu.Unlock()
	return transfer{id: t.id, move: t.move, jobs: t.jobs, item: t.item, dst: t.dst, done: t.done, total: t.total,
		filesDone: t.filesDone, bytesDone: t.bytesDone, bytesTotal: t.bytesTotal, start: t.start,
		itemStart: t.itemStart, finished: t.finished, err: t.err, okCount: t.okCount}
}

// EventTransfer is posted when a transfer made progress.
type EventTransfer struct{ tcell.EventTime }

// EventTransferDone is posted when a transfer finished (or was cancelled).
type EventTransferDone struct {
	tcell.EventTime
	t *transfer
}

// transferQueue runs transfers consecutively and remembers finished ones.
type transferQueue struct {
	mu      sync.Mutex
	queue   []*transfer // waiting, in order
	running *transfer
	history []*transfer // finished, oldest first
	nextID  int
	screen  tcell.Screen
	wake    chan struct{}
	started bool
	lastEv  time.Time
}

const transferHistoryMax = 50

func newTransferQueue(screen tcell.Screen) *transferQueue {
	return &transferQueue{screen: screen, wake: make(chan struct{}, 1)}
}

// add queues a transfer and starts the worker if needed.
func (q *transferQueue) add(t *transfer) {
	q.mu.Lock()
	q.nextID++
	t.id = q.nextID
	q.queue = append(q.queue, t)
	if !q.started {
		q.started = true
		go q.worker()
	}
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// pending returns the running transfer (first, if any) and the waiting ones.
func (q *transferQueue) pending() []*transfer {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []*transfer
	if q.running != nil {
		out = append(out, q.running)
	}
	return append(out, q.queue...)
}

func (q *transferQueue) active() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.running != nil || len(q.queue) > 0
}

// all returns finished, running and queued transfers in that order.
func (q *transferQueue) all() []*transfer {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := append([]*transfer{}, q.history...)
	if q.running != nil {
		out = append(out, q.running)
	}
	return append(out, q.queue...)
}

func (q *transferQueue) rememberLocked(t *transfer) {
	q.history = append(q.history, t)
	if len(q.history) > transferHistoryMax {
		q.history = q.history[len(q.history)-transferHistoryMax:]
	}
}

// remove cancels a transfer: a waiting one is dropped, the running one is
// asked to stop.
func (q *transferQueue) remove(t *transfer) {
	q.mu.Lock()
	for i, x := range q.queue {
		if x == t {
			q.queue = append(q.queue[:i], q.queue[i+1:]...)
			t.mu.Lock()
			t.finished, t.err = true, errCancelled
			t.mu.Unlock()
			q.rememberLocked(t)
			q.mu.Unlock()
			q.post(&EventTransferDone{t: t})
			return
		}
	}
	q.mu.Unlock()
	t.cancel.Store(true)
}

func (q *transferQueue) post(ev tcell.Event) {
	if q.screen == nil {
		return
	}
	switch e := ev.(type) {
	case *EventTransfer:
		e.SetEventNow()
	case *EventTransferDone:
		e.SetEventNow()
	}
	_ = q.screen.PostEvent(ev)
}

func (q *transferQueue) worker() {
	for range q.wake {
		for {
			q.mu.Lock()
			if len(q.queue) == 0 {
				q.mu.Unlock()
				break
			}
			t := q.queue[0]
			q.queue = q.queue[1:]
			q.running = t
			q.mu.Unlock()
			q.run(t)
			q.mu.Lock()
			q.running = nil
			q.rememberLocked(t)
			q.mu.Unlock()
			q.post(&EventTransferDone{t: t})
		}
	}
}

// run executes one transfer, reporting progress into it.
func (q *transferQueue) run(t *transfer) {
	t.mu.Lock()
	t.start = time.Now()
	t.bytesTotal = 0
	for _, j := range t.jobs {
		st, err := os.Lstat(j.src)
		if err != nil || !st.Mode().IsRegular() {
			t.bytesTotal = 0
			break
		}
		t.bytesTotal += st.Size()
	}
	t.mu.Unlock()

	var err error
	for _, j := range t.jobs {
		if t.cancel.Load() {
			err = errCancelled
			break
		}
		if sameFile(j.src, j.dst) && !t.move {
			err = fmt.Errorf("%s: source and destination are the same", j.name)
			break
		}
		flags := t.flags
		if t.policy == "Only newer" {
			flags.Overwrite = newerThan(j.src, j.dst)
		}
		// An existing file is skipped unless overwriting; an existing
		// directory is merged when copying and skipped when moving.
		if st, exists := os.Lstat(j.dst); exists == nil && !flags.Overwrite && !sameFile(j.src, j.dst) && (t.move || !st.IsDir()) {
			t.mu.Lock()
			t.filesDone++
			t.mu.Unlock()
			continue
		}
		job := j
		pf := func(item string, done, total int64) bool {
			if t.cancel.Load() {
				return false
			}
			t.mu.Lock()
			if item != t.item {
				t.itemStart = time.Now()
			}
			t.item, t.done, t.total = item, done, total
			t.dst = job.dst
			if rel, rerr := filepath.Rel(job.src, item); rerr == nil && rel != "." {
				t.dst = filepath.Join(job.dst, rel)
			}
			t.mu.Unlock()
			q.mu.Lock()
			due := time.Since(q.lastEv) > 80*time.Millisecond
			if due {
				q.lastEv = time.Now()
			}
			q.mu.Unlock()
			if due {
				q.post(&EventTransfer{})
			}
			return true
		}
		if t.move {
			err = moveTree(j.src, j.dst, flags, pf)
		} else {
			err = copyTree(j.src, j.dst, flags, pf)
		}
		if err != nil {
			break
		}
		dstDir, dstName := filepath.Split(j.dst)
		dstDir = filepath.Clean(dstDir)
		if t.move {
			moveNote(t.srcDir, j.name, dstDir, dstName, t.notes)
		} else {
			copyNote(t.srcDir, j.name, dstDir, dstName, t.notes)
		}
		t.mu.Lock()
		if st, serr := os.Lstat(j.dst); serr == nil && st.Mode().IsRegular() {
			t.bytesDone += st.Size()
		}
		t.filesDone++
		t.okCount++
		t.mu.Unlock()
	}
	t.mu.Lock()
	t.finished, t.err = true, err
	t.mu.Unlock()
}

// ---------------------------------------------------------------- app side

// startTransfer queues a transfer; unless background is set, a modal
// progress box follows it (B sends it to the background, Esc cancels).
func (a *App) startTransfer(t *transfer, background bool) {
	a.transfers.add(t)
	if !background {
		a.push(&TransferDialog{t: t})
	} else {
		a.setStatus(fmt.Sprintf("%s queued in the background (Ctrl+B lists transfers)", t.title()))
	}
}

func (a *App) transferDone(t *transfer) {
	s := t.snapshot()
	// Close the modal box if it is showing this transfer.
	if len(a.dialogs) > 0 {
		if d, ok := a.top().(*TransferDialog); ok && d.t == t {
			a.pop()
		}
	}
	a.reloadAll()
	switch {
	case errors.Is(s.err, errCancelled):
		a.setStatus(fmt.Sprintf("%s cancelled (%d entries done)", s.verb(), s.okCount))
	case s.err != nil:
		a.setErr(fmt.Errorf("%s: %v (%d entries done)", s.verb(), s.err, s.okCount))
	default:
		a.setStatus(fmt.Sprintf("%s: %d entries", s.verb(), s.okCount))
	}
}

// describe returns a one-line label and the status of a transfer.
func (t *transfer) describe() (label, status string) {
	s := t.snapshot()
	dest := ""
	if len(s.jobs) > 0 {
		dest = filepath.Dir(s.jobs[0].dst)
	}
	label = fmt.Sprintf("%s %d entries to %s", s.verb(), len(s.jobs), fitRight(dest, 40))
	switch {
	case s.finished && errors.Is(s.err, errCancelled):
		status = "cancelled"
	case s.finished && s.err != nil:
		status = "failed: " + s.err.Error()
	case s.finished:
		status = "done"
	case s.start.IsZero():
		status = "queued"
	case s.total > 0:
		status = fmt.Sprintf("%d%%", s.done*100/s.total)
	default:
		status = "running"
	}
	return label, status
}

// transferStatus is the status line summary of background activity.
func (a *App) transferStatus() string {
	pend := a.transfers.pending()
	if len(pend) == 0 {
		return ""
	}
	s := pend[0].snapshot()
	pct := ""
	if s.total > 0 {
		pct = fmt.Sprintf(" %d%%", s.done*100/s.total)
	}
	msg := fmt.Sprintf("Background: %s %s%s", s.verb(), filepath.Base(s.item), pct)
	if len(pend) > 1 {
		msg += fmt.Sprintf(", %d more queued", len(pend)-1)
	}
	return msg + "  (Ctrl+B)"
}

// showTransfers lists running and queued transfers.
func (a *App) showTransfers() {
	all := a.transfers.all()
	if len(all) == 0 {
		a.setStatus("No transfers yet")
		return
	}
	var items []ListItem
	for _, t := range all {
		label, status := t.describe()
		items = append(items, ListItem{Label: fitLeft(label, 60), Right: fitLeft(status, 24), Data: t})
	}
	d := a.list("Transfers", items, func(_ int, li ListItem) {
		a.push(&TransferDialog{t: li.Data.(*transfer)})
	})
	d.Width = clampInt(a.w-10, 50, 90)
	d.Cursor = len(items) - 1
	d.Hint = "Enter: show   Delete: cancel a running or queued transfer   Esc: close"
	d.OnKey = func(a *App, d *ListDialog, ev *tcell.EventKey) bool {
		if keyName(ev) == "Delete" && d.Cursor < len(d.Items) {
			t := d.Items[d.Cursor].Data.(*transfer)
			if !t.snapshot().finished {
				a.transfers.remove(t)
			}
			_, d.Items[d.Cursor].Right = t.describe()
			return true
		}
		return false
	}
}

// ---------------------------------------------------------------- progress box

// TransferDialog is FAR's Copy / Rename-Move progress box over a transfer.
type TransferDialog struct {
	t *transfer
}

func (d *TransferDialog) HandleKey(a *App, ev *tcell.EventKey) {
	finished := d.t.snapshot().finished
	switch keyName(ev) {
	case "Esc":
		if !finished {
			a.transfers.remove(d.t)
		}
		a.pop()
	case "Enter":
		if finished {
			a.pop()
		}
	case "b", "B", "Ctrl+B":
		a.pop()
		if !finished {
			a.setStatus(d.t.title() + " continues in the background (Ctrl+B lists transfers)")
		}
	}
}

func (d *TransferDialog) Draw(a *App) {
	th := a.theme
	s := d.t.snapshot()
	w := clampInt(a.w-10, 40, 76)
	h := 12
	x, y := a.center(w, h)
	inner := w - 4
	a.drawBox(x, y, w, h, th.Dialog, s.title())
	if s.start.IsZero() {
		// Not started yet: show the queue with the earlier transfers and
		// their status, this one marked.
		all := a.transfers.all()
		ahead := 0
		mine := -1
		for i, p := range all {
			if p == d.t {
				mine = i
				break
			}
			if !p.snapshot().finished {
				ahead++
			}
		}
		a.print(x+2, y+1, inner, th.Dialog, fmt.Sprintf("Waiting for %d earlier transfer(s) to finish…", ahead))
		a.separator(x, y+2, w, th.Dialog)
		rows := h - 5
		first := maxInt(0, mine+1-rows)
		for i := 0; i < rows && first+i < len(all); i++ {
			p := all[first+i]
			label, status := p.describe()
			st := th.Dialog
			if p == d.t {
				label = "▶ " + label
				status = "this one"
			} else if p.snapshot().finished {
				st = th.ListDim
			}
			sw := runewidth.StringWidth(status)
			a.print(x+2, y+3+i, inner-sw-1, st, fitLeft(label, inner-sw-1))
			a.print(x+2+inner-sw, y+3+i, sw, st, status)
		}
		a.print(x+2, y+h-2, inner, th.DialogHint, "B: background   Esc: cancel")
		return
	}
	what := "the file"
	if s.total < 0 {
		what = "the folder"
	}
	a.print(x+2, y+1, inner, th.Dialog, s.verb()+" "+what)
	a.print(x+2, y+2, inner, th.Dialog, fitRight(s.item, inner))
	a.print(x+2, y+3, inner, th.Dialog, "to")
	a.print(x+2, y+4, inner, th.Dialog, fitRight(s.dst, inner))
	pct := 0
	if s.total > 0 {
		pct = int(s.done * 100 / s.total)
	}
	if s.finished && s.err == nil {
		pct, s.done, s.total = 100, 1, 1
	}
	pctText := fmt.Sprintf("%3d%%", pct)
	barW := inner - len(pctText) - 1
	filled := 0
	if s.total > 0 && barW > 0 {
		filled = int(int64(barW) * s.done / s.total)
	}
	a.print(x+2, y+5, barW, th.Dialog, strings.Repeat("█", filled)+strings.Repeat("░", maxInt(0, barW-filled)))
	a.print(x+2+barW+1, y+5, len(pctText), th.Dialog, pctText)

	a.separator(x, y+6, w, th.Dialog)
	tl := " Total "
	a.print(x+(w-len(tl))/2, y+6, len(tl), th.Dialog, tl)
	files := fmt.Sprintf("%d / %d", s.filesDone, len(s.jobs))
	overall := s.bytesDone + maxInt64(s.done, 0)
	bytesText := humanSize(overall)
	if s.bytesTotal > 0 {
		bytesText = fmt.Sprintf("%s / %s", humanSize(overall), humanSize(s.bytesTotal))
	}
	a.print(x+2, y+7, inner, th.Dialog, "Files:")
	a.print(x+2+inner-len(files), y+7, len(files), th.Dialog, files)
	a.print(x+2, y+8, inner, th.Dialog, "Bytes:")
	a.print(x+2+inner-len(bytesText), y+8, len(bytesText), th.Dialog, bytesText)
	a.separator(x, y+9, w, th.Dialog)
	rates := transferRates(&s)
	hint := "B: background  Esc: cancel"
	if s.finished {
		_, status := d.t.describe()
		rates = "Finished: " + status
		hint = "Enter / Esc: close"
	}
	a.print(x+2, y+10, inner-runewidth.StringWidth(hint)-1, th.Dialog, fitLeft(rates, inner-runewidth.StringWidth(hint)-1))
	a.print(x+2+inner-runewidth.StringWidth(hint), y+10, runewidth.StringWidth(hint), th.DialogHint, hint)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// transferRates formats FAR's time line: elapsed, remaining and speed.
func transferRates(s *transfer) string {
	var parts []string
	parts = append(parts, "Time: "+fmtDuration(time.Since(s.start)))
	var speed float64
	if el := time.Since(s.itemStart).Seconds(); el >= 0.5 && s.done > 0 {
		speed = float64(s.done) / el
	}
	switch {
	case s.bytesTotal > 0:
		overall := s.bytesDone + maxInt64(s.done, 0)
		if el := time.Since(s.start).Seconds(); el >= 0.5 && overall > 0 && s.bytesTotal > overall {
			rate := float64(overall) / el
			parts = append(parts, "Remaining: "+fmtDuration(time.Duration(float64(s.bytesTotal-overall)/rate*float64(time.Second))))
		}
	case speed > 0 && s.total > s.done:
		parts = append(parts, "Remaining: "+fmtDuration(time.Duration(float64(s.total-s.done)/speed*float64(time.Second))))
	}
	if speed > 0 {
		parts = append(parts, "Speed: "+humanSize(int64(speed))+"/s")
	}
	return strings.Join(parts, "  ")
}
