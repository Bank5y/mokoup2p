package easytier

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// killStaleCore kills pid and its process group if it is still an
// easytier-core (the PID may have been reused since).
func killStaleCore(pid int) error {
	comm, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil || strings.TrimSpace(string(comm)) != "easytier-core" {
		return nil
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
	for range 50 {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("stale easytier-core (pid %d) did not exit", pid)
}

// removeDevice deletes the TUN device if it still exists (only possible if
// something made it persistent).
func removeDevice(name string) error {
	if _, err := net.InterfaceByName(name); err != nil {
		return nil
	}
	if out, err := exec.Command("ip", "link", "del", name).CombinedOutput(); err != nil {
		return fmt.Errorf("remove leftover device %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}
