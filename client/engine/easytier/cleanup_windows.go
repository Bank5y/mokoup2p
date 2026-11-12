package easytier

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// killStaleCore kills pid if it is still an easytier-core.exe (the PID may
// have been reused since). Normally the job object (internal/etproc) already
// took it down with the previous daemon.
func killStaleCore(pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return nil // gone, or not ours to touch
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return nil
	}
	if !strings.EqualFold(filepath.Base(windows.UTF16ToString(buf[:n])), "easytier-core.exe") {
		return nil
	}
	if err := windows.TerminateProcess(h, 1); err != nil {
		return fmt.Errorf("kill stale easytier-core (pid %d): %w", pid, err)
	}
	if ev, err := windows.WaitForSingleObject(h, 5000); err != nil || ev != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("stale easytier-core (pid %d) did not exit", pid)
	}
	return nil
}

// removeDevice has nothing to do: Wintun removes the adapter when the
// process that created it is gone.
func removeDevice(string) error { return nil }
