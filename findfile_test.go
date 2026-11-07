package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Alt+F7 opens FAR's Find file dialog; its results replace the panel's
// listing, named by their path below the directory, and Esc brings the
// directory back.
func TestFindFileDialogAndResults(t *testing.T) {
	a, data := testApp(t)
	key(a, "Alt+F7")
	d, ok := a.top().(*FindDialog)
	if !ok {
		t.Fatalf("expected the Find file dialog, got %T", a.top())
	}
	txt := screenText(a)
	for _, want := range []string{"Find file", "A file mask or several file masks:", "*.*", "Containing", "(•) Text", "( ) Hex",
		"[ ] Case sensitive", "[ ] Whole words", "[ ] Not containing", "[x] Search for folders", "[x] Search in symbolic links",
		"Select search area:", "From the current folder", "{ Find }", "[ Cancel ]"} {
		if !strings.Contains(txt, want) {
			t.Fatalf("dialog is missing %q:\n%s", want, txt)
		}
	}
	// Alt+T and Alt+H pick the kind of content and then hand over to the
	// field, so the text can be typed straight away.
	key(a, "Alt+H")
	if !d.spec.Hex || d.focus != findFocusText {
		t.Fatalf("Alt+H: hex=%v focus=%d, want hex with the text field focused", d.spec.Hex, d.focus)
	}
	typeText(a, "4d5a")
	if d.text.String() != "4d5a" {
		t.Fatalf("typing after Alt+H went somewhere else: %q", d.text.String())
	}
	key(a, "Alt+T")
	if d.spec.Hex || d.focus != findFocusText {
		t.Fatalf("Alt+T: hex=%v focus=%d, want text with the field focused", d.spec.Hex, d.focus)
	}
	d.text.Set("")
	key(a, "Alt+M") // back to the masks for the rest of the test
	if d.focus != findFocusMasks {
		t.Fatalf("Alt+M should focus the mask field, focus=%d", d.focus)
	}
	d.masks.Anchor, d.masks.Cur = 0, len(d.masks.Text)

	// The mask is preselected, so typing replaces it.
	typeText(a, "*.md;*.jpg")
	key(a, "Enter")
	tab := a.tab()
	if tab.Find == nil {
		t.Fatal("the panel should show search results")
	}
	var names []string
	for _, e := range tab.Entries {
		names = append(names, e.Name)
	}
	want := []string{"..", filepath.Join("gamma", "inner", "deep.md"), "photo.jpg"}
	if strings.Join(names, "|") != strings.Join(want, "|") {
		t.Fatalf("results = %v, want %v", names, want)
	}
	if s := screenText(a); !strings.Contains(s, "Find *.md;*.jpg in ") {
		t.Fatalf("path row should describe the search:\n%s", s)
	}
	// Entries are usable: F4 on a result edits the right file.
	tab.SetCursorOn(filepath.Join("gamma", "inner", "deep.md"))
	key(a, "F4")
	if e, ok := a.top().(*Editor); !ok || e.path != filepath.Join(data, "gamma", "inner", "deep.md") {
		t.Fatalf("F4 on a result: %T", a.top())
	}
	key(a, "Esc")
	// Esc (up_dir) leaves the results.
	key(a, "Esc")
	if tab.Find != nil || tab.Path != data {
		t.Fatalf("Esc should restore the directory: find=%v path=%s", tab.Find != nil, tab.Path)
	}
	_ = d
}

// What a Find file content search looked for is carried into the viewer:
// F3 on a result opens on the first hit and marks it.
func TestFindFileContentCarriesIntoTheViewer(t *testing.T) {
	a, data := testApp(t)
	a.screen.(interface{ SetSize(int, int) }).SetSize(70, 14)
	doc := "first line\nsecond line\nthird line\nnginx is here\n"
	os.WriteFile(filepath.Join(data, "conf.txt"), []byte(doc), 0o644)
	a.reloadAll()

	key(a, "Alt+F7")
	typeText(a, "*.txt") // the mask field is preselected
	key(a, "Alt+T")      // Text, with the content field focused
	typeText(a, "nginx")
	key(a, "Enter")
	if a.tab().Find == nil || len(a.tab().Entries) < 2 {
		t.Fatalf("the search should have found conf.txt: %v", names(a.tab()))
	}
	a.tab().SetCursorOn("conf.txt")
	key(a, "F3")
	v, ok := a.top().(*Viewer)
	if !ok {
		t.Fatalf("F3 should open the viewer, got %T", a.top())
	}
	if v.query != "nginx" {
		t.Fatalf("the viewer should know what was searched for, query=%q", v.query)
	}
	if v.cursor != 3 {
		t.Fatalf("it should open on the hit, cursor is on line %d", v.cursor)
	}
	a.draw()
	if s := screenText(a); !strings.Contains(s, `"nginx" on line 4`) {
		t.Fatalf("the viewer should report the hit:\n%s", s)
	}
}

// Content search with the whole-word and case options, and "Current
// folder only" not descending.
func TestFindFileContent(t *testing.T) {
	a, data := testApp(t)
	os.WriteFile(filepath.Join(data, "alpha", "note.txt"), []byte("Hello WORLD, hello world\n"), 0o644)
	os.WriteFile(filepath.Join(data, "alpha", "other.txt"), []byte("helloworld\n"), 0o644)
	a.reloadAll()

	find := func(spec findSpec) []string {
		a.runFind(a.tab(), spec)
		var names []string
		for _, e := range a.tab().Entries {
			if !e.IsUp {
				names = append(names, e.Name)
			}
		}
		a.leaveFind(a.tab())
		return names
	}
	got := find(findSpec{Masks: "*.txt", Text: "hello world", Folders: true})
	if strings.Join(got, "|") != filepath.Join("alpha", "note.txt") {
		t.Fatalf("plain content search: %v", got)
	}
	// "world" as a whole word: note.txt has it, other.txt only "helloworld".
	got = find(findSpec{Masks: "*.txt", Text: "world", WholeWords: true, Folders: true})
	if strings.Join(got, "|") != filepath.Join("alpha", "note.txt") {
		t.Fatalf("whole words: %v", got)
	}
	got = find(findSpec{Masks: "*.txt", Text: "WORLD", CaseSens: true, Folders: true})
	if strings.Join(got, "|") != filepath.Join("alpha", "note.txt") {
		t.Fatalf("case sensitive: %v", got)
	}
	got = find(findSpec{Masks: "*.txt", Text: "world", NotContain: true, Folders: true})
	if strings.Join(got, "|") != "file1.txt|file2.txt|file10.txt" {
		t.Fatalf("not containing: %v", got)
	}
	got = find(findSpec{Masks: "*", Hex: true, Text: "68656c6c6f776f726c64", Folders: true}) // "helloworld"
	if strings.Join(got, "|") != filepath.Join("alpha", "other.txt") {
		t.Fatalf("hex: %v", got)
	}
	got = find(findSpec{Masks: "*.txt", Area: 1, Folders: true}) // current folder only
	if strings.Join(got, "|") != "file1.txt|file2.txt|file10.txt" {
		t.Fatalf("current folder only: %v", got)
	}
	got = find(findSpec{Masks: "inner", Folders: true})
	if strings.Join(got, "|") != filepath.Join("gamma", "inner") {
		t.Fatalf("folders as results: %v", got)
	}
	got = find(findSpec{Masks: "inner", Folders: false})
	if len(got) != 0 {
		t.Fatalf("folders excluded: %v", got)
	}
}
