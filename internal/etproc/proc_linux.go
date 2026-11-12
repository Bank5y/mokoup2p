package etproc

import (
	"os/exec"
	"syscall"
)

// sysProcAttr puts the child in its own process group (so Stop reaches
// anything it spawns) and has the kernel kill it if the parent dies without
// stopping it.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}

func terminate(pid int) error { return syscall.Kill(-pid, syscall.SIGTERM) }

func kill(pid int) error { return syscall.Kill(-pid, syscall.SIGKILL) }

// afterStart has nothing to do: Pdeathsig already ties the child to us.
func afterStart(*exec.Cmd) error { return nil }
