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
type winWatcher struct {
	events  chan fsEvent
	mu      sync.Mutex
	handles []windows.Handle
	closed  bool
	wg      sync.WaitGroup
}

func newWinWatcher() *winWatcher {
	return &winWatcher{events: make(chan fsEvent, 4096)}
}

func (w *winWatcher) Recursive() bool        { return true }
func (w *winWatcher) Events() <-chan fsEvent { return w.events }

func (w *winWatcher) Add(root string) error {
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(p, windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		windows.CloseHandle(h)
		return nil
	}
	w.handles = append(w.handles, h)
	w.wg.Add(1)
	w.mu.Unlock()
	go w.readLoop(h, root)
	return nil
}

func (w *winWatcher) readLoop(h windows.Handle, root string) {
	defer w.wg.Done()
	buf := make([]byte, 256*1024)
	const mask = windows.FILE_NOTIFY_CHANGE_FILE_NAME | windows.FILE_NOTIFY_CHANGE_DIR_NAME
	for {
		var n uint32
		err := windows.ReadDirectoryChanges(h, &buf[0], uint32(len(buf)), true, mask, &n, nil, 0)
		w.mu.Lock()
		closed := w.closed
		w.mu.Unlock()
		if closed {
			return
		}
		if err != nil {
			if err == windows.ERROR_NOTIFY_ENUM_DIR {
				w.events <- fsEvent{Op: fsOverflow}
				continue
			}
			return
		}
		if n == 0 {
			// The buffer overflowed; the caller must rescan.
			w.events <- fsEvent{Op: fsOverflow}
			continue
		}
		for off := uint32(0); off < n; {
			info := (*windows.FileNotifyInformation)(unsafe.Pointer(&buf[off]))
			nameLen := int(info.FileNameLength) / 2
			name := windows.UTF16ToString(unsafe.Slice(&info.FileName, nameLen))
			full := filepath.Join(root, name)
			switch info.Action {
			case windows.FILE_ACTION_ADDED, windows.FILE_ACTION_RENAMED_NEW_NAME:
				w.events <- fsEvent{fsCreate, full}
			case windows.FILE_ACTION_REMOVED, windows.FILE_ACTION_RENAMED_OLD_NAME:
				w.events <- fsEvent{fsRemove, full}
			}
			if info.NextEntryOffset == 0 {
				break
			}
			off += info.NextEntryOffset
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
	handles := w.handles
	w.mu.Unlock()
	for _, h := range handles {
		_ = windows.CancelIoEx(h, nil)
		_ = windows.CloseHandle(h)
	}
	w.wg.Wait()
	close(w.events)
	return nil
}
