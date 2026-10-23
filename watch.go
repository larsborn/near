package main

import (
	"github.com/fsnotify/fsnotify"
)

// fsOp is the kind of change a watcher reports.
type fsOp int

const (
	fsCreate   fsOp = iota // a file or directory appeared (also: renamed to)
	fsRemove               // a file or directory vanished (also: renamed from)
	fsOverflow             // events were lost; the index must be rebuilt
)

type fsEvent struct {
	Op   fsOp
	Path string
}

// fsWatcher is a filesystem change source. Recursive backends watch a whole
// tree per Add; others need one Add per directory.
type fsWatcher interface {
	Add(dir string) error
	Recursive() bool
	Events() <-chan fsEvent
	Close() error
}

// fsnotifyWatcher is the portable backend (inotify, kqueue, or a
// non-recursive ReadDirectoryChangesW): one watch per directory.
type fsnotifyWatcher struct {
	w      *fsnotify.Watcher
	events chan fsEvent
}

func newFsnotifyWatcher() (fsWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	fw := &fsnotifyWatcher{w: w, events: make(chan fsEvent, 4096)}
	go fw.loop()
	return fw, nil
}

func (fw *fsnotifyWatcher) loop() {
	defer close(fw.events)
	for {
		select {
		case ev, ok := <-fw.w.Events:
			if !ok {
				return
			}
			switch {
			case ev.Has(fsnotify.Create):
				fw.events <- fsEvent{fsCreate, ev.Name}
			case ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename):
				fw.events <- fsEvent{fsRemove, ev.Name}
			}
		case err, ok := <-fw.w.Errors:
			if !ok {
				return
			}
			if err == fsnotify.ErrEventOverflow {
				fw.events <- fsEvent{Op: fsOverflow}
			}
		}
	}
}

func (fw *fsnotifyWatcher) Add(dir string) error   { return fw.w.Add(dir) }
func (fw *fsnotifyWatcher) Recursive() bool        { return false }
func (fw *fsnotifyWatcher) Events() <-chan fsEvent { return fw.events }
func (fw *fsnotifyWatcher) Close() error           { return fw.w.Close() }
