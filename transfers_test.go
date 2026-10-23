package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// waitTransfers pumps events until the queue is idle.
func waitTransfers(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		pump(a)
		if !a.transfers.active() {
			pump(a)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("transfers did not finish")
}

func TestTransferForegroundBox(t *testing.T) {
	a, data := testApp(t)
	other := filepath.Join(filepath.Dir(data), "dest")
	os.MkdirAll(other, 0o755)
	a.navigate(a.other(), other, "")
	a.tab().SetCursorOn("photo.jpg")
	key(a, "F5")
	key(a, "Enter")
	if _, ok := a.top().(*TransferDialog); !ok {
		t.Fatalf("expected the progress box, got %T", a.top())
	}
	waitTransfers(t, a)
	if len(a.dialogs) != 0 {
		t.Fatal("the progress box should close when the transfer is done")
	}
	if _, err := os.Stat(filepath.Join(other, "photo.jpg")); err != nil {
		t.Fatal("file not copied")
	}
	if !strings.Contains(a.status, "Copying: 1 entries") {
		t.Fatalf("status = %q", a.status)
	}
}

func TestTransfersRunConsecutively(t *testing.T) {
	a, data := testApp(t)
	other := filepath.Join(filepath.Dir(data), "dest")
	os.MkdirAll(other, 0o755)
	big := bytes.Repeat([]byte{1}, 6*copyChunk)
	for _, n := range []string{"a.bin", "b.bin", "c.bin"} {
		os.WriteFile(filepath.Join(data, n), big, 0o644)
	}
	a.reloadAll()
	a.navigate(a.other(), other, "")

	// Queue three background copies via the dialog's Background button.
	for _, n := range []string{"a.bin", "b.bin", "c.bin"} {
		a.tab().SetCursorOn(n)
		key(a, "F5")
		key(a, "Alt+B") // the Background button's hotkey
	}
	pend := a.transfers.pending()
	if len(pend) != 3 {
		t.Fatalf("expected 3 pending transfers, got %d", len(pend))
	}
	if len(a.dialogs) != 0 {
		t.Fatal("background transfers must not open a progress box")
	}
	// Only one runs at a time, in order; the others wait with no start time.
	seen := map[int]bool{}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) && a.transfers.active() {
		p := a.transfers.pending()
		running := 0
		for i, x := range p {
			s := x.snapshot()
			if !s.start.IsZero() && !s.finished {
				running++
				seen[s.id] = true
				if i != 0 {
					t.Fatalf("a queued transfer is running out of order: %d", s.id)
				}
			}
		}
		if running > 1 {
			t.Fatalf("%d transfers running in parallel", running)
		}
		pump(a)
		time.Sleep(5 * time.Millisecond)
	}
	waitTransfers(t, a)
	for _, n := range []string{"a.bin", "b.bin", "c.bin"} {
		if st, err := os.Stat(filepath.Join(other, n)); err != nil || st.Size() != int64(len(big)) {
			t.Fatalf("%s not copied completely", n)
		}
	}
	if !strings.Contains(screenText(a), "Copying: 1 entries") {
		t.Fatalf("status after the queue: %q", a.status)
	}
}

func TestTransferSendToBackgroundAndCancel(t *testing.T) {
	a, data := testApp(t)
	other := filepath.Join(filepath.Dir(data), "dest")
	os.MkdirAll(other, 0o755)
	os.WriteFile(filepath.Join(data, "huge.bin"), bytes.Repeat([]byte{2}, 40*copyChunk), 0o644)
	a.reloadAll()
	a.navigate(a.other(), other, "")
	a.tab().SetCursorOn("huge.bin")
	key(a, "F5")
	key(a, "Enter")
	d, ok := a.top().(*TransferDialog)
	if !ok {
		t.Fatalf("expected progress box, got %T", a.top())
	}
	// B sends it to the background: the box closes, the transfer goes on.
	key(a, "B")
	if len(a.dialogs) != 0 || !a.transfers.active() {
		t.Fatal("B should close the box and keep the transfer running")
	}
	if !strings.Contains(a.transferStatus(), "Background: Copying huge.bin") {
		t.Fatalf("status = %q", a.transferStatus())
	}
	// Ctrl+B lists it; Delete cancels it; the partial target disappears.
	key(a, "Ctrl+B")
	if l, ok := a.top().(*ListDialog); !ok || l.Title != "Transfers" {
		t.Fatalf("expected the transfers list, got %T", a.top())
	}
	key(a, "Delete")
	waitTransfers(t, a)
	if !strings.Contains(a.status, "cancelled") {
		t.Fatalf("status = %q", a.status)
	}
	if _, err := os.Stat(filepath.Join(other, "huge.bin")); !os.IsNotExist(err) {
		t.Fatal("cancelled copy should remove the partial file")
	}
	_ = d
}

func TestTransferBoxRendering(t *testing.T) {
	a, _ := testApp(t)
	tr := &transfer{jobs: []transferJob{{src: "a", dst: "b", name: "a"}}}
	tr.start = time.Now().Add(-12 * time.Second)
	tr.itemStart = time.Now().Add(-10 * time.Second)
	tr.item, tr.dst = `A:\src\x.bin`, `D:\dest\x.bin`
	tr.done, tr.total, tr.bytesTotal = 50<<20, 200<<20, 200<<20
	a.push(&TransferDialog{t: tr})
	a.draw()
	txt := screenText(a)
	for _, want := range []string{" Copy ", "Copying the file", "x.bin", "to", "dest", " 25%", " Total ", "Files:", "0 / 1", "Bytes:", "50.0 M / 200 M", "█", "Time: 0:12", "Remaining:", "Speed: 5.0 M/s", "B: background"} {
		if !strings.Contains(txt, want) {
			t.Fatalf("missing %q in progress box:\n%s", want, txt)
		}
	}
	// Esc on the box cancels the transfer.
	a.screen.(tcell.SimulationScreen).InjectKey(tcell.KeyEscape, 0, 0)
	pump(a)
	if !tr.cancel.Load() || len(a.dialogs) != 0 {
		t.Fatal("Esc should cancel and close")
	}
}
