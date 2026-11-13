//go:build !windows

package main

import "errors"

// Only the Windows installer edition installs updates itself.
func startInstaller(string) error { return errors.New("not supported on this platform") }

func afterUpdate() bool { return false }
