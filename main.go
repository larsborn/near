// near is a keyboard-driven dual pane file manager for the terminal.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gdamore/tcell/v2"
)

var version = "dev"

func main() {
	cfgPath := flag.String("config", "", "config file (default: <user config dir>/near/config.toml)")
	reindex := flag.Bool("reindex", false, "rebuild the file name search index at startup")
	showVersion := flag.Bool("version", false, "print version and exit")
	showDefaults := flag.Bool("defaults", false, "print the default config file and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: near [flags] [left-dir [right-dir]]\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *showVersion {
		fmt.Println("near", version)
		return
	}
	if *showDefaults {
		fmt.Print(defaultConfigText())
		return
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "near:", err)
		os.Exit(1)
	}
	if *reindex {
		cfg.SearchIndexOnStart = true
	}
	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintln(os.Stderr, "near:", err)
		os.Exit(1)
	}
	if err := screen.Init(); err != nil {
		fmt.Fprintln(os.Stderr, "near:", err)
		os.Exit(1)
	}
	app := newApp(cfg, screen)
	screen.SetStyle(app.theme.Panel)
	if cfg.Mouse {
		screen.EnableMouse()
	}
	startMenuKeyWatcher(screen)
	defer func() {
		if r := recover(); r != nil {
			screen.Fini()
			panic(r)
		}
	}()
	var left, right string
	if args := flag.Args(); len(args) > 0 {
		left = args[0]
		if len(args) > 1 {
			right = args[1]
		}
	}
	app.setup(left, right)
	app.startIndex()
	app.run()
	screen.Fini()
}
