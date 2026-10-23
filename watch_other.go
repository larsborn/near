//go:build !windows

package main

// newWatcher returns the portable per-directory watcher (inotify on Linux,
// kqueue on macOS and the BSDs).
func newWatcher() (fsWatcher, error) { return newFsnotifyWatcher() }
