package main

import "encoding/binary"

// nameBlob holds one directory's entry names as a single run of bytes, each
// name preceded by its length as a varint.
//
// The obvious representation is []string, and that is what the index used
// at first. It costs a 16-byte header per name plus whatever append left
// spare in the slice: on a whole-machine index of 7.8 million names in
// 846,000 directories that was about 160 MB of pure overhead, on top of
// the 179 MB the names themselves need. A blob pays one length byte per
// name instead and nothing per directory beyond the map value.
//
// Names are addressed by ordinal, the position the flat search view
// records in nameIx, and the ordinals behave exactly as slice indices did:
// removeAt swaps the last name into the hole so the others keep their
// places. A length prefix rather than a separator means no byte is
// forbidden in a name — on Linux a file name may contain anything but '/'
// and NUL, newlines included.
type nameBlob []byte

// span returns the byte range of ordinal i, length prefix included, or
// (len, len) when there is no such ordinal.
func (b nameBlob) span(i int) (int, int) {
	p := 0
	for k := 0; p < len(b); k++ {
		n, w := binary.Uvarint(b[p:])
		if w <= 0 {
			break
		}
		end := p + w + int(n)
		if end > len(b) {
			break
		}
		if k == i {
			return p, end
		}
		p = end
	}
	return len(b), len(b)
}

// count is how many names the blob holds.
func (b nameBlob) count() int {
	p, k := 0, 0
	for p < len(b) {
		n, w := binary.Uvarint(b[p:])
		if w <= 0 {
			break
		}
		end := p + w + int(n)
		if end > len(b) {
			break
		}
		p, k = end, k+1
	}
	return k
}

// at returns the name at ordinal i, or nil. The result aliases the blob.
func (b nameBlob) at(i int) []byte {
	s, e := b.span(i)
	if s == e {
		return nil
	}
	_, w := binary.Uvarint(b[s:])
	return b[s+w : e]
}

// each calls fn for every name until it returns false. The name aliases
// the blob and must not be kept.
func (b nameBlob) each(fn func(i int, name []byte) bool) {
	p, k := 0, 0
	for p < len(b) {
		n, w := binary.Uvarint(b[p:])
		if w <= 0 {
			return
		}
		end := p + w + int(n)
		if end > len(b) {
			return
		}
		if !fn(k, b[p+w:end]) {
			return
		}
		p, k = end, k+1
	}
}

// indexOf is the ordinal of name, or -1.
func (b nameBlob) indexOf(name string) int {
	found := -1
	b.each(func(i int, n []byte) bool {
		if string(n) == name { // compares without allocating
			found = i
			return false
		}
		return true
	})
	return found
}

// add appends a name. It does not check for duplicates.
func (b nameBlob) add(name string) nameBlob {
	var hdr [binary.MaxVarintLen64]byte
	w := binary.PutUvarint(hdr[:], uint64(len(name)))
	b = append(b, hdr[:w]...)
	return append(b, name...)
}

// removeAt drops ordinal i, moving the last name into its place so that
// every other ordinal still points at the same name. The moved name and
// its new ordinal are returned so the flat view can relearn them; moved is
// nil when the last name was the one removed.
func (b nameBlob) removeAt(i int) (out nameBlob, movedTo int, moved []byte) {
	n := b.count()
	if i < 0 || i >= n {
		return b, -1, nil
	}
	si, ei := b.span(i)
	if i == n-1 {
		return b[:si], -1, nil
	}
	sl, el := b.span(n - 1)
	out = make(nameBlob, 0, len(b)-(ei-si))
	out = append(out, b[:si]...)
	out = append(out, b[sl:el]...) // the last entry takes i's place
	out = append(out, b[ei:sl]...)
	return out, i, out.at(i)
}

// compact returns the blob in a buffer of exactly its length, giving back
// what append left spare. It is a no-op when there is nothing spare.
func (b nameBlob) compact() nameBlob {
	if cap(b) == len(b) || b == nil {
		return b
	}
	out := make(nameBlob, len(b))
	copy(out, b)
	return out
}

// blobOf builds a blob from names, for tests and conversions.
func blobOf(names ...string) nameBlob {
	n := 0
	for _, s := range names {
		n += len(s) + binary.MaxVarintLen64
	}
	b := make(nameBlob, 0, n)
	for _, s := range names {
		b = b.add(s)
	}
	return b.compact()
}

// strings returns the names as a slice, for the few callers that want one.
func (b nameBlob) strings() []string {
	out := make([]string, 0, b.count())
	b.each(func(_ int, n []byte) bool {
		out = append(out, string(n))
		return true
	})
	return out
}
