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

// listDirWait returns a directory listing, from the cache when the path is
// on a network drive and the listing is still fresh. The returned time is
// when a cached listing was read, and zero when it comes from the disk.
func (a *App) listDirWait(path string) ([]Entry, map[string]string, time.Time, error) {
	if e, ok := a.dirs.get(path); ok {
		return visible(e.entries, a.showHidden), e.notes, e.at, nil
	}
	entries, notes, err := a.listDirFresh(path)
	return entries, notes, time.Time{}, err
}

// listDirFresh reads a directory in the background, skipping the cache and
// replacing what is in it. If the read takes longer than slowDirDelay a box
// tells the user and Esc gives up (the read itself keeps running in its
// goroutine until the OS returns; its result is dropped).
func (a *App) listDirFresh(path string) ([]Entry, map[string]string, error) {
	res := make(chan listResult, 1)
	notesFile := a.cfg.NotesFile
	cache := a.dirs
	go func() {
		// Read everything, hidden entries included, so that the cache does
		// not have to be thrown away when hidden files are switched on.
		entries, err := listDirFn(path, true, notesFile)
		notes := loadNotes(path, notesFile)
		if err == nil {
			cache.put(path, entries, notes)
		}
		res <- listResult{entries: entries, notes: notes, err: err}
	}()
	select {
	case r := <-res:
		return visible(r.entries, a.showHidden), r.notes, r.err
	case <-time.After(slowDirDelay):
	}
	start := time.Now()
	spin := 0
	for {
		select {
		case r := <-res:
			return visible(r.entries, a.showHidden), r.notes, r.err
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
