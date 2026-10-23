package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFileProgressAndCancel(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "big.bin")
	data := bytes.Repeat([]byte{7}, 3*copyChunk+123)
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(src)

	// Progress is reported per block with the running byte count.
	var calls []int64
	dst := filepath.Join(dir, "copy.bin")
	err := copyFile(src, dst, info, true, func(item string, done, total int64) bool {
		if total != int64(len(data)) || item != src {
			t.Fatalf("progress item=%s total=%d", item, total)
		}
		calls = append(calls, done)
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 4 || calls[len(calls)-1] != int64(len(data)) {
		t.Fatalf("progress calls = %v", calls)
	}
	if got, _ := os.ReadFile(dst); !bytes.Equal(got, data) {
		t.Fatal("copied data differs")
	}

	// Cancelling after the first block removes the partial file.
	dst2 := filepath.Join(dir, "cancelled.bin")
	n := 0
	err = copyFile(src, dst2, info, true, func(string, int64, int64) bool { n++; return n < 2 })
	if err != errCancelled {
		t.Fatalf("expected errCancelled, got %v", err)
	}
	if _, serr := os.Stat(dst2); !os.IsNotExist(serr) {
		t.Fatal("partial destination should have been removed")
	}
}
