package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// errCancelled is returned when the user aborts a long operation.
var errCancelled = errors.New("cancelled")

// progressFn reports the item being worked on and, for a file copy, the
// bytes done so far and the total (-1 when unknown); returning false cancels.
type progressFn func(item string, done, total int64) bool

// copyChunk is the block size for file copies; progress is reported and
// cancellation checked after every block.
const copyChunk = 4 << 20

// copyPace, when set, is called after every chunk; tests use it to make
// copies take a predictable time so queue states can be observed.
var copyPace func()

func isSubPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

// copyFlags tune copyTree: Deref copies what symbolic links point to,
// KeepTimes preserves modification times.
type copyFlags struct {
	Overwrite bool
	Deref     bool
	KeepTimes bool
}

// copyTree copies a file, symlink or directory tree from src to dst.
func copyTree(src, dst string, flags copyFlags, progress progressFn) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if flags.Deref && info.Mode()&os.ModeSymlink != 0 {
		if info, err = os.Stat(src); err != nil {
			return err
		}
	}
	if info.IsDir() && isSubPath(src, dst) {
		return fmt.Errorf("cannot copy %s into itself", filepath.Base(src))
	}
	return copyNode(src, dst, info, flags, progress)
}

func copyNode(src, dst string, info os.FileInfo, flags copyFlags, progress progressFn) error {
	if !progress(src, -1, -1) {
		return errCancelled
	}
	overwrite := flags.Overwrite
	if flags.Deref && info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Stat(src)
		if err != nil {
			return nil // dangling link: nothing to copy
		}
		info = target
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(dst); err == nil {
			if !overwrite {
				return nil
			}
			if err := os.RemoveAll(dst); err != nil {
				return err
			}
		}
		return os.Symlink(target, dst)
	case info.IsDir():
		if err := os.MkdirAll(dst, info.Mode().Perm()|0o700); err != nil {
			return err
		}
		des, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, de := range des {
			ci, err := de.Info()
			if err != nil {
				return err
			}
			if err := copyNode(filepath.Join(src, de.Name()), filepath.Join(dst, de.Name()), ci, flags, progress); err != nil {
				return err
			}
		}
		if flags.KeepTimes {
			return os.Chtimes(dst, info.ModTime(), info.ModTime())
		}
		return nil
	default:
		if _, err := os.Lstat(dst); err == nil && !overwrite {
			return nil
		}
		return copyFile(src, dst, info, flags.KeepTimes, progress)
	}
}

// copyFile copies one file in blocks, reporting progress after each. On
// cancellation the partial destination is removed.
func copyFile(src, dst string, info os.FileInfo, keepTimes bool, progress progressFn) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm()|0o600)
	if err != nil {
		return err
	}
	total := info.Size()
	buf := make([]byte, copyChunk)
	var done int64
	for {
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				os.Remove(dst)
				return werr
			}
			done += int64(n)
			if copyPace != nil {
				copyPace()
			}
			if progress != nil && !progress(src, done, total) {
				out.Close()
				os.Remove(dst)
				return errCancelled
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			os.Remove(dst)
			return rerr
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	if keepTimes {
		return os.Chtimes(dst, info.ModTime(), info.ModTime())
	}
	return nil
}

// moveTree renames src to dst, falling back to copy and delete across
// devices.
func moveTree(src, dst string, flags copyFlags, progress progressFn) error {
	overwrite := flags.Overwrite
	if !progress(src, -1, -1) {
		return errCancelled
	}
	if _, err := os.Lstat(dst); err == nil {
		if !overwrite {
			return fmt.Errorf("%s exists", filepath.Base(dst))
		}
		if sameFile(src, dst) {
			return os.Rename(src, dst)
		}
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	flags.Overwrite = true
	if err := copyTree(src, dst, flags, progress); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

// newerThan reports whether src was modified after dst (true if dst is missing).
func newerThan(src, dst string) bool {
	ds, err := os.Stat(dst)
	if err != nil {
		return true
	}
	ss, err := os.Stat(src)
	return err == nil && ss.ModTime().After(ds.ModTime())
}

func sameFile(a, b string) bool {
	ia, err1 := os.Stat(a)
	ib, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ia, ib)
}

func deleteTree(path string, progress progressFn) error {
	if !progress(path, -1, -1) {
		return errCancelled
	}
	return os.RemoveAll(path)
}

// humanSize formats a byte count compactly ("12.3 M").
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	v := float64(n) / float64(div)
	suffix := "KMGTPE"[exp]
	if v >= 100 {
		return fmt.Sprintf("%.0f %c", v, suffix)
	}
	return fmt.Sprintf("%.1f %c", v, suffix)
}

// uniqueTempName returns a name in dir that does not exist yet.
func uniqueTempName(dir, base string, i int) string {
	for n := 0; ; n++ {
		name := fmt.Sprintf("%s.near-tmp-%d-%d", base, i, n)
		if _, err := os.Lstat(filepath.Join(dir, name)); os.IsNotExist(err) {
			return name
		}
	}
}

// renameMany applies a set of renames inside dir, using temporary names so
// that swaps and chains (a->b, b->c) work. Notes follow the entries.
func renameMany(dir string, pairs []renamePair, notesFile string) error {
	notes := loadNotes(dir, notesFile)
	var tmp []string
	for i, p := range pairs {
		if p.Old == p.New {
			tmp = append(tmp, "")
			continue
		}
		t := uniqueTempName(dir, p.Old, i)
		if err := os.Rename(filepath.Join(dir, p.Old), filepath.Join(dir, t)); err != nil {
			return err
		}
		tmp = append(tmp, t)
	}
	var firstErr error
	for i, p := range pairs {
		if tmp[i] == "" {
			continue
		}
		dst := filepath.Join(dir, p.New)
		if _, err := os.Lstat(dst); err == nil {
			// Target appeared meanwhile; leave the temp name rather than clobber.
			firstErr = fmt.Errorf("%s exists, left as %s", p.New, tmp[i])
			continue
		}
		if err := os.Rename(filepath.Join(dir, tmp[i]), dst); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if text, ok := notes[p.Old]; ok {
			delete(notes, p.Old)
			notes[p.New] = text
		}
	}
	_ = saveNotes(dir, notesFile, notes)
	return firstErr
}
