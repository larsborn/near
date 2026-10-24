//go:build !windows

package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var trashName = func() string {
	if runtime.GOOS == "darwin" {
		return "Trash"
	}
	return "trash"
}()

// trashPaths moves the given absolute paths to the trash: the FreeDesktop
// trash (home trash, or <mount>/.Trash-<uid> for other filesystems) on
// Linux and the BSDs, ~/.Trash on macOS.
func trashPaths(paths []string) error {
	for _, p := range paths {
		if err := trashOne(p); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(p), err)
		}
	}
	return nil
}

func trashOne(p string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		dir := filepath.Join(home, ".Trash")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		return os.Rename(p, filepath.Join(dir, uniqueName(dir, filepath.Base(p))))
	}
	// FreeDesktop: try the home trash first.
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		data = filepath.Join(home, ".local", "share")
	}
	if err := trashInto(filepath.Join(data, "Trash"), p); err == nil {
		return nil
	} else if !isCrossDevice(err) {
		return err
	}
	// Other filesystem: <top of mount>/.Trash-<uid>.
	top, err := mountTop(p)
	if err != nil {
		return err
	}
	return trashInto(filepath.Join(top, fmt.Sprintf(".Trash-%d", os.Getuid())), p)
}

// trashInto moves p into the trash directory t, writing the .trashinfo
// record first as the spec requires.
func trashInto(t, p string) error {
	files, info := filepath.Join(t, "files"), filepath.Join(t, "info")
	if err := os.MkdirAll(files, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(info, 0o700); err != nil {
		return err
	}
	name := uniqueName(files, filepath.Base(p))
	rec := "[Trash Info]\nPath=" + url.PathEscape(p) + "\nDeletionDate=" + time.Now().Format("2006-01-02T15:04:05") + "\n"
	infoPath := filepath.Join(info, name+".trashinfo")
	if err := os.WriteFile(infoPath, []byte(rec), 0o600); err != nil {
		return err
	}
	if err := os.Rename(p, filepath.Join(files, name)); err != nil {
		os.Remove(infoPath)
		return err
	}
	return nil
}

func isCrossDevice(err error) bool {
	var errno syscall.Errno
	return errors.As(err, &errno) && errno == syscall.EXDEV
}

// mountTop walks up from p to the highest directory on the same device.
func mountTop(p string) (string, error) {
	st, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	dev := st.Sys().(*syscall.Stat_t).Dev
	dir := filepath.Dir(p)
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir, nil
		}
		pst, err := os.Stat(parent)
		if err != nil || pst.Sys().(*syscall.Stat_t).Dev != dev {
			return dir, nil
		}
		dir = parent
	}
}

// uniqueName returns name, or name with a counter, that does not exist in dir.
func uniqueName(dir, name string) string {
	if _, err := os.Lstat(filepath.Join(dir, name)); err != nil {
		return name
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 2; ; i++ {
		n := base + " (" + strconv.Itoa(i) + ")" + ext
		if _, err := os.Lstat(filepath.Join(dir, n)); err != nil {
			return n
		}
	}
}
