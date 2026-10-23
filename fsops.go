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

// progressFn reports the item being worked on; returning false cancels.
type progressFn func(item string) bool

func isSubPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

// copyTree copies a file, symlink or directory tree from src to dst.
func copyTree(src, dst string, overwrite bool, progress progressFn) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.IsDir() && isSubPath(src, dst) {
		return fmt.Errorf("cannot copy %s into itself", filepath.Base(src))
	}
	return copyNode(src, dst, info, overwrite, progress)
}

func copyNode(src, dst string, info os.FileInfo, overwrite bool, progress progressFn) error {
	if !progress(src) {
		return errCancelled
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
			if err := copyNode(filepath.Join(src, de.Name()), filepath.Join(dst, de.Name()), ci, overwrite, progress); err != nil {
				return err
			}
		}
		return os.Chtimes(dst, info.ModTime(), info.ModTime())
	default:
		if _, err := os.Lstat(dst); err == nil && !overwrite {
			return nil
		}
		return copyFile(src, dst, info)
	}
}

func copyFile(src, dst string, info os.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm()|0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}

// moveTree renames src to dst, falling back to copy and delete across
// devices.
func moveTree(src, dst string, overwrite bool, progress progressFn) error {
	if !progress(src) {
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
	if err := copyTree(src, dst, true, progress); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func sameFile(a, b string) bool {
	ia, err1 := os.Stat(a)
	ib, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ia, ib)
}

func deleteTree(path string, progress progressFn) error {
	if !progress(path) {
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
