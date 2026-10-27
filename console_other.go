//go:build !windows

package main

// ensureConsole is a no-op outside Windows: a terminal is either there or
// tcell reports its absence.
func ensureConsole() {}
