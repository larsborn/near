package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestNameBlobBasics(t *testing.T) {
	b := blobOf("alpha", "beta.txt", "gamma")
	if b.count() != 3 {
		t.Fatalf("count = %d, want 3", b.count())
	}
	for i, want := range []string{"alpha", "beta.txt", "gamma"} {
		if got := string(b.at(i)); got != want {
			t.Errorf("at(%d) = %q, want %q", i, got, want)
		}
	}
	if b.at(3) != nil || b.at(-1) != nil {
		t.Error("at() out of range should be nil")
	}
	if b.indexOf("beta.txt") != 1 || b.indexOf("nope") != -1 {
		t.Errorf("indexOf: %d, %d", b.indexOf("beta.txt"), b.indexOf("nope"))
	}
	if got := b.strings(); !reflect.DeepEqual(got, []string{"alpha", "beta.txt", "gamma"}) {
		t.Errorf("strings() = %q", got)
	}
}

// Removing a name must swap the last one into the hole, exactly as the
// []string version did, so that the ordinals the flat view recorded for
// the other names still point at the same names.
func TestNameBlobRemoveKeepsOrdinals(t *testing.T) {
	b := blobOf("a", "bb", "ccc", "dddd")
	out, movedTo, moved := b.removeAt(1)
	if movedTo != 1 || string(moved) != "dddd" {
		t.Fatalf("removeAt(1): movedTo=%d moved=%q", movedTo, moved)
	}
	if got := out.strings(); !reflect.DeepEqual(got, []string{"a", "dddd", "ccc"}) {
		t.Fatalf("after removeAt(1): %q", got)
	}
	// "ccc" kept ordinal 2, which is the whole point.
	if string(out.at(2)) != "ccc" {
		t.Errorf("at(2) = %q, want ccc", out.at(2))
	}

	// Removing the last one moves nothing.
	out2, movedTo2, moved2 := out.removeAt(2)
	if movedTo2 != -1 || moved2 != nil {
		t.Errorf("removing the last: movedTo=%d moved=%q", movedTo2, moved2)
	}
	if got := out2.strings(); !reflect.DeepEqual(got, []string{"a", "dddd"}) {
		t.Errorf("after removing the last: %q", got)
	}

	// Out of range leaves it alone.
	if out3, mt, m := out2.removeAt(7); mt != -1 || m != nil || out3.count() != 2 {
		t.Errorf("removeAt(7) changed something: count=%d", out3.count())
	}
	// Down to empty.
	one, _, _ := out2.removeAt(0)
	if got := one.strings(); !reflect.DeepEqual(got, []string{"dddd"}) {
		t.Fatalf("after removeAt(0): %q", got)
	}
	empty, _, _ := one.removeAt(0)
	if empty.count() != 0 {
		t.Errorf("count = %d, want 0", empty.count())
	}
}

// A length prefix rather than a separator means a name may contain
// anything, which on Linux includes newlines.
func TestNameBlobHandlesAwkwardNames(t *testing.T) {
	names := []string{"with\nnewline", "with spaces", "", strings.Repeat("x", 300), "ünïcødé"}
	b := blobOf(names...)
	if got := b.strings(); !reflect.DeepEqual(got, names) {
		t.Fatalf("round trip: %q", got)
	}
	if b.indexOf("with\nnewline") != 0 || b.indexOf(strings.Repeat("x", 300)) != 3 {
		t.Error("indexOf failed on an awkward name")
	}
	// A 300-byte name needs a two-byte length prefix; removal must still
	// find the right boundaries.
	out, _, moved := b.removeAt(0)
	if string(moved) != "ünïcødé" {
		t.Errorf("moved = %q", moved)
	}
	if got := out.strings(); !reflect.DeepEqual(got,
		[]string{"ünïcødé", "with spaces", "", strings.Repeat("x", 300)}) {
		t.Errorf("after removal: %q", got)
	}
}

func TestNameBlobAddAndCompact(t *testing.T) {
	var b nameBlob
	if b.count() != 0 || b.at(0) != nil {
		t.Fatal("the empty blob should hold nothing")
	}
	for i := 0; i < 50; i++ {
		b = b.add("name")
	}
	if b.count() != 50 {
		t.Fatalf("count = %d, want 50", b.count())
	}
	c := b.compact()
	if cap(c) != len(c) {
		t.Errorf("compact left %d bytes spare", cap(c)-len(c))
	}
	if c.count() != 50 || string(c.at(49)) != "name" {
		t.Error("compact changed the contents")
	}
}
