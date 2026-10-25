package main

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
)

// Index is an Everything-like in-memory list of every file name below the
// configured roots, kept current by a filesystem watcher. Names are stored
// per directory so that changes are cheap to apply; searching scans the
// names case-insensitively without allocating.
type Index struct {
	mu      sync.RWMutex
	dirs    map[string][]string // directory -> entry names; every directory is a key
	count   int
	builtAt time.Time
	err     error

	building atomic.Bool
	progress atomic.Int64

	roots     []string
	exclude   excludeSet
	cacheFile string
	screen    tcell.Screen
	watch     bool
	maxWatch  int
	dirty     bool
	lastSave  time.Time

	flat        *flatIndex // fast search view, rebuilt in the background
	flatBusy    bool
	flatAgain   bool
	flatCompact bool // a compaction is scheduled

	newWatcher func() (fsWatcher, error)
	watcher    fsWatcher
	watchStop  chan struct{}
	watchDone  chan struct{}
	watchN     int
	watchLimit bool
	watchErr   error
}

// EventIndexDone is posted to the screen when a build ends.
type EventIndexDone struct{ tcell.EventTime }

// EventIndexChanged is posted when the watcher applied changes.
type EventIndexChanged struct{ tcell.EventTime }

func newIndex() *Index {
	return &Index{dirs: map[string][]string{}, newWatcher: newWatcher, maxWatch: 100000}
}

// Configure sets roots, excludes, cache location and watch settings.
func (ix *Index) Configure(cfg *Config, screen tcell.Screen) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.roots = ix.roots[:0]
	for _, r := range cfg.SearchRoots {
		ix.roots = append(ix.roots, expandPath(r))
	}
	ix.roots = outerRoots(ix.roots)
	ix.exclude = newExcludeSet(cfg.SearchExclude)
	ix.cacheFile = filepath.Join(cfg.dir, "index.gob")
	ix.screen = screen
	ix.watch = cfg.SearchWatch
	if cfg.SearchWatchMaxDirs > 0 {
		ix.maxWatch = cfg.SearchWatchMaxDirs
	}
}

func (ix *Index) Count() int {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return ix.count
}

func (ix *Index) BuiltAt() time.Time {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return ix.builtAt
}

func (ix *Index) Building() bool { return ix.building.Load() }

func (ix *Index) Err() error {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return ix.err
}

// WatchStatus describes the watcher for the status line.
func (ix *Index) WatchStatus() string {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	switch {
	case ix.watcher == nil && ix.watchErr != nil:
		return "not watching: " + ix.watchErr.Error()
	case ix.watcher == nil:
		return "not watching"
	case ix.watchLimit:
		return fmt.Sprintf("watching %s of %s directories", groupDigits(int64(ix.watchN)), groupDigits(int64(len(ix.dirs))))
	case ix.watcher.Recursive():
		return "watching"
	default:
		return fmt.Sprintf("watching %s directories", groupDigits(int64(ix.watchN)))
	}
}

// ---------------------------------------------------------------- excludes

// excludeSet holds directory names skipped anywhere and exact paths skipped.
type excludeSet struct {
	names map[string]bool
	paths []string // lowercased, forward slashes
}

func newExcludeSet(exclude []string) excludeSet {
	e := excludeSet{names: map[string]bool{}}
	for _, x := range exclude {
		if strings.ContainsAny(x, `/\`) {
			e.paths = append(e.paths, strings.ToLower(filepath.ToSlash(expandPath(x))))
		} else {
			e.names[strings.ToLower(x)] = true
		}
	}
	return e
}

// skipDir reports whether the directory at path should not be indexed.
func (e excludeSet) skipDir(path string) bool {
	if e.names[strings.ToLower(filepath.Base(path))] {
		return true
	}
	lp := strings.ToLower(filepath.ToSlash(path))
	for _, sp := range e.paths {
		if lp == sp {
			return true
		}
	}
	return false
}

// skipPath reports whether any directory component of path is excluded.
func (e excludeSet) skipPath(path string) bool {
	for d := filepath.Dir(path); ; d = filepath.Dir(d) {
		if e.skipDir(d) {
			return true
		}
		if parent := filepath.Dir(d); parent == d {
			return false
		}
	}
}

// ---------------------------------------------------------------- build

// Build walks the roots synchronously and replaces the index contents.
func (ix *Index) Build() {
	if !ix.building.CompareAndSwap(false, true) {
		return
	}
	defer ix.building.Store(false)
	ix.progress.Store(0)
	ix.mu.RLock()
	roots, exclude := append([]string{}, ix.roots...), ix.exclude
	ix.mu.RUnlock()

	dirs := map[string][]string{}
	count := 0
	var firstErr error
	for _, root := range roots {
		root = filepath.Clean(root)
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if p == root && firstErr == nil {
					firstErr = err
				}
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if p == root {
				dirs[p] = nil
				return nil
			}
			if d.IsDir() {
				if exclude.skipDir(p) || d.Type()&fs.ModeSymlink != 0 {
					return fs.SkipDir
				}
				dirs[p] = nil
			}
			parent := filepath.Dir(p)
			dirs[parent] = append(dirs[parent], d.Name())
			count++
			if count%2048 == 0 {
				ix.progress.Store(int64(count))
			}
			return nil
		})
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	ix.mu.Lock()
	ix.dirs, ix.count, ix.err, ix.builtAt = dirs, count, firstErr, time.Now()
	ix.dirty = true
	ix.mu.Unlock()
	ix.rebuildFlat()
}

// rebuildFlat builds the flat search view from the current map, in the
// calling goroutine. If a build is already running the request is queued.
func (ix *Index) rebuildFlat() {
	ix.mu.Lock()
	if ix.flatBusy {
		ix.flatAgain = true
		ix.mu.Unlock()
		return
	}
	ix.flatBusy = true
	ix.mu.Unlock()
	for {
		ix.mu.RLock()
		f := buildFlatIndex(ix.dirs, ix.count)
		ix.mu.RUnlock()
		ix.mu.Lock()
		ix.flat = f
		again := ix.flatAgain
		ix.flatAgain = false
		if !again {
			ix.flatBusy = false
			ix.mu.Unlock()
			return
		}
		ix.mu.Unlock()
	}
}

// flatCompactAfter is how long after a change the flat view is rebuilt to
// drop entries that vanished; new entries are appended immediately.
const flatCompactAfter = 30 * time.Second

// scheduleFlat compacts the flat view in the background a while after the
// last change, so bursts of watcher events cause one rebuild.
func (ix *Index) scheduleFlat() {
	ix.mu.Lock()
	if ix.flatCompact {
		ix.mu.Unlock()
		return
	}
	ix.flatCompact = true
	ix.mu.Unlock()
	go func() {
		time.Sleep(flatCompactAfter)
		ix.mu.Lock()
		ix.flatCompact = false
		ix.mu.Unlock()
		ix.rebuildFlat()
	}()
}

// RebuildAsync rebuilds in the background, saves the cache, restarts the
// watcher and posts EventIndexDone.
func (ix *Index) RebuildAsync() {
	if ix.Building() {
		return
	}
	go func() {
		ix.StopWatch()
		ix.Build()
		ix.post(&EventIndexDone{})
		if ix.watch {
			ix.StartWatch()
		}
		_ = ix.saveCache()
	}()
}

func (ix *Index) post(ev tcell.Event) {
	ix.mu.RLock()
	s := ix.screen
	ix.mu.RUnlock()
	if s == nil {
		return
	}
	switch e := ev.(type) {
	case *EventIndexDone:
		e.SetEventNow()
	case *EventIndexChanged:
		e.SetEventNow()
	}
	_ = s.PostEvent(ev)
}

// closeSaveMax is the index size up to which Close still writes the cache;
// larger indexes take hundreds of milliseconds to encode, which would make
// quitting feel slow, so they rely on the periodic save and the save after
// each rebuild instead.
const closeSaveMax = 200000

// Close stops the watcher and writes the cache if that is cheap.
func (ix *Index) Close() {
	ix.StopWatch()
	if ix.Count() <= closeSaveMax {
		_ = ix.saveCache()
	}
}

// ---------------------------------------------------------------- cache

type indexCache struct {
	BuiltAt time.Time
	Dirs    map[string][]string
	Count   int
}

var errOldCache = errors.New("index cache has an old format")

func (ix *Index) saveCache() error {
	ix.mu.RLock()
	if !ix.dirty || ix.cacheFile == "" {
		ix.mu.RUnlock()
		return nil
	}
	c := indexCache{BuiltAt: ix.builtAt, Dirs: ix.dirs, Count: ix.count}
	path := ix.cacheFile
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		ix.mu.RUnlock()
		return err
	}
	err = gob.NewEncoder(f).Encode(&c)
	ix.mu.RUnlock()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	ix.mu.Lock()
	ix.dirty, ix.lastSave = false, time.Now()
	ix.mu.Unlock()
	return nil
}

// LoadCache fills the index from the cache file written by a previous run.
func (ix *Index) LoadCache() error {
	ix.mu.RLock()
	path := ix.cacheFile
	ix.mu.RUnlock()
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var c indexCache
	if err := gob.NewDecoder(f).Decode(&c); err != nil {
		return err
	}
	if c.Dirs == nil {
		return errOldCache
	}
	ix.mu.Lock()
	ix.dirs, ix.count, ix.builtAt, ix.dirty = c.Dirs, c.Count, c.BuiltAt, false
	ix.mu.Unlock()
	go ix.rebuildFlat()
	return nil
}

// ---------------------------------------------------------------- search

// matcher is one query term.
type matcher struct {
	s    string
	path bool // contains a separator: matches the full path
	glob bool // contains * ? [: glob against the base name
}

func parseTerms(query string) []matcher {
	terms := strings.Fields(strings.ToLower(query))
	ms := make([]matcher, 0, len(terms))
	for _, t := range terms {
		ms = append(ms, matcher{
			s:    strings.ReplaceAll(t, `\`, `/`),
			path: strings.ContainsAny(t, `/\`),
			glob: strings.ContainsAny(t, "*?["),
		})
	}
	return ms
}

// Search matches every whitespace-separated term of query. A term matches
// the base name; if it contains a path separator it matches the full path.
// Terms with * ? or [ are glob patterns against the base name. Up to max
// results are returned, sorted, with the number of matches counted and
// whether counting stopped at searchCountCap. The flat view is used when
// it exists; right after a build the slower map scan serves.
func (ix *Index) Search(query string, max int) (out []string, total int, capped bool) {
	ms := parseTerms(query)
	if len(ms) == 0 {
		return nil, 0, false
	}
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	if ix.flat != nil {
		return ix.flat.search(ms, ix.dirs, max)
	}
	needPath := false
	for _, m := range ms {
		needPath = needPath || m.path
	}
	var buf []byte
	for dir, names := range ix.dirs {
		for _, name := range names {
			if needPath {
				buf = append(buf[:0], dir...)
				buf = append(buf, '/')
				buf = append(buf, name...)
			}
			ok := true
			for _, m := range ms {
				switch {
				case m.glob:
					n := name
					if hasUpper(n) {
						n = strings.ToLower(n)
					}
					if matched, _ := filepath.Match(m.s, n); !matched {
						ok = false
					}
				case m.path:
					if !containsFold(buf, m.s) {
						ok = false
					}
				default:
					if !containsFold(name, m.s) {
						ok = false
					}
				}
				if !ok {
					break
				}
			}
			if ok {
				total++
				if len(out) < max {
					out = append(out, filepath.Join(dir, name))
				}
				if total >= searchCountCap {
					sort.Strings(out)
					return out, total, true
				}
			}
		}
	}
	sort.Strings(out)
	return out, total, false
}

// containsFold reports whether s contains the lowercase term, ignoring
// ASCII case and treating / and \ alike. Non-ASCII text falls back to a
// full Unicode lowering.
func containsFold[T string | []byte](s T, term string) bool {
	if term == "" {
		return true
	}
	if !isASCII(s) || !isASCII(term) {
		return strings.Contains(strings.ReplaceAll(strings.ToLower(string(s)), `\`, `/`), term)
	}
	n := len(term)
	for i := 0; i+n <= len(s); i++ {
		j := 0
		for ; j < n; j++ {
			c := s[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			} else if c == '\\' {
				c = '/'
			}
			if c != term[j] {
				break
			}
		}
		if j == n {
			return true
		}
	}
	return false
}

func hasUpper(s string) bool {
	for i := 0; i < len(s); i++ {
		if c := s[i]; (c >= 'A' && c <= 'Z') || c >= 0x80 {
			return true
		}
	}
	return false
}

func isASCII[T string | []byte](s T) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------- watching

const (
	watchBatchDelay = 300 * time.Millisecond
	watchBatchMax   = 2 * time.Second
	cacheSaveEvery  = 2 * time.Minute
)

// StartWatch starts watching the roots and applying changes to the index.
func (ix *Index) StartWatch() {
	ix.StopWatch()
	w, err := ix.newWatcher()
	ix.mu.Lock()
	if err != nil {
		ix.watchErr = err
		ix.mu.Unlock()
		return
	}
	ix.watcher, ix.watchErr, ix.watchN, ix.watchLimit = w, nil, 0, false
	roots := append([]string{}, ix.roots...)
	var dirs []string
	if !w.Recursive() {
		for d := range ix.dirs {
			dirs = append(dirs, d)
		}
		sort.Strings(dirs) // parents before children, so shallow dirs win the cap
	}
	ix.mu.Unlock()

	if w.Recursive() {
		for _, r := range roots {
			if err := w.Add(r); err != nil {
				ix.mu.Lock()
				ix.watchErr = err
				ix.mu.Unlock()
			} else {
				ix.mu.Lock()
				ix.watchN++
				ix.mu.Unlock()
			}
		}
	} else {
		for _, d := range dirs {
			if !ix.addDirWatch(d) {
				break
			}
		}
	}
	stop, done := make(chan struct{}), make(chan struct{})
	ix.mu.Lock()
	ix.watchStop, ix.watchDone = stop, done
	ix.mu.Unlock()
	go ix.watchLoop(w, stop, done)
}

// addDirWatch adds one directory watch (non-recursive backends) and
// reports false once the cap or a system limit is hit.
func (ix *Index) addDirWatch(dir string) bool {
	ix.mu.Lock()
	w, n, max := ix.watcher, ix.watchN, ix.maxWatch
	ix.mu.Unlock()
	if w == nil {
		return false
	}
	if n >= max {
		ix.mu.Lock()
		ix.watchLimit = true
		ix.mu.Unlock()
		return false
	}
	if err := w.Add(dir); err != nil {
		ix.mu.Lock()
		if os.IsNotExist(err) {
			ix.mu.Unlock()
			return true
		}
		ix.watchLimit, ix.watchErr = true, err
		ix.mu.Unlock()
		return false
	}
	ix.mu.Lock()
	ix.watchN++
	ix.mu.Unlock()
	return true
}

// StopWatch stops the watcher, if any.
func (ix *Index) StopWatch() {
	ix.mu.Lock()
	w, stop, done := ix.watcher, ix.watchStop, ix.watchDone
	ix.watcher, ix.watchStop, ix.watchDone = nil, nil, nil
	ix.mu.Unlock()
	if w == nil {
		return
	}
	close(stop)
	_ = w.Close()
	<-done
}

func (ix *Index) watchLoop(w fsWatcher, stop, done chan struct{}) {
	defer close(done)
	var batch []fsEvent
	var flush <-chan time.Time
	var first time.Time
	saveTick := time.NewTicker(cacheSaveEvery)
	defer saveTick.Stop()
	for {
		select {
		case <-stop:
			return
		case ev, ok := <-w.Events():
			if !ok {
				return
			}
			if ev.Op == fsOverflow {
				batch = nil
				flush = nil
				go ix.RebuildAsync()
				continue
			}
			if len(batch) == 0 {
				first = time.Now()
			}
			batch = append(batch, ev)
			if time.Since(first) > watchBatchMax {
				ix.apply(batch)
				batch, flush = nil, nil
			} else {
				flush = time.After(watchBatchDelay)
			}
		case <-flush:
			ix.apply(batch)
			batch, flush = nil, nil
		case <-saveTick.C:
			_ = ix.saveCache()
		}
	}
}

// apply folds a batch of change events into the index.
func (ix *Index) apply(batch []fsEvent) {
	if len(batch) == 0 {
		return
	}
	removes := map[string]bool{}
	var creates []string
	for _, ev := range batch {
		p := filepath.Clean(ev.Path)
		if ix.exclude.skipPath(p) {
			continue
		}
		removes[p] = true // a create is applied as remove + add, so it is idempotent
		if ev.Op == fsCreate {
			creates = append(creates, p)
		}
	}
	sort.Strings(creates) // parents before children

	ix.mu.Lock()
	var adds []flatAdd
	var prefixes []string
	for p := range removes {
		isDir, moved := ix.removeNameLocked(p)
		if moved != nil {
			adds = append(adds, *moved)
		}
		if isDir {
			prefixes = append(prefixes, p+string(filepath.Separator))
		}
	}
	if len(prefixes) > 0 {
		for d, names := range ix.dirs {
			for _, pre := range prefixes {
				if strings.HasPrefix(d, pre) || d == pre[:len(pre)-1] {
					ix.count -= len(names)
					delete(ix.dirs, d)
					break
				}
			}
		}
	}
	var newDirs []string
	for _, p := range creates {
		st, err := os.Lstat(p)
		if err != nil {
			continue
		}
		if _, known := ix.dirs[filepath.Dir(p)]; !known {
			continue // outside the index (parent excluded or not indexed)
		}
		if st.IsDir() && st.Mode()&os.ModeSymlink == 0 {
			if ix.exclude.skipDir(p) {
				continue
			}
			dirs, treeAdds := ix.addTreeLocked(p)
			newDirs = append(newDirs, dirs...)
			adds = append(adds, treeAdds...)
		} else if fa, ok := ix.addNameLocked(p); ok {
			adds = append(adds, fa)
		}
	}
	ix.dirty = true
	if ix.flat != nil {
		ix.flat.appendEntries(adds)
	}
	ix.mu.Unlock()
	ix.scheduleFlat()

	if w := ix.currentWatcher(); w != nil && !w.Recursive() {
		for _, d := range newDirs {
			if !ix.addDirWatch(d) {
				break
			}
		}
	}
	ix.post(&EventIndexChanged{})
}

func (ix *Index) currentWatcher() fsWatcher {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return ix.watcher
}

// removeNameLocked drops p from its parent's names (swapping the last name
// into its slot so the other positions stay valid for the flat view) and
// reports whether p is a directory key (whose subtree the caller must drop
// too). The moved name, if any, is returned so the flat view can relearn
// its position.
func (ix *Index) removeNameLocked(p string) (isDir bool, moved *flatAdd) {
	parent, name := filepath.Dir(p), filepath.Base(p)
	if names, ok := ix.dirs[parent]; ok {
		for i, n := range names {
			if n == name {
				last := len(names) - 1
				if i != last {
					names[i] = names[last]
					moved = &flatAdd{dir: parent, name: names[i], nameIx: int32(i)}
				}
				ix.dirs[parent] = names[:last]
				ix.count--
				break
			}
		}
	}
	_, isDir = ix.dirs[p]
	return isDir, moved
}

// addNameLocked adds p under its parent and returns the flat entry to
// append, or false if it was already there.
func (ix *Index) addNameLocked(p string) (flatAdd, bool) {
	parent, name := filepath.Dir(p), filepath.Base(p)
	for _, n := range ix.dirs[parent] {
		if n == name {
			return flatAdd{}, false
		}
	}
	ix.dirs[parent] = append(ix.dirs[parent], name)
	ix.count++
	return flatAdd{dir: parent, name: name, nameIx: int32(len(ix.dirs[parent]) - 1)}, true
}

// addTreeLocked indexes a new directory and everything below it, returning
// the directories added and the flat entries to append.
func (ix *Index) addTreeLocked(root string) ([]string, []flatAdd) {
	var added []string
	var adds []flatAdd
	if fa, ok := ix.addNameLocked(root); ok {
		adds = append(adds, fa)
	}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if p != root && (ix.exclude.skipDir(p) || d.Type()&fs.ModeSymlink != 0) {
				return fs.SkipDir
			}
			if _, ok := ix.dirs[p]; !ok {
				ix.dirs[p] = nil
			}
			added = append(added, p)
		}
		if p != root {
			if fa, ok := ix.addNameLocked(p); ok {
				adds = append(adds, fa)
			}
		}
		return nil
	})
	return added, adds
}

// outerRoots drops roots that lie inside another root (and duplicates), so
// ["~", "C:\\"] walks and watches the home directory only once.
func outerRoots(roots []string) []string {
	var out []string
	for i, r := range roots {
		r = filepath.Clean(r)
		inner := false
		for j, o := range roots {
			o = filepath.Clean(o)
			if j == i || (o == r && j > i) {
				continue
			}
			if o == r || strings.HasPrefix(foldPath(r), foldPath(withSep(o))) {
				inner = true
				break
			}
		}
		if !inner {
			out = append(out, r)
		}
	}
	return out
}

// foldPath lowercases a path where the file system ignores case.
func foldPath(p string) string {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.ToLower(p)
	}
	return p
}

// withSep returns the directory with one trailing separator.
func withSep(dir string) string {
	return strings.TrimRight(dir, `\/`) + string(filepath.Separator)
}
