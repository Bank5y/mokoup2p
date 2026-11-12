//go:build !windows

package ipc

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

// DefaultSocketPath is where the daemon listens for the current user. The
// systemd unit (deploy/systemd/mokoup2pd@.service, instance = UID) listens
// on SystemSocketPath; a daemon started by hand uses XDG_RUNTIME_DIR.
func DefaultSocketPath() string {
	if p := os.Getenv("MOKOUP2P_SOCKET"); p != "" {
		return p
	}
	if p := SystemSocketPath(os.Getuid()); fileExists(p) {
		return p
	}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "mokoup2p", "daemon.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("mokoup2p-%d", os.Getuid()), "daemon.sock")
}

// SystemSocketPath is the socket of the systemd service instance running
// for the user with the given UID.
func SystemSocketPath(uid int) string {
	return filepath.Join("/run/mokoup2p", strconv.Itoa(uid), "daemon.sock")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dial(ctx context.Context, socketPath string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "unix", socketPath)
}
