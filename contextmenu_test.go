package main

import (
	"runtime"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestContextMenuOpensAndNavigates(t *testing.T) {
	a, _ := testApp(t)
	a.tab().SetCursorOn("file1.txt")
	key(a, "Shift+F10")
	d, ok := a.top().(*ListDialog)
	if !ok || !d.Menu {
		t.Fatalf("expected the context menu, got %T", a.top())
	}
	if d.Title != "file1.txt" {
		t.Fatalf("title = %q", d.Title)
	}
	labels := ""
	for _, it := range d.Items {
		labels += it.Label + "|"
	}
	if !strings.Contains(labels, "Open") {
		t.Fatalf("menu should offer Open: %s", labels)
	}
	// near's own actions follow the platform entries.
	for _, want := range []string{"Copy to other panel…", "Rename…", "Delete", "Copy path to clipboard"} {
		if !strings.Contains(labels, want) {
			t.Fatalf("menu lacks %q: %s", want, labels)
		}
	}
	if runtime.GOOS == "windows" {
		// Explorer's menu for a .txt file has more than near's own entries and
		// usually a submenu (Send to); opening and leaving one must work.
		if len(d.Items) < 8 {
			t.Fatalf("expected Explorer's entries, got %d items", len(d.Items))
		}
		for i, it := range d.Items {
			if e, ok := it.Data.(menuEntry); ok && e.hasSub {
				d.Cursor = i
				key(a, "Right")
				sub, ok := a.top().(*ListDialog)
				if !ok || sub.Title != e.text {
					t.Fatalf("submenu %q did not open: %T", e.text, a.top())
				}
				key(a, "Left")
				if back, ok := a.top().(*ListDialog); !ok || back.Title != "file1.txt" {
					t.Fatalf("Left should return to the parent menu, got %T", a.top())
				}
				break
			}
		}
	}
	// Esc closes the menu; a near action from the menu runs.
	key(a, "Esc")
	if len(a.dialogs) != 0 {
		t.Fatal("Esc should close the menu")
	}
	key(a, "Shift+F10")
	d = a.top().(*ListDialog)
	for i, it := range d.Items {
		if it.Label == "Copy path to clipboard" {
			d.Cursor = i
		}
	}
	key(a, "Enter")
	if !strings.HasSuffix(a.clipboard, "file1.txt") {
		t.Fatalf("menu action did not run: clipboard=%q", a.clipboard)
	}
}

// fakeMenu records the order of Invoke and Close, which matters: the shell
// objects must still be alive when a command is invoked.
type fakeMenu struct{ calls []string }

func (f *fakeMenu) Entries() []menuEntry {
	return []menuEntry{{text: "Top", hasSub: true}, {text: "Plain"}}
}
func (f *fakeMenu) Expand(e menuEntry) []menuEntry { return []menuEntry{{text: "Add to zip"}} }
func (f *fakeMenu) Invoke(e menuEntry) error       { f.calls = append(f.calls, "invoke:"+e.text); return nil }
func (f *fakeMenu) Close()                         { f.calls = append(f.calls, "close") }

func TestContextMenuInvokesBeforeClose(t *testing.T) {
	a, _ := testApp(t)
	f := &fakeMenu{}
	a.showMenuLevel(f, "x", f.Entries(), nil)
	key(a, "Right") // into the submenu
	key(a, "Enter") // "Add to zip"
	if strings.Join(f.calls, ",") != "invoke:Add to zip,close" {
		t.Fatalf("call order = %v", f.calls)
	}
	// Esc at the root closes the menu objects exactly once.
	f = &fakeMenu{}
	a.showMenuLevel(f, "x", f.Entries(), nil)
	key(a, "Esc")
	if strings.Join(f.calls, ",") != "close" {
		t.Fatalf("Esc call order = %v", f.calls)
	}
	// Mouse: a click on an entry activates it, a click outside closes.
	f = &fakeMenu{}
	a.showMenuLevel(f, "x", f.Entries(), nil)
	a.draw()
	d := a.top().(*ListDialog)
	a.handleEvent(tcell.NewEventMouse(d.box.x+2, d.box.y+2, tcell.ButtonPrimary, 0)) // second entry: Plain
	a.handleEvent(tcell.NewEventMouse(d.box.x+2, d.box.y+2, tcell.ButtonNone, 0))
	if strings.Join(f.calls, ",") != "invoke:Plain,close" {
		t.Fatalf("mouse call order = %v", f.calls)
	}
	f = &fakeMenu{}
	a.showMenuLevel(f, "x", f.Entries(), nil)
	a.draw()
	a.handleEvent(tcell.NewEventMouse(0, 0, tcell.ButtonPrimary, 0))
	a.handleEvent(tcell.NewEventMouse(0, 0, tcell.ButtonNone, 0))
	if len(a.dialogs) != 0 || strings.Join(f.calls, ",") != "close" {
		t.Fatalf("click outside: dialogs=%d calls=%v", len(a.dialogs), f.calls)
	}
}

func TestMenuLabelMnemonics(t *testing.T) {
	cases := map[string][2]string{
		"&Open":                 {"Open", "O"},
		"Open &with…":           {"Open with…", "w"},
		"Cut\tCtrl+X":           {"Cut", ""},
		"Save && Exit":          {"Save & Exit", ""},
		"7-&Zip":                {"7-Zip", "Z"},
		"Run as &administrator": {"Run as administrator", "a"},
	}
	for in, want := range cases {
		text, hot := menuLabel(in)
		if text != want[0] || (want[1] == "" && hot != 0) || (want[1] != "" && string(hot) != want[1]) {
			t.Errorf("menuLabel(%q) = %q %q, want %q %q", in, text, string(hot), want[0], want[1])
		}
	}
}

func TestMouseClicksAndContextMenu(t *testing.T) {
	a, _ := testApp(t)
	a.draw()
	r := a.panelRect[1]
	if r.w == 0 {
		t.Fatal("panel rect not recorded")
	}
	// Left click on the second entry of the right panel: panel and cursor follow.
	click := func(x, y int, b tcell.ButtonMask) {
		a.handleEvent(tcell.NewEventMouse(x, y, b, 0))
		a.handleEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, 0))
		a.draw()
	}
	click(r.x+3, r.y+3+1, tcell.ButtonPrimary)
	if a.active != 1 || a.tab().Cursor != 1 {
		t.Fatalf("left click: active=%d cursor=%d", a.active, a.tab().Cursor)
	}
	// Right click on the left panel's photo.jpg opens its context menu.
	l := a.panelRect[0]
	idx := a.panels[0].Tab().indexOf("photo.jpg")
	click(l.x+3, l.y+3+idx, tcell.ButtonSecondary)
	d, ok := a.top().(*ListDialog)
	if !ok || !d.Menu || d.Title != "photo.jpg" {
		t.Fatalf("right click should open the context menu for photo.jpg, got %T", a.top())
	}
	key(a, "Esc")
	// Double click enters a directory.
	gi := a.panels[0].Tab().indexOf("gamma")
	click(l.x+3, l.y+3+gi, tcell.ButtonPrimary)
	click(l.x+3, l.y+3+gi, tcell.ButtonPrimary)
	if !strings.HasSuffix(a.tab().Path, "gamma") {
		t.Fatalf("double click should enter gamma: %s", a.tab().Path)
	}
	// Wheel scrolls the cursor.
	key(a, "Esc")
	a.tab().Cursor = 0
	a.handleEvent(tcell.NewEventMouse(l.x+3, l.y+4, tcell.WheelDown, 0))
	if a.tab().Cursor != 3 {
		t.Fatalf("wheel: cursor=%d", a.tab().Cursor)
	}
}
