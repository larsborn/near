//go:build !windows

package main

import (
	"bufio"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// desktopMenu is the context menu outside Windows: "Open with" entries from
// the desktop's application registrations (Linux, BSD) or Finder actions
// (macOS). near's own actions are appended by the caller.
type desktopMenu struct {
	root []menuEntry
}

func openShellMenu(dir string, names []string) (*desktopMenu, error) {
	m := &desktopMenu{}
	paths := make([]string, len(names))
	for i, n := range names {
		paths[i] = filepath.Join(dir, n)
	}
	first := paths[0]
	detach := func(name string, args ...string) func() error {
		return func() error {
			cmd := exec.Command(name, args...)
			if err := cmd.Start(); err != nil {
				return err
			}
			go func() { _ = cmd.Wait() }()
			return nil
		}
	}
	if runtime.GOOS == "darwin" {
		m.root = []menuEntry{
			{text: "Open", hotkey: 'O', run: detach("open", paths...)},
			{text: "Reveal in Finder", hotkey: 'R', run: detach("open", "-R", first)},
			{text: "Quick Look", hotkey: 'Q', run: detach("qlmanage", append([]string{"-p"}, paths...)...)},
			{text: "Get Info in Finder", hotkey: 'I', run: func() error {
				script := `tell application "Finder" to open information window of (POSIX file "` + strings.ReplaceAll(first, `"`, `\"`) + `" as alias)`
				return detach("osascript", "-e", script)()
			}},
		}
		return m, nil
	}
	m.root = []menuEntry{{text: "Open", hotkey: 'O', run: detach("xdg-open", first)}}
	for _, app := range desktopAppsFor(first) {
		app := app
		m.root = append(m.root, menuEntry{text: "Open with " + app.name, run: func() error { return app.launch(paths) }})
	}
	return m, nil
}

func (m *desktopMenu) Entries() []menuEntry           { return m.root }
func (m *desktopMenu) Expand(e menuEntry) []menuEntry { return nil }
func (m *desktopMenu) Invoke(e menuEntry) error {
	if e.run != nil {
		return e.run()
	}
	return nil
}
func (m *desktopMenu) Close() {}

// desktopApp is an application from a .desktop file.
type desktopApp struct {
	name string
	exec string
}

// launch runs the Exec line with the files substituted for %f %F %u %U.
func (d desktopApp) launch(paths []string) error {
	fields := strings.Fields(d.exec)
	var args []string
	placed := false
	for _, f := range fields {
		switch f {
		case "%f", "%u":
			args = append(args, paths[0])
			placed = true
		case "%F", "%U":
			args = append(args, paths...)
			placed = true
		default:
			if strings.HasPrefix(f, "%") && len(f) == 2 {
				continue // %i %c %k and friends
			}
			args = append(args, strings.Trim(f, `"`))
		}
	}
	if !placed {
		args = append(args, paths...)
	}
	if len(args) == 0 {
		return nil
	}
	cmd := exec.Command(args[0], args[1:]...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// desktopAppsFor lists the applications registered for the file's MIME type.
func desktopAppsFor(path string) []desktopApp {
	mt := mimeTypeOf(path)
	if mt == "" {
		return nil
	}
	var dirs []string
	if h, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(h, ".local", "share", "applications"))
	}
	for _, d := range strings.Split(os.Getenv("XDG_DATA_DIRS"), ":") {
		if d != "" {
			dirs = append(dirs, filepath.Join(d, "applications"))
		}
	}
	dirs = append(dirs, "/usr/local/share/applications", "/usr/share/applications")
	seen := map[string]bool{}
	var apps []desktopApp
	for _, d := range dirs {
		files, err := filepath.Glob(filepath.Join(d, "*.desktop"))
		if err != nil {
			continue
		}
		for _, f := range files {
			if seen[filepath.Base(f)] {
				continue
			}
			seen[filepath.Base(f)] = true
			if app, ok := readDesktopFile(f, mt); ok {
				apps = append(apps, app)
			}
		}
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].name < apps[j].name })
	return apps
}

func mimeTypeOf(path string) string {
	if out, err := exec.Command("xdg-mime", "query", "filetype", path).Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s
		}
	}
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		return "inode/directory"
	}
	return strings.Split(mime.TypeByExtension(filepath.Ext(path)), ";")[0]
}

// readDesktopFile returns the app if its MimeType line lists mt.
func readDesktopFile(path, mt string) (desktopApp, bool) {
	f, err := os.Open(path)
	if err != nil {
		return desktopApp{}, false
	}
	defer f.Close()
	var app desktopApp
	inEntry, matches, hidden := false, false, false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			inEntry = line == "[Desktop Entry]"
			continue
		}
		if !inEntry {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k {
		case "Name":
			if app.name == "" {
				app.name = v
			}
		case "Exec":
			app.exec = v
		case "NoDisplay", "Hidden":
			hidden = hidden || v == "true"
		case "MimeType":
			for _, t := range strings.Split(v, ";") {
				if t == mt {
					matches = true
				}
			}
		}
	}
	if !matches || hidden || app.name == "" || app.exec == "" {
		return desktopApp{}, false
	}
	return app, true
}
