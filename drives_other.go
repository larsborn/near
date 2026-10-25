//go:build !windows && !linux && !darwin && !freebsd

package main

// fixedDrives falls back to the root file system where mounts cannot be
// enumerated.
func fixedDrives() []string { return []string{"/"} }
