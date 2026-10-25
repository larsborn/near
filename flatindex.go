package main

import (
	"bytes"
	"path/filepath"
	"sort"
	"strings"
)

// flatIndex is a compact, read-only view of the index built for speed: all
// names lowercased in one contiguous buffer, so a query is a substring
// search over ~100 MB of memory (bytes.Index is vectorised) rather than a
// walk over millions of Go strings. New entries are appended as the watcher
// reports them; entries that vanished are skipped by a liveness check and
// dropped by the periodic rebuild (compaction).
type flatIndex struct {
	names   []byte           // "\n" + lower(name) + "\n" + lower(name) ... + "\n"
	off     []int32          // offset of each name's first byte in names, ascending; one extra at the end
	dirIx   []int32          // directory of each entry (index into dirs)
	nameIx  []int32          // position of the entry's name in the directory's name list
	dirs    []string         // directories, sorted for the built part
	dirLow  []string         // lowercased directories with forward slashes
	dirPos  map[string]int32 // directory -> index into dirs
	dirFrom []int32          // first entry of each directory in the built part (grouped by directory)
	built   int              // entries from the build; later appends follow ungrouped
	count   int
}

// flatAdd is an entry to append to the view.
type flatAdd struct {
	dir, name string
	nameIx    int32
}

// searchCountCap bounds the work of a broad query: counting stops here and
// the dialog shows "10,000+".
const searchCountCap = 10000

// buildFlatIndex makes the flat view from the directory map.
func buildFlatIndex(dirs map[string][]string, count int) *flatIndex {
	keys := make([]string, 0, len(dirs))
	for d := range dirs {
		keys = append(keys, d)
	}
	sort.Strings(keys)
	f := &flatIndex{
		dirs:    keys,
		dirLow:  make([]string, len(keys)),
		dirPos:  make(map[string]int32, len(keys)),
		dirFrom: make([]int32, len(keys)+1),
		off:     make([]int32, 0, count+1),
		dirIx:   make([]int32, 0, count),
		nameIx:  make([]int32, 0, count),
		names:   make([]byte, 0, count*16),
	}
	f.names = append(f.names, '\n')
	for di, d := range keys {
		f.dirLow[di] = strings.ToLower(filepath.ToSlash(d))
		f.dirPos[d] = int32(di)
		f.dirFrom[di] = int32(len(f.off))
		for ni, n := range dirs[d] {
			f.off = append(f.off, int32(len(f.names)))
			f.dirIx = append(f.dirIx, int32(di))
			f.nameIx = append(f.nameIx, int32(ni))
			f.names = append(f.names, lowerASCII(n)...)
			f.names = append(f.names, '\n')
		}
	}
	f.dirFrom[len(keys)] = int32(len(f.off))
	f.off = append(f.off, int32(len(f.names)))
	f.count = len(f.dirIx)
	f.built = f.count
	return f
}

// appendEntries adds entries to the view in place (the caller holds the
// index write lock).
func (f *flatIndex) appendEntries(adds []flatAdd) {
	for _, ad := range adds {
		di, ok := f.dirPos[ad.dir]
		if !ok {
			di = int32(len(f.dirs))
			f.dirs = append(f.dirs, ad.dir)
			f.dirLow = append(f.dirLow, strings.ToLower(filepath.ToSlash(ad.dir)))
			f.dirPos[ad.dir] = di
		}
		f.off[len(f.off)-1] = int32(len(f.names)) // replace the end marker
		f.dirIx = append(f.dirIx, di)
		f.nameIx = append(f.nameIx, ad.nameIx)
		f.names = append(f.names, lowerASCII(ad.name)...)
		f.names = append(f.names, '\n')
		f.off = append(f.off, int32(len(f.names)))
		f.count++
	}
}

// lowerASCII lowercases fast for ASCII and falls back to Unicode lowering.
func lowerASCII(s string) string {
	if !hasUpper(s) {
		return s
	}
	if isASCII(s) {
		b := []byte(s)
		for i, c := range b {
			if c >= 'A' && c <= 'Z' {
				b[i] = c + 'a' - 'A'
			}
		}
		return string(b)
	}
	return strings.ToLower(s)
}

// scanLiteral returns the longest literal substring of a term that a fast
// buffer scan can use: the term itself for plain terms, the longest run
// between wildcards for globs, nothing for path terms.
func scanLiteral(m matcher) string {
	if m.path {
		return ""
	}
	if !m.glob {
		return m.s
	}
	best, cur := "", ""
	for _, r := range m.s {
		if strings.ContainsRune("*?[]", r) {
			if len(cur) > len(best) {
				best = cur
			}
			cur = ""
			continue
		}
		cur += string(r)
	}
	if len(cur) > len(best) {
		best = cur
	}
	return best
}

// entryAt returns the entry whose name contains byte position pos.
func (f *flatIndex) entryAt(pos int) int {
	// off is ascending; find the last offset <= pos.
	i := sort.Search(len(f.off), func(i int) bool { return int(f.off[i]) > pos })
	return i - 1
}

func (f *flatIndex) nameOf(k int) []byte { return f.names[f.off[k] : f.off[k+1]-1] }

// matches reports whether entry k satisfies every term.
func (f *flatIndex) matches(k int, ms []matcher) bool {
	name := f.nameOf(k)
	for _, m := range ms {
		switch {
		case m.glob:
			if ok, _ := filepath.Match(m.s, string(name)); !ok {
				return false
			}
		case m.path:
			dir := f.dirLow[f.dirIx[k]]
			// the term may span the directory/name boundary: check the joined form
			if !strings.Contains(dir, m.s) && !bytes.Contains(name, []byte(m.s)) {
				joined := make([]byte, 0, len(dir)+1+len(name))
				joined = append(append(append(joined, dir...), '/'), name...)
				if !bytes.Contains(joined, []byte(m.s)) {
					return false
				}
			}
		default:
			if !bytes.Contains(name, []byte(m.s)) {
				return false
			}
		}
	}
	return true
}

// search returns up to max matching paths (sorted), the number of matches
// counted, and whether counting stopped at searchCountCap.
func (f *flatIndex) search(ms []matcher, dirs map[string][]string, max int) (out []string, total int, capped bool) {
	// The term with the longest literal drives the buffer scan; it is the
	// most selective. Path-only queries narrow by directory instead.
	scanTerm := ""
	for _, m := range ms {
		if lit := scanLiteral(m); len(lit) > len(scanTerm) {
			scanTerm = lit
		}
	}
	emit := func(k int) bool {
		// The map may have moved on since this view was built: verify the
		// entry still exists before reporting it.
		d := f.dirs[f.dirIx[k]]
		names := dirs[d]
		ni := int(f.nameIx[k])
		if ni >= len(names) || lowerASCII(names[ni]) != string(f.nameOf(k)) {
			return true
		}
		total++
		if len(out) < max {
			out = append(out, filepath.Join(d, names[ni]))
		}
		if total >= searchCountCap {
			capped = true
			return false
		}
		return true
	}
	switch {
	case scanTerm != "":
		term := []byte(scanTerm)
		pos := 1
		for {
			i := bytes.Index(f.names[pos:], term)
			if i < 0 {
				break
			}
			k := f.entryAt(pos + i)
			if f.matches(k, ms) && !emit(k) {
				break
			}
			pos = int(f.off[k+1]) // continue after this entry
		}
	default:
		// Path terms only: the part before the last slash must occur in
		// the directory, which narrows the directories to visit; matches()
		// then checks the precise joined form.
		dirPart := ""
		for _, m := range ms {
			if i := strings.LastIndex(m.s, "/"); i >= 0 && len(m.s[:i+1]) > len(dirPart) {
				dirPart = m.s[:i+1]
			}
		}
		stop := false
		for di := 0; di+1 < len(f.dirFrom) && !stop; di++ {
			if dirPart != "" && !strings.Contains(f.dirLow[di]+"/", dirPart) {
				continue
			}
			for k := int(f.dirFrom[di]); k < int(f.dirFrom[di+1]); k++ {
				if f.matches(k, ms) && !emit(k) {
					stop = true
					break
				}
			}
		}
		for k := f.built; k < f.count && !stop; k++ { // entries appended since the build
			if f.matches(k, ms) && !emit(k) {
				break
			}
		}
	}
	sort.Strings(out)
	return
}
