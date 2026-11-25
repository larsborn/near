package main

import (
	"fmt"
	"os"
	"strings"

	runewidth "github.com/mattn/go-runewidth"
)

// conflict is a destination that is already taken: what is there now, and
// what would replace it. The "Destination exists" box shows both, so that
// overwriting is decided on the facts rather than on a count.
type conflict struct {
	name string      // the destination's own name, for the list
	path string      // its full path, shown when it is the only one
	dst  os.FileInfo // what is already there
	src  os.FileInfo // what would replace it; nil if it could not be read
}

// conflictsShown bounds the height of the box; the rest are counted.
const conflictsShown = 8

// conflictLines describes the clashes for the confirmation box. A single
// one gets the existing entry and the new one on a row each with their
// full timestamps; several get one row each, with the sizes side by side
// and the note carrying the rest, so the box stays narrow.
func (a *App) conflictLines(cs []conflict) []string {
	if len(cs) == 0 {
		return nil
	}
	stamp := func(fi os.FileInfo) string {
		if fi == nil {
			return "?"
		}
		return fi.ModTime().Format(a.cfg.DateFormat + " " + a.cfg.TimeFormat)
	}
	size := func(fi os.FileInfo) string {
		switch {
		case fi == nil:
			return "?"
		case fi.IsDir():
			return "<DIR>"
		}
		return groupDigits(fi.Size())
	}
	// Column widths, so the numbers line up however long they are.
	sw, nw := 0, 0
	for _, c := range cs {
		sw = maxInt(sw, maxInt(len(size(c.dst)), len(size(c.src))))
		nw = maxInt(nw, runewidth.StringWidth(c.name))
	}

	if len(cs) == 1 {
		c := cs[0]
		row := fmt.Sprintf("  new       %s  %s", padLeft(size(c.src), sw), stamp(c.src))
		if n := compareNote(c.dst, c.src); n != "" {
			row += "   " + n
		}
		return []string{
			fitRight(c.path, 64),
			"",
			fmt.Sprintf("  existing  %s  %s", padLeft(size(c.dst), sw), stamp(c.dst)),
			row,
		}
	}

	// The sizes are bare numbers, so the heading says which is which.
	lines := []string{fmt.Sprintf("%d entries already exist in the destination (existing -> new):", len(cs)), ""}
	for i, c := range cs {
		if i == conflictsShown {
			lines = append(lines, fmt.Sprintf("  … and %d more", len(cs)-i))
			break
		}
		row := fmt.Sprintf("  %s  %s -> %s", padRight(c.name, nw), padLeft(size(c.dst), sw), padLeft(size(c.src), sw))
		if n := compareNote(c.dst, c.src); n != "" {
			row += "   " + n
		}
		lines = append(lines, row)
	}
	return lines
}

// compareNote says how the new entry differs from the one already there,
// which is what the decision turns on. Times are compared the strict way
// newerThan does, so this never disagrees with the "Only newer" policy.
func compareNote(dst, src os.FileInfo) string {
	if dst == nil || src == nil {
		return ""
	}
	if dst.IsDir() != src.IsDir() {
		if src.IsDir() {
			return "a directory would replace a file"
		}
		return "a file would replace a directory"
	}
	var parts []string
	switch {
	case src.ModTime().After(dst.ModTime()):
		parts = append(parts, "newer")
	case src.ModTime().Before(dst.ModTime()):
		parts = append(parts, "older")
	}
	if !src.IsDir() {
		switch {
		case src.Size() > dst.Size():
			parts = append(parts, "larger")
		case src.Size() < dst.Size():
			parts = append(parts, "smaller")
		}
	}
	if len(parts) == 0 {
		if src.IsDir() {
			return "same time"
		}
		return "same size and time"
	}
	return strings.Join(parts, ", ")
}
