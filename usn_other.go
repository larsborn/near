//go:build !windows

package main

import "errors"

// No change journal on this platform: the index catches up by rebuilding.

type journalPos struct {
	ID   uint64
	Next int64
}

func journalsSupported() bool { return false }

func journalVolume(root string) string { return "" }

func journalPositions(roots []string) map[string]journalPos { return nil }

var errJournalUnusable = errors.New("change journal unusable")

func journalChangedDirs(vol string, pos journalPos) ([]string, journalPos, error) {
	return nil, pos, errJournalUnusable
}
