package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTransferQueueViewAndHistory(t *testing.T) {
	a, data := testApp(t)
	paceCopies(t)
	other := filepath.Join(filepath.Dir(data), "dest")
	os.MkdirAll(other, 0o755)
	big := bytes.Repeat([]byte{3}, 8*copyChunk)
	for _, n := range []string{"a.bin", "b.bin"} {
		os.WriteFile(filepath.Join(data, n), big, 0o644)
	}
	a.reloadAll()
	a.navigate(a.other(), other, "")

	// One finished transfer in the history.
	a.tab().SetCursorOn("photo.jpg")
	key(a, "F5")
	key(a, "Enter")
	waitTransfers(t, a)

	// Two big ones in the background, then a foreground one that must wait.
	for _, n := range []string{"a.bin", "b.bin"} {
		a.tab().SetCursorOn(n)
		key(a, "F5")
		key(a, "Alt+B")
	}
	a.tab().SetCursorOn("file1.txt")
	key(a, "F5")
	key(a, "Enter")
	d, ok := a.top().(*TransferDialog)
	if !ok {
		t.Fatalf("expected the progress box, got %T", a.top())
	}
	txt := screenText(a)
	for _, want := range []string{"Waiting for 2 earlier transfer(s)", "Copying 1 entries to", "done", "queued", "▶", "this one"} {
		if !strings.Contains(txt, want) {
			t.Fatalf("waiting box should list the queue, missing %q:\n%s", want, txt)
		}
	}
	if !strings.Contains(txt, "%") && !strings.Contains(txt, "running") {
		t.Fatalf("the running transfer should show its progress:\n%s", txt)
	}
	waitTransfers(t, a)
	_ = d

	// Ctrl+B lists the finished transfers with their outcome.
	key(a, "Ctrl+B")
	l, ok := a.top().(*ListDialog)
	if !ok || l.Title != "Transfers" {
		t.Fatalf("expected the transfers list, got %T", a.top())
	}
	if len(l.Items) != 4 {
		t.Fatalf("expected 4 transfers in the list, got %d", len(l.Items))
	}
	for _, it := range l.Items {
		if it.Right != "done" {
			t.Fatalf("finished transfer should read done, got %q", it.Right)
		}
	}
	// Enter on a finished one shows its final box.
	key(a, "Enter")
	if _, ok := a.top().(*TransferDialog); !ok {
		t.Fatalf("expected the final box, got %T", a.top())
	}
	if txt := screenText(a); !strings.Contains(txt, "Finished: done") || !strings.Contains(txt, "100%") {
		t.Fatalf("final box:\n%s", txt)
	}
	key(a, "Enter")
	if len(a.dialogs) != 0 {
		t.Fatal("Enter should close the final box")
	}
	// A cancelled queued transfer is remembered as cancelled.
	os.Remove(filepath.Join(other, "a.bin"))
	os.Remove(filepath.Join(other, "b.bin"))
	a.tab().SetCursorOn("a.bin")
	key(a, "F5")
	key(a, "Alt+B")
	a.tab().SetCursorOn("b.bin")
	key(a, "F5")
	key(a, "Alt+B")
	pend := a.transfers.pending()
	a.transfers.remove(pend[len(pend)-1])
	waitTransfers(t, a)
	cancelled := 0
	for _, tr := range a.transfers.all() {
		if _, status := tr.describe(); status == "cancelled" {
			cancelled++
		}
	}
	if cancelled != 1 {
		t.Fatalf("expected one cancelled transfer in the history, got %d", cancelled)
	}
}
