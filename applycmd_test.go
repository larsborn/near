package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExpandApply(t *testing.T) {
	e := Entry{Name: "report.final.pdf"}
	dir := `C:\docs\in`
	sep := `\`
	if runtime.GOOS != "windows" {
		dir, sep = "/docs/in", "/"
	}
	got := expandApply("conv !.! -> !.txt ext=!~ dir=!\\ nodir=!/ bang=!! list=!@! all=!&", dir, e, "LIST", "A B")
	for _, want := range []string{
		"conv " + shQuote("report.final.pdf"),
		"-> " + shQuote("report.final") + ".txt",
		"ext=pdf",
		"dir=" + shQuote(dir+sep),
		"nodir=" + shQuote(dir),
		"bang=!",
		"list=" + shQuote("LIST"),
		"all=A B",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expansion %q lacks %q", got, want)
		}
	}
	if runtime.GOOS == "windows" {
		if got := expandApply("!:", dir, e, "", ""); got != "C:" {
			t.Fatalf("drive = %q", got)
		}
	}
	// A directory has no extension: ! is the whole name.
	if got := expandApply("!", dir, Entry{Name: "my.dir", IsDir: true}, "", ""); got != shQuote("my.dir") {
		t.Fatalf("dir name = %q", got)
	}
}

func TestApplyCommandRunsPerFileConsecutively(t *testing.T) {
	a, _ := testApp(t)
	a.tab().Selected["file1.txt"] = true
	a.tab().Selected["file2.txt"] = true
	a.tab().Selected["photo.jpg"] = true
	key(a, "Ctrl+G")
	if _, ok := a.top().(*ApplyDialog); !ok {
		t.Fatalf("expected the apply dialog, got %T", a.top())
	}
	if !strings.Contains(screenText(a), "3 selected entries") {
		t.Fatal("dialog should name the selection")
	}
	typeText(a, "echo applying !.! base=! ext=!~")
	key(a, "Enter")
	if a.running == nil || len(a.cmdQueue) != 2 {
		t.Fatalf("first command should run with two queued: running=%v queued=%d", a.running != nil, len(a.cmdQueue))
	}
	if !a.hideAll {
		t.Fatal("the console view should show while the commands run")
	}
	waitCommand(t, a)
	// waitCommand returns once no command runs; the queue drains one by one.
	for len(a.cmdQueue) > 0 || a.running != nil {
		waitCommand(t, a)
	}
	pump(a)
	// cmd.exe echoes the shell quoting back; compare without quotes.
	out := strings.NewReplacer(`"`, "", `'`, "").Replace(a.console.Text())
	for _, want := range []string{"applying file1.txt base=file1 ext=txt", "applying file2.txt base=file2 ext=txt", "applying photo.jpg base=photo ext=jpg"} {
		if !strings.Contains(out, want) {
			t.Fatalf("console lacks %q:\n%s", want, out)
		}
	}
	// Order is preserved and the panels are back afterwards.
	if strings.Index(out, "file1.txt") > strings.Index(out, "file2.txt") || strings.Index(out, "file2.txt") > strings.Index(out, "photo.jpg") {
		t.Fatal("commands ran out of order")
	}
	if a.hideAll {
		t.Fatal("panels should return after the last command")
	}
	if n, _ := a.tab().SelectedStats(); n != 0 {
		t.Fatal("selection should be cleared")
	}
	// The template is remembered for next time; the prompt form asks first.
	a.tab().SetCursorOn("photo.jpg")
	key(a, "Ctrl+G")
	if a.top().(*ApplyDialog).edit.String() != "echo applying !.! base=! ext=!~" {
		t.Fatal("last template not remembered")
	}
	key(a, "Esc")
	a.tab().SetCursorOn("photo.jpg")
	key(a, "Ctrl+G")
	a.top().(*ApplyDialog).edit.Set("echo !?Suffix?bak! !.!")
	key(a, "Enter")
	in, ok := a.top().(*InputDialog)
	if !ok || in.Edit.String() != "bak" {
		t.Fatalf("expected the prompt with its default, got %T", a.top())
	}
	in.Edit.Set("old")
	key(a, "Enter")
	waitCommand(t, a)
	if !strings.Contains(strings.ReplaceAll(a.console.Text(), `"`, ""), "old photo.jpg") {
		t.Fatalf("prompted value not substituted:\n%s", a.console.Text())
	}
}

func TestCdExpandsEnvironmentVariables(t *testing.T) {
	a, data := testApp(t)
	t.Setenv("NEAR_TEST_DIR", data)
	for _, form := range []string{"cd %NEAR_TEST_DIR%/gamma", "cd $NEAR_TEST_DIR/gamma", "cd ${NEAR_TEST_DIR}/gamma"} {
		a.navigate(a.tab(), data, "")
		typeText(a, form)
		key(a, "Enter")
		if a.tab().Path != filepath.Join(data, "gamma") {
			t.Fatalf("%s: path = %s", form, a.tab().Path)
		}
	}
	// Unknown variables are left alone rather than blanked.
	if expandPath("%NEAR_NO_SUCH_VAR%") != "%NEAR_NO_SUCH_VAR%" {
		t.Fatal("unknown %VAR% should stay as typed")
	}
}

func TestGotoPathKeyMoved(t *testing.T) {
	a, _ := testApp(t)
	key(a, "Alt+F10")
	if d, ok := a.top().(*InputDialog); !ok || d.Title != "Go to directory" {
		t.Fatalf("Alt+F10 should open go-to-directory, got %T", a.top())
	}
	key(a, "Esc")
}
