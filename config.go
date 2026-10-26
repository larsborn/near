package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

// MenuItem is one line of a custom menu.
type MenuItem struct {
	Title   string `toml:"title"`
	Command string `toml:"command,omitempty"`
	Hotkey  string `toml:"hotkey,omitempty"`
	Submenu string `toml:"submenu,omitempty"`
	Detach  bool   `toml:"detach,omitempty"`
	NoWait  bool   `toml:"nowait,omitempty"`
	Confirm bool   `toml:"confirm,omitempty"`
	Cwd     string `toml:"cwd,omitempty"`
}

// Menu is a custom menu, optionally bound to a key.
type Menu struct {
	Name  string     `toml:"name"`
	Key   string     `toml:"key,omitempty"`
	Items []MenuItem `toml:"items"`
}

// Bookmark is a named directory shown in the locations dialog.
type Bookmark struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

// Config is the on-disk configuration.
type Config struct {
	Editor              string              `toml:"editor"`
	Shell               string              `toml:"shell"`
	ShellRC             string              `toml:"shell_rc"`
	Theme               string              `toml:"theme"`
	ShowHidden          bool                `toml:"show_hidden"`
	Mouse               bool                `toml:"mouse"`
	ConfirmDelete       bool                `toml:"confirm_delete"`
	DeleteToTrash       bool                `toml:"delete_to_trash"`
	CommandWait         bool                `toml:"command_wait"`
	InteractiveCommands []string            `toml:"interactive_commands"`
	ConsoleLines        int                 `toml:"console_lines"`
	ConsoleRows         int                 `toml:"console_rows"`
	DateFormat          string              `toml:"date_format"`
	TimeFormat          string              `toml:"time_format"`
	InternalEditor      bool                `toml:"internal_editor"`
	EditorTabSize       int                 `toml:"editor_tab_size"`
	NotesFile           string              `toml:"notes_file"`
	SearchRoots         []string            `toml:"search_roots"`
	SearchExclude       []string            `toml:"search_exclude"`
	SearchMaxResults    int                 `toml:"search_max_results"`
	SearchIndexOnStart  bool                `toml:"search_index_on_start"`
	SearchWatch         bool                `toml:"search_watch"`
	SearchWatchMaxDirs  int                 `toml:"search_watch_max_dirs"`
	Keys                map[string][]string `toml:"keys"`
	Menus               []Menu              `toml:"menus"`
	Bookmarks           []Bookmark          `toml:"bookmarks"`

	dir  string // directory holding config, state and index cache
	path string
}

func configDir(override string) string {
	if override != "" {
		return filepath.Dir(override)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "near")
}

// loadConfig reads the config file, creating it with defaults on first run.
// Values in the file override the defaults; unknown actions in [keys] are
// kept so that menus can bind to them.
func loadConfig(path string) (*Config, error) {
	dir := configDir(path)
	if path == "" {
		path = filepath.Join(dir, "config.toml")
	}
	cfg := &Config{}
	if _, err := toml.Decode(defaultConfigText(), cfg); err != nil {
		return nil, fmt.Errorf("internal default config: %w", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(path, []byte(defaultConfigText()), 0o644)
	} else if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	// Menus edited inside NEAR live in menus.toml and replace the ones here.
	if menus, err := loadMenusFile(filepath.Join(dir, menusFileName)); err == nil {
		cfg.Menus = menus
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("%s: %w", filepath.Join(dir, menusFileName), err)
	}
	if cfg.SearchMaxResults <= 0 {
		cfg.SearchMaxResults = 500
	}
	if cfg.DateFormat == "" {
		cfg.DateFormat = "2006-01-02"
	}
	if cfg.TimeFormat == "" {
		cfg.TimeFormat = "15:04"
	}
	if cfg.EditorTabSize <= 0 {
		cfg.EditorTabSize = 4
	}
	if cfg.NotesFile == "" {
		cfg.NotesFile = ".near-notes"
	}
	cfg.dir = dir
	cfg.path = path
	return cfg, nil
}

func (c *Config) menu(name string) *Menu {
	for i := range c.Menus {
		if strings.EqualFold(c.Menus[i].Name, name) {
			return &c.Menus[i]
		}
	}
	return nil
}

// expandPath expands a leading ~ and environment variables in both the
// Unix ($VAR, ${VAR}) and the Windows (%VAR%) notation.
func expandPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		home, _ := os.UserHomeDir()
		p = home + p[1:]
	}
	p = percentEnv.ReplaceAllStringFunc(p, func(m string) string {
		if v, ok := os.LookupEnv(m[1 : len(m)-1]); ok {
			return v
		}
		return m
	})
	p = os.ExpandEnv(p)
	return filepath.Clean(p)
}

var percentEnv = regexp.MustCompile(`%[^%\s]+%`)

func defaultConfigText() string { return defaultConfigTextFor(runtime.GOOS) }

func defaultConfigTextFor(goos string) string {
	var quick string
	if goos == "windows" {
		quick = `[[menus]]
name = "Quick launch"
key = "F2"

  [[menus.items]]
  title = "Command prompt here"
  hotkey = "c"
  command = "cmd"
  nowait = true

  [[menus.items]]
  title = "PowerShell here"
  hotkey = "p"
  command = "powershell"
  nowait = true

  [[menus.items]]
  title = "Explorer here"
  hotkey = "e"
  command = "explorer ."
  detach = true

  [[menus.items]]
  title = "Git status"
  hotkey = "g"
  command = "git status"

  [[menus.items]]
  title = "Tools..."
  hotkey = "t"
  submenu = "Tools"

[[menus]]
name = "Tools"
key = "Shift+F2"

  [[menus.items]]
  title = "Directory size"
  hotkey = "d"
  command = "dir /s {file}"

  [[menus.items]]
  title = "Count lines of file"
  hotkey = "l"
  command = "find /c /v \"\" {file}"

  [[menus.items]]
  title = "Zip selected to other panel"
  hotkey = "z"
  command = "tar -a -c -f {other}\\archive.zip {files}"
  confirm = true
`
	} else {
		quick = `[[menus]]
name = "Quick launch"
key = "F2"

  [[menus.items]]
  title = "Shell here"
  hotkey = "s"
  command = "${SHELL:-sh}"
  nowait = true

  [[menus.items]]
  title = "Git status"
  hotkey = "g"
  command = "git status"

  [[menus.items]]
  title = "Disk usage here"
  hotkey = "d"
  command = "du -sh -- * | sort -h"

  [[menus.items]]
  title = "Open in file browser"
  hotkey = "o"
  command = "xdg-open ."
  detach = true

  [[menus.items]]
  title = "Tools..."
  hotkey = "t"
  submenu = "Tools"

[[menus]]
name = "Tools"
key = "Shift+F2"

  [[menus.items]]
  title = "Count lines of file"
  hotkey = "l"
  command = "wc -l {file}"

  [[menus.items]]
  title = "Tar selected to other panel"
  hotkey = "z"
  command = "tar czvf {other}/archive.tgz {files}"
  confirm = true

  [[menus.items]]
  title = "Make executable"
  hotkey = "x"
  command = "chmod +x {files}"
  nowait = true
`
	}

	return `# NEAR - dual pane terminal file manager.
# This file was generated on first start; every value shown is the default.
#
# Key names: "F5", "Shift+F6", "Ctrl+X", "Alt+X", "Esc", "Enter", "Tab",
# "Shift+Tab", "Space", "Insert", "Delete", "Backspace", "Home", "End",
# "PgUp", "PgDn", "Up", "Down", "Left", "Right", or a plain character
# such as "+" or "a". Terminals report Ctrl+H as Backspace, Ctrl+I as Tab
# and Ctrl+M as Enter, so those three cannot be bound separately.

editor = ""          # external editor (Ctrl+F4): empty = $VISUAL, $EDITOR, then vi / notepad
internal_editor = true   # F4 opens the built-in FAR-like editor; false = external editor
editor_tab_size = 4
shell = ""           # empty: $SHELL or sh (cmd on Windows)
shell_rc = ""        # file sourced before every command, for aliases and functions (POSIX shells only):
                     # commands run non-interactively, so ~/.bashrc is skipped by its own guard; point this
                     # at a file without that guard, e.g. "~/.bash_aliases". Ignored on Windows.
theme = "far"        # "far" (blue, FAR-like) or "plain" (terminal colours)
show_hidden = true   # list dot files and files with the hidden attribute (Ctrl+F10 toggles)
mouse = true         # click to move the cursor, double-click to open, right-click for the context menu, wheel to scroll
confirm_delete = true
delete_to_trash = true   # F8 / Delete move to the Recycle Bin (trash); Shift+F8 / Shift+Delete delete for good
# Commands typed at the command line run with their output captured into
# NEAR's console view, which replaces the panels and stays until Ctrl+O; the
# command line keeps working like a shell and Ctrl+C stops the command.
# Programs that need a real terminal are run outside instead: everything
# listed here, or any command sent with Shift+Enter.
interactive_commands = ["vi", "vim", "nvim", "nano", "emacs", "less", "more", "man", "ssh", "telnet",
  "htop", "top", "python", "python3", "node", "irb", "sh", "bash", "zsh", "fish", "cmd", "powershell", "pwsh"]
console_lines = 5000     # output lines kept in the console view
console_rows = 6         # rows under the panels showing the tail of the last output (0 = none)
command_wait = true      # after a command run outside (menu item, Shift+Enter), wait for Enter before returning
date_format = "2006-01-02"         # Go time layout for the Date column
time_format = "15:04"              # ... and the Time column
notes_file = ".near-notes"         # per-directory hidden notes file (JSON)

# Everything-like search: directories to index and what to skip. "*" stands
# for every fixed drive (network, removable and subst drives are left out);
# any other entry is a directory, e.g. ["~", "D:\\", "/srv/data"]. In search_exclude a
# plain name skips every directory of that name; an entry with a path
# separator skips that exact directory. On start (see search_index_on_start)
# the index catches up from the NTFS change journal of each volume, without
# administrator rights; it is rebuilt from scratch only when a journal cannot
# be used (recreated, wrapped after a long absence, disabled, non-NTFS, or on
# Linux and macOS). Ctrl+R in the search dialog always rebuilds.
search_roots = ["*"]
search_exclude = [".git", "node_modules", "__pycache__", ".cache", "/proc", "/sys", "/dev", "/run"]
search_max_results = 500
search_index_on_start = true
# Keep the index current by watching the roots for changes. Windows watches
# whole trees with one handle per root; Linux and macOS need one watch per
# directory, so the number is capped (raise fs.inotify.max_user_watches on
# Linux to go higher). When the cap is hit, deeper changes are only picked up
# by the next rebuild.
search_watch = true
search_watch_max_dirs = 100000

# Actions and their keys. An action may have several keys. Remove or
# change entries freely; Esc goes to the parent directory by default.
#
# The command line at the bottom always has the focus: printable characters
# go into it, and while it is not empty Enter runs it, Esc clears it, and
# Left/Right/Home/End/Backspace/Delete edit it. Characters bound to an
# action (+ - * Space) only trigger the action while the command line is
# empty. "cd <dir>" changes the active panel instead of running a shell.
#
# Alt+<character> starts a quick search in the active panel with that
# character (keep Alt+letter combinations unbound for this to work).
[keys]
insert_name = ["Ctrl+Enter", "Ctrl+J"]
insert_path = ["Alt+Enter"]
history_prev = ["Ctrl+E"]
history_next = ["Ctrl+X"]
history = ["Alt+F8"]                   # FAR: command history
folder_history = ["Alt+F12"]           # FAR: folders history
file_history = ["Alt+F11"]             # FAR: view and edit history
compare = ["Shift+F3"]                 # compare directories or files
transfers = ["Ctrl+B"]                 # list background transfers (cancel with Delete)
help = ["F1"]
quit = ["F10", "Ctrl+Q"]
switch_panel = ["Tab"]
up = ["Up"]
down = ["Down"]
page_up = ["PgUp"]
page_down = ["PgDn"]
home = ["Home"]
end = ["End"]
enter = ["Enter"]
up_dir = ["Esc", "Backspace"]
select = ["Insert", "Space"]
select_up = ["Shift+Up"]               # FAR: toggle selection and move up
select_down = ["Shift+Down"]           # FAR: toggle selection and move down
select_to_home = ["Shift+Home"]        # FAR: select from the cursor to the first entry
select_to_end = ["Shift+End"]          # ... to the last entry
select_page_up = ["Shift+PgUp"]
select_page_down = ["Shift+PgDn"]
select_pattern = ["+"]
unselect_pattern = ["-"]
invert_selection = ["*"]
select_all = ["Ctrl+A"]
unselect_all = ["Ctrl+D"]
new_tab = ["Ctrl+T"]
close_tab = ["Ctrl+W"]
next_tab = ["Ctrl+PgDn", "Alt+Right"]
prev_tab = ["Ctrl+PgUp", "Alt+Left"]
tab_1 = ["Alt+1"]
tab_2 = ["Alt+2"]
tab_3 = ["Alt+3"]
tab_4 = ["Alt+4"]
tab_5 = ["Alt+5"]
tab_6 = ["Alt+6"]
tab_7 = ["Alt+7"]
tab_8 = ["Alt+8"]
tab_9 = ["Alt+9"]
view = ["F3"]
edit = ["F4"]
edit_new = ["Shift+F4"]                # FAR: edit a new file
edit_external = ["Ctrl+F4"]            # open in the external editor
copy_path = ["Ctrl+Alt+Insert"]        # copy the full path of the current entry to the clipboard
copy = ["F5"]
move = ["F6"]
rename = ["Shift+F6"]
mass_rename = ["Ctrl+F6", "Alt+F6"]
mkdir = ["F7"]
delete = ["F8", "Delete"]           # to the Recycle Bin / trash (see delete_to_trash)
delete_permanent = ["Shift+F8", "Shift+Delete"]
quick_launch = ["F2"]
menus = ["F9"]
notes = ["Ctrl+N"]
find_file = ["Alt+F7"]            # FAR's Find file: masks and content below the current directory, results in the panel
search = ["Ctrl+F"]               # Everything-like search over the whole index
quick_search = ["Ctrl+S"]          # Alt+<character> also starts a quick search with that character
sort = ["Ctrl+F12"]
toggle_hidden = ["Ctrl+F10"]
refresh = ["Ctrl+R"]
swap_panels = ["Ctrl+U"]
sync_other = ["Shift+F12"]
locations = ["Alt+F1", "Alt+F2", "Ctrl+L"]
goto_path = ["Alt+F10"]               # go to a directory by path
context_menu = ["Shift+F10", "Menu"]   # the file's context menu (Explorer's on Windows); also by right-click
apply_command = ["Ctrl+G"]             # FAR: apply a shell command to every selected entry
run_command = []                       # the command line replaces this dialog; bind if wanted
toggle_panels = ["Ctrl+O"]
toggle_inactive_panel = ["Ctrl+P"]     # FAR: hide/show the inactive panel
toggle_left_panel = ["Ctrl+F1"]
toggle_right_panel = ["Ctrl+F2"]
panels_shorter = ["Ctrl+Up"]           # FAR: the bottom edge of both panels moves up
panels_taller = ["Ctrl+Down"]          # ... and down again
panel_shorter = ["Ctrl+Shift+Up"]      # FAR: the same for the current panel only
panel_taller = ["Ctrl+Shift+Down"]
split_left = ["Ctrl+Left"]             # FAR: change the panels' width (command line empty)
split_right = ["Ctrl+Right"]
panels_default_width = []              # FAR: Ctrl+Numpad5, which terminals cannot report; bind as you like
panels_default_height = []             # FAR: Ctrl+Alt+Numpad5

# Bookmarks appear in the locations dialog (Alt+F1).
[[bookmarks]]
name = "Home"
path = "~"

# Custom menus. The first menu is the quick launch menu (F2); F9 lists all
# menus. A menu with "key" is also opened directly by that key.
#
# Item fields:
#   title    text shown in the menu
#   hotkey   single character that activates the item
# Menus can also be edited inside NEAR like FAR's user menu: open one (F2),
# then Ins inserts a command or submenu, Del deletes, F4 edits, Alt+F4 edits
# the file, Ctrl+Up/Ctrl+Down move. Such edits are kept in menus.toml next to
# this file; once it exists the [[menus]] here are ignored. Commands may
# span several lines (run one after another) and use FAR's symbols (!.!
# name, ! name without extension, !~ extension, !\ directory, !& selected
# names, !@! list file, !?Title?Default! asks first); a command naming a
# file runs once per selected entry.
#   command  shell command; placeholders (all quoted for the shell):
#              {file}   current entry name       {path}   its full path
#              {files}  selected names (or current)  {paths} their full paths
#              {dir}    current directory          {other}  other panel's directory
#              {name}   current name without extension  {ext} extension (no dot)
#   submenu  name of another menu to open instead of running a command
#   nowait   hand the terminal to the command (shells, editors, other interactive
#            programs) and come back without a pause when it ends; without it the
#            output is captured into the console view and stays visible under the
#            panels, like a command typed on the command line
#   detach   start in the background without leaving the file manager (GUI apps)
#   confirm  ask before running
#   cwd      working directory (default: current directory)
` + quick
}
