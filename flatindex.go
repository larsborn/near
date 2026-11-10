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
	names  []byte  // "\n" + lower(name) + "\n" + lower(name) ... + "\n"
	off    []int32 // offset of each name's first byte in names, ascending; one extra at the end
	dirIx  []int32 // directory of each entry (index into dirs)
	nameIx []int32 // position of the entry's name in the directory's name list
	dirs   []string

	// The lowercased, forward-slashed form of every directory, needed by
	// path terms. On a whole-machine index these were 846,000 separate Go
	// strings — ~96 MB — so they live in one buffer, each followed by the
	// separator that a prefix test wants anyway.
	lowBuf []byte
	lowOff []int32 // start of each directory in lowBuf, one extra at the end

	// dirs[:dirsBuilt] is sorted, so a directory is found by binary search
	// rather than by a map of every directory (another ~40 MB). Only the
	// ones appended after the build need one.
	dirsBuilt int
	dirAdded  map[string]int32

	dirFrom []int32 // first entry of each directory in the built part (grouped by directory)
	built   int     // entries from the build; later appends follow ungrouped
	count   int
}

// dirLow is directory di lowercased with forward slashes.
func (f *flatIndex) dirLow(di int32) []byte {
	return f.lowBuf[f.lowOff[di] : f.lowOff[di+1]-1]
}

// dirLowSlash is dirLow with the trailing separator, so that a path term
// ending in "/" can match the last component.
func (f *flatIndex) dirLowSlash(di int32) []byte {
	return f.lowBuf[f.lowOff[di]:f.lowOff[di+1]]
}

// dirIndex finds a directory's position in dirs.
func (f *flatIndex) dirIndex(dir string) (int32, bool) {
	built := f.dirs[:f.dirsBuilt]
	if i := sort.SearchStrings(built, dir); i < len(built) && built[i] == dir {
		return int32(i), true
	}
	di, ok := f.dirAdded[dir]
	return di, ok
}

// addDir appends a directory discovered after the build.
func (f *flatIndex) addDir(dir string) int32 {
	di := int32(len(f.dirs))
	f.dirs = append(f.dirs, dir)
	f.lowBuf = append(f.lowBuf, strings.ToLower(filepath.ToSlash(dir))...)
	f.lowBuf = append(f.lowBuf, '/')
	f.lowOff = append(f.lowOff, int32(len(f.lowBuf)))
	if f.dirAdded == nil {
		f.dirAdded = map[string]int32{}
	}
	f.dirAdded[dir] = di
	return di
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
//
// The sizes are measured first rather than estimated: on a large index the
// name buffer is hundreds of megabytes, and letting append grow it would
// copy it to a buffer of twice the size — a transient of several hundred
// megabytes that the process keeps. One pass over the map costs far less.
func buildFlatIndex(dirs map[string]nameBlob) *flatIndex {
	keys := make([]string, 0, len(dirs))
	total, nameBytes, keyBytes := 0, 0, 0
	for d, names := range dirs {
		keys = append(keys, d)
		keyBytes += len(d)
		names.each(func(_ int, n []byte) bool {
			total++
			nameBytes += len(n)
			return true
		})
	}
	sort.Strings(keys)
	// One '\n' before the first name and one after every name, and one '/'
	// after every directory. Lowercasing can lengthen a string in rare
	// non-ASCII cases, hence the small margins.
	nameCap := nameBytes + total + 1
	nameCap += nameCap / 64
	lowCap := keyBytes + len(keys)
	lowCap += lowCap / 64
	f := &flatIndex{
		dirs:      keys,
		dirsBuilt: len(keys),
		lowBuf:    make([]byte, 0, lowCap),
		lowOff:    make([]int32, 1, len(keys)+1),
		dirFrom:   make([]int32, len(keys)+1),
		off:       make([]int32, 0, total+1),
		dirIx:     make([]int32, 0, total),
		nameIx:    make([]int32, 0, total),
		names:     make([]byte, 0, nameCap),
	}
	f.names = append(f.names, '\n')
	for di, d := range keys {
		f.lowBuf = append(f.lowBuf, strings.ToLower(filepath.ToSlash(d))...)
		f.lowBuf = append(f.lowBuf, '/')
		f.lowOff = append(f.lowOff, int32(len(f.lowBuf)))
		f.dirFrom[di] = int32(len(f.off))
		dirs[d].each(func(ni int, n []byte) bool {
			f.off = append(f.off, int32(len(f.names)))
			f.dirIx = append(f.dirIx, int32(di))
			f.nameIx = append(f.nameIx, int32(ni))
			f.names = appendLower(f.names, n)
			f.names = append(f.names, '\n')
			return true
		})
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
		di, ok := f.dirIndex(ad.dir)
		if !ok {
			di = f.addDir(ad.dir)
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

// appendLower appends name to dst, lowercased. Working in bytes keeps the
// build from allocating a string per name.
func appendLower(dst, name []byte) []byte {
	if !isASCII(name) {
		return append(dst, strings.ToLower(string(name))...)
	}
	for _, c := range name {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		dst = append(dst, c)
	}
	return dst
}

// equalLower reports whether name lowercases to lower, which is already
// lowercased. Used to check a flat entry against the directory map.
func equalLower(name, lower []byte) bool {
	if !isASCII(name) {
		return strings.ToLower(string(name)) == string(lower)
	}
	if len(name) != len(lower) {
		return false
	}
	for i, c := range name {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != lower[i] {
			return false
		}
	}
	return true
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
			dir := f.dirLow(f.dirIx[k])
			// the term may span the directory/name boundary: check the joined form
			if !bytes.Contains(dir, m.b) && !bytes.Contains(name, m.b) {
				joined := make([]byte, 0, len(dir)+1+len(name))
				joined = append(append(append(joined, dir...), '/'), name...)
				if !bytes.Contains(joined, m.b) {
					return false
				}
			}
		default:
			if !bytes.Contains(name, m.b) {
				return false
			}
		}
	}
	return true
}

// search returns up to max matching paths (sorted), the number of matches
// counted, and whether counting stopped at searchCountCap.
func (f *flatIndex) search(ms []matcher, dirs map[string]nameBlob, max int) (out []string, total int, capped bool) {
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
		name := dirs[d].at(int(f.nameIx[k]))
		if name == nil || !equalLower(name, f.nameOf(k)) {
			return true
		}
		total++
		if len(out) < max {
			out = append(out, filepath.Join(d, string(name)))
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
		part := []byte(dirPart)
		for di := 0; di+1 < len(f.dirFrom) && !stop; di++ {
			if dirPart != "" && !bytes.Contains(f.dirLowSlash(int32(di)), part) {
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
