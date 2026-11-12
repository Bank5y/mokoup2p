//go:build !linux && !windows

package easytier

// No leftover cleanup outside Linux and Windows.
func killStaleCore(int) error { return nil }

func removeDevice(string) error { return nil }
