//go:build !linux

package easytier

// Leftover cleanup outside Linux arrives with the Windows client (#12).
func killStaleCore(int) error { return nil }

func removeDevice(string) error { return nil }
