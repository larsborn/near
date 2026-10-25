//go:build !windows

package main

import "github.com/gdamore/tcell/v2"

// startMenuKeyWatcher is only needed on the Windows console.
func startMenuKeyWatcher(s tcell.Screen) {}
