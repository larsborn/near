//go:build windows

package main

import (
	"encoding/binary"
	"errors"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// NTFS change journal (USN journal) access without administrator rights.
//
// A volume handle opened with FILE_TRAVERSE may query the journal and read
// it with FSCTL_READ_UNPRIVILEGED_USN_JOURNAL. The unprivileged read strips
// every file name, so a record only tells that *something* happened to an
// entry of the directory with the given parent reference number. That is
// enough: the parent is resolved to a path with OpenFileById and the
// directory is re-read and reconciled with the index (journalChangedDirs,
// Index.syncDir). A parent that cannot be opened was deleted itself; its own
// record, which names its still-existing parent, covers the subtree.

const (
	fsctlQueryUsnJournal            = 0x000900f4
	fsctlReadUnprivilegedUsnJournal = 0x000903ab

	usnReasonFileCreate    = 0x00000100
	usnReasonFileDelete    = 0x00000200
	usnReasonRenameOldName = 0x00001000
	usnReasonRenameNewName = 0x00002000
	usnReasonHardLink      = 0x00010000

	// usnReasons are the record kinds that change a directory's listing.
	usnReasons = usnReasonFileCreate | usnReasonFileDelete | usnReasonRenameOldName | usnReasonRenameNewName | usnReasonHardLink
)

var procOpenFileById = windows.NewLazySystemDLL("kernel32.dll").NewProc("OpenFileById")

// journalPos is where the index stands in a volume's change journal.
type journalPos struct {
	ID   uint64 // journal identity; a recreated journal gets a new one
	Next int64  // first USN not yet applied
}

// journalsSupported reports whether this platform has change journals.
func journalsSupported() bool { return true }

// journalVolume returns the volume a root lies on, as "C:" (empty when the
// root has no drive letter).
func journalVolume(root string) string {
	v := filepath.VolumeName(root)
	if len(v) != 2 || v[1] != ':' {
		return ""
	}
	return strings.ToUpper(v)
}

// openVolume opens a volume for journal access without elevation.
func openVolume(vol string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(`\\.\` + vol)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(p, windows.FILE_TRAVERSE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, 0, 0)
}

// queryJournal returns the journal's identity and its first and next USN.
func queryJournal(h windows.Handle) (id uint64, first, next int64, err error) {
	var q [80]byte
	var n uint32
	if err := windows.DeviceIoControl(h, fsctlQueryUsnJournal, nil, 0, &q[0], uint32(len(q)), &n, nil); err != nil {
		return 0, 0, 0, err
	}
	return binary.LittleEndian.Uint64(q[0:]), int64(binary.LittleEndian.Uint64(q[8:])), int64(binary.LittleEndian.Uint64(q[16:])), nil
}

// journalPositions queries the current position of every journaled volume
// among roots. Volumes without a readable journal are left out.
func journalPositions(roots []string) map[string]journalPos {
	out := map[string]journalPos{}
	for _, r := range roots {
		vol := journalVolume(r)
		if vol == "" {
			continue
		}
		if _, ok := out[vol]; ok {
			continue
		}
		h, err := openVolume(vol)
		if err != nil {
			continue
		}
		if id, _, next, err := queryJournal(h); err == nil {
			out[vol] = journalPos{ID: id, Next: next}
		}
		windows.CloseHandle(h)
	}
	return out
}

var errJournalUnusable = errors.New("change journal unusable")

// journalChangedDirs lists the directories whose listing changed on vol
// since pos, in order of first appearance, and the position reached. It
// fails when the journal was recreated, has wrapped past pos, or cannot be
// read; the caller then rebuilds.
func journalChangedDirs(vol string, pos journalPos) (dirs []string, next journalPos, err error) {
	h, err := openVolume(vol)
	if err != nil {
		return nil, pos, err
	}
	defer windows.CloseHandle(h)
	id, first, _, err := queryJournal(h)
	if err != nil {
		return nil, pos, err
	}
	if id != pos.ID || pos.Next < first {
		return nil, pos, errJournalUnusable
	}
	seen := map[uint64]string{}
	start := pos.Next
	in := make([]byte, 56) // READ_USN_JOURNAL_DATA_V1
	binary.LittleEndian.PutUint32(in[8:], usnReasons)
	binary.LittleEndian.PutUint64(in[32:], id)
	binary.LittleEndian.PutUint16(in[40:], 2) // V2 records only (NTFS)
	binary.LittleEndian.PutUint16(in[42:], 2)
	out := make([]byte, 1<<20)
	for {
		binary.LittleEndian.PutUint64(in[0:], uint64(start))
		var n uint32
		if err := windows.DeviceIoControl(h, fsctlReadUnprivilegedUsnJournal, &in[0], uint32(len(in)), &out[0], uint32(len(out)), &n, nil); err != nil {
			return nil, pos, err
		}
		if n < 8 {
			return nil, pos, errJournalUnusable
		}
		newStart := int64(binary.LittleEndian.Uint64(out[0:]))
		for off := 8; off+60 <= int(n); {
			rl := int(binary.LittleEndian.Uint32(out[off:]))
			if rl < 60 || off+rl > int(n) {
				break
			}
			parent := binary.LittleEndian.Uint64(out[off+16:])
			reason := binary.LittleEndian.Uint32(out[off+40:])
			off += rl
			if reason&usnReasons == 0 {
				continue
			}
			if _, done := seen[parent]; done {
				continue
			}
			p := resolveFileID(h, parent)
			seen[parent] = p
			if p != "" {
				dirs = append(dirs, p)
			}
		}
		if newStart <= start || n <= 8 {
			return dirs, journalPos{ID: id, Next: newStart}, nil
		}
		start = newStart
	}
}

// resolveFileID turns a file reference number into a path, "" when the
// entry no longer exists or cannot be opened.
func resolveFileID(vol windows.Handle, frn uint64) string {
	var desc [24]byte // FILE_ID_DESCRIPTOR
	binary.LittleEndian.PutUint32(desc[0:], 24)
	binary.LittleEndian.PutUint32(desc[4:], 0) // FileIdType
	binary.LittleEndian.PutUint64(desc[8:], frn)
	r, _, _ := procOpenFileById.Call(uintptr(vol), uintptr(unsafe.Pointer(&desc[0])), 0,
		uintptr(windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE), 0,
		uintptr(windows.FILE_FLAG_BACKUP_SEMANTICS))
	if r == 0 || r == ^uintptr(0) {
		return ""
	}
	h := windows.Handle(r)
	defer windows.CloseHandle(h)
	buf := make([]uint16, 1024)
	n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), 0)
	if err != nil || n == 0 {
		return ""
	}
	if int(n) > len(buf) {
		buf = make([]uint16, n+1)
		if n, err = windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), 0); err != nil {
			return ""
		}
	}
	p := windows.UTF16ToString(buf[:n])
	p = strings.TrimPrefix(p, `\\?\`)
	if strings.HasPrefix(p, `UNC\`) || strings.HasPrefix(p, `Volume{`) {
		return ""
	}
	return filepath.Clean(p)
}
