package main

import (
	"errors"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// runElevated starts exe with args through the UAC prompt ("runas") and
// returns once it has started.
func runElevated(exe, args string) error {
	verb, _ := windows.UTF16PtrFromString("runas")
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	params, _ := windows.UTF16PtrFromString(args)
	dir, _ := windows.UTF16PtrFromString(filepath.Dir(exe))
	err = windows.ShellExecute(0, verb, file, params, dir, windows.SW_SHOWNORMAL)
	if errors.Is(err, windows.ERROR_CANCELLED) {
		return errElevationCancelled
	}
	return err
}
