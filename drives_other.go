//go:build !windows && !linux && !darwin && !freebsd

package main

// fixedDrives falls back to the root file system where mounts cannot be
// enumerated.
func fixedDrives() []string { return []string{"/"} }

// isNetworkPath cannot be answered here, so nothing is cached.
func isNetworkPath(p string) bool { return false }
