//go:build windows

package main

import (
	"path/filepath"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// newWatcher returns the native recursive watcher on Windows.
func newWatcher() (fsWatcher, error) { return newWinWatcher(), nil }

// winWatcher uses ReadDirectoryChangesW with subtree watching: one handle
// per root covers the whole tree, so no per-directory watches are needed.
// The watch is armed with an overlapped read before Add returns, so a
// change made right after Add is not lost.
type winWatcher struct {
	events chan fsEvent
	mu     sync.Mutex
	roots  []*winRoot
	closed bool
	wg     sync.WaitGroup
}

type winRoot struct {
	h    windows.Handle
	ov   windows.Overlapped
	buf  []byte
	path string
}

func newWinWatcher() *winWatcher {
	return &winWatcher{events: make(chan fsEvent, 4096)}
}

func (w *winWatcher) Recursive() bool        { return true }
func (w *winWatcher) Events() <-chan fsEvent { return w.events }

const winNotifyMask = windows.FILE_NOTIFY_CHANGE_FILE_NAME | windows.FILE_NOTIFY_CHANGE_DIR_NAME

// arm issues the next asynchronous directory read.
func (r *winRoot) arm() error {
	var n uint32
	err := windows.ReadDirectoryChanges(r.h, &r.buf[0], uint32(len(r.buf)), true, winNotifyMask, &n, &r.ov, 0)
	if err != nil && err != windows.ERROR_IO_PENDING {
		return err
	}
	return nil
}

func (w *winWatcher) Add(root string) error {
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(p, windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return err
	}
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		windows.CloseHandle(h)
		return err
	}
	r := &winRoot{h: h, buf: make([]byte, 256*1024), path: root}
	r.ov.HEvent = ev
	if err := r.arm(); err != nil {
		windows.CloseHandle(ev)
		windows.CloseHandle(h)
		return err
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		windows.CancelIoEx(h, &r.ov)
		windows.CloseHandle(ev)
		windows.CloseHandle(h)
		return nil
	}
	w.roots = append(w.roots, r)
	w.wg.Add(1)
	w.mu.Unlock()
	go w.readLoop(r)
	return nil
}

func (w *winWatcher) readLoop(r *winRoot) {
	defer w.wg.Done()
	for {
		var n uint32
		err := windows.GetOverlappedResult(r.h, &r.ov, &n, true)
		w.mu.Lock()
		closed := w.closed
		w.mu.Unlock()
		if closed {
			return
		}
		if err != nil {
			if err == windows.ERROR_NOTIFY_ENUM_DIR {
				w.events <- fsEvent{Op: fsOverflow}
				if r.arm() != nil {
					return
				}
				continue
			}
			return
		}
		if n == 0 {
			// The buffer overflowed; the caller must rescan.
			w.events <- fsEvent{Op: fsOverflow}
		}
		// Copy the records out before re-arming so the buffer can be reused.
		var evs []fsEvent
		for off := uint32(0); off < n; {
			info := (*windows.FileNotifyInformation)(unsafe.Pointer(&r.buf[off]))
			nameLen := int(info.FileNameLength) / 2
			name := windows.UTF16ToString(unsafe.Slice(&info.FileName, nameLen))
			full := filepath.Join(r.path, name)
			switch info.Action {
			case windows.FILE_ACTION_ADDED, windows.FILE_ACTION_RENAMED_NEW_NAME:
				evs = append(evs, fsEvent{fsCreate, full})
			case windows.FILE_ACTION_REMOVED, windows.FILE_ACTION_RENAMED_OLD_NAME:
				evs = append(evs, fsEvent{fsRemove, full})
			}
			if info.NextEntryOffset == 0 {
				break
			}
			off += info.NextEntryOffset
		}
		if r.arm() != nil {
			return
		}
		for _, e := range evs {
			w.events <- e
		}
	}
}

func (w *winWatcher) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	roots := w.roots
	w.mu.Unlock()
	for _, r := range roots {
		_ = windows.CancelIoEx(r.h, &r.ov)
		_ = windows.CloseHandle(r.h)
	}
	w.wg.Wait()
	for _, r := range roots {
		_ = windows.CloseHandle(r.ov.HEvent)
	}
	close(w.events)
	return nil
}
