//go:build windows

package main

import "testing"

// Text with non-ASCII characters must survive the system clipboard. Going
// through clip.exe did not: it decodes stdin in the console's OEM code
// page, so "Löschanfrage" came back as "L├╢schanfrage".
func TestClipboardRoundTripKeepsUmlauts(t *testing.T) {
	before, had := readClipboard() // leave the user's clipboard as it was
	defer func() {
		if had {
			writeClipboard(before)
		}
	}()
	for _, want := range []string{
		`C:\Users\born\Downloads\Widerspruch und Löschanfrage_Wallenborn.docx`,
		"Grüße, Straße, Ümläute",
		"日本語とemoji🙂",
		"plain ascii",
	} {
		if !writeClipboard(want) {
			t.Fatalf("writeClipboard(%q) failed", want)
		}
		got, ok := readClipboard()
		if !ok {
			t.Fatalf("readClipboard after writing %q: nothing there", want)
		}
		if got != want {
			t.Fatalf("clipboard round trip:\n got %q\nwant %q", got, want)
		}
	}
}
