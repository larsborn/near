package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
)

// listDirFn is the directory lister; tests replace it with a slow one.
var listDirFn = listDir

// slowDirDelay is how long a directory read may take before the waiting
// box appears.
const slowDirDelay = 300 * time.Millisecond

var errDirCancelled = errors.New("directory read cancelled")

type listResult struct {
	entries []Entry
	notes   map[string]string
	err     error
}

// listDirWait reads a directory in the background. If it takes longer than
// slowDirDelay a box tells the user and Esc gives up (the read itself keeps
// running in its goroutine until the OS returns; its result is dropped).
func (a *App) listDirWait(path string) ([]Entry, map[string]string, error) {
	res := make(chan listResult, 1)
	showHidden, notesFile := a.showHidden, a.cfg.NotesFile
	go func() {
		entries, err := listDirFn(path, showHidden, notesFile)
		res <- listResult{entries: entries, notes: loadNotes(path, notesFile), err: err}
	}()
	select {
	case r := <-res:
		return r.entries, r.notes, r.err
	case <-time.After(slowDirDelay):
	}
	start := time.Now()
	spin := 0
	for {
		select {
		case r := <-res:
			return r.entries, r.notes, r.err
		default:
		}
		for a.screen.HasPendingEvent() {
			if ev, ok := a.screen.PollEvent().(*tcell.EventKey); ok && ev.Key() == tcell.KeyEscape {
				return nil, nil, errDirCancelled
			}
		}
		spin++
		a.slowBoxShown = true
		a.drawAll()
		th := a.theme
		w := clampInt(a.w-10, 40, 76)
		h := 6
		x, y := a.center(w, h)
		a.drawBox(x, y, w, h, th.Dialog, "Reading directory "+[]string{"|", "/", "-", "\\"}[spin%4])
		a.print(x+2, y+1, w-4, th.Dialog, fitRight(path, w-4))
		a.print(x+2, y+2, w-4, th.Dialog, fmt.Sprintf("This is taking a while… %s", fmtDuration(time.Since(start))))
		a.print(x+2, y+4, w-4, th.DialogHint, "Esc: give up and stay where you were")
		a.screen.Show()
		time.Sleep(50 * time.Millisecond)
	}
}
