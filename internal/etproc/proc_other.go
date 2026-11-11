//go:build !linux

package etproc

import (
	"os"
	"syscall"
)

// Process groups and parent-death signals are Linux-only here; the Windows
// client (ticket #12) brings its own job-object handling.
func sysProcAttr() *syscall.SysProcAttr { return nil }

func terminate(pid int) error { return kill(pid) }

func kill(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}
