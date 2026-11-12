//go:build !linux

package main

// trayAvailable reports whether the desktop shows tray icons. Windows always
// has a notification area.
func trayAvailable() bool { return true }
