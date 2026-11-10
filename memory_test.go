package main

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestGCPercentForHeapSize(t *testing.T) {
	cases := []struct {
		live int64
		want int
	}{
		{0, 100},                 // nothing known yet
		{64 << 20, 100},          // small heap: the default is already tight
		{384 << 20, 100},         // exactly the headroom
		{768 << 20, 50},          // a big index: half of it, not all of it
		{960 << 20, 40},          // the developer's 8-million-name index
		{64 << 30, gcPercentMin}, // absurd heap: do not thrash
	}
	for _, c := range cases {
		if got := gcPercentFor(c.live); got != c.want {
			t.Errorf("gcPercentFor(%d MB) = %d, want %d", c.live>>20, got, c.want)
		}
	}
}

// The flat view's buffers are sized from the map before filling them, so
// building one must not make append grow (and copy) them: on a real index
// that costs hundreds of megabytes of transient memory.
func TestFlatIndexAllocatesExactly(t *testing.T) {
	dirs := map[string][]string{}
	for d := 0; d < 40; d++ {
		dir := filepath.Join("/root", fmt.Sprintf("d%02d", d))
		for i := 0; i < 200; i++ {
			dirs[dir] = append(dirs[dir], fmt.Sprintf("Datei%d-ÄÖÜ.txt", i))
		}
	}
	f := buildFlatIndex(dirs)
	if cap(f.off) != len(f.off) || cap(f.dirIx) != len(f.dirIx) || cap(f.nameIx) != len(f.nameIx) {
		t.Errorf("index arrays regrew: off %d/%d dirIx %d/%d nameIx %d/%d",
			len(f.off), cap(f.off), len(f.dirIx), cap(f.dirIx), len(f.nameIx), cap(f.nameIx))
	}
	// The margin for non-ASCII lowering is 1/64; anything beyond that means
	// append doubled the buffer.
	if cap(f.names) > len(f.names)+len(f.names)/32+64 {
		t.Errorf("name buffer regrew: len=%d cap=%d", len(f.names), cap(f.names))
	}
}
