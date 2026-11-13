//go:build !windows

package main

import "errors"

// runElevated is only needed by the Windows installer edition.
func runElevated(exe, args string) error {
	return errors.New("not supported on this platform")
}
