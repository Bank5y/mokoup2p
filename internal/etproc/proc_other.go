//go:build !linux && !windows

package etproc

import (
	"os"
	"os/exec"
	"syscall"
)

// Process groups and parent-death signals are Linux-only, job objects
// Windows-only; elsewhere children are just killed.
func sysProcAttr() *syscall.SysProcAttr { return nil }

func terminate(pid int) error { return kill(pid) }

func kill(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}

func afterStart(*exec.Cmd) error { return nil }
