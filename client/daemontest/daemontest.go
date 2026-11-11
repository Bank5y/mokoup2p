// Package daemontest runs a real client daemon for tests (seam 2): it talks
// to a servertest server, uses a fake engine, and is driven over IPC.
package daemontest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Bank5y/mokoup2p/client/daemon"
	"github.com/Bank5y/mokoup2p/client/engine/enginetest"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

const Hostname = "test-host"

type Daemon struct {
	IPC        *ipc.Client
	Engine     *enginetest.Fake
	SocketPath string

	stateDir string
	d        *daemon.Daemon
	done     chan struct{}
}

// Start runs a daemon with empty state. It is stopped when the test ends.
func Start(t testing.TB) *Daemon {
	t.Helper()
	// Unix socket paths are limited to ~108 bytes, so avoid t.TempDir's long names.
	sockDir, err := os.MkdirTemp("", "mkp2p")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(sockDir) })
	d := &Daemon{
		stateDir:   t.TempDir(),
		SocketPath: filepath.Join(sockDir, "s", "daemon.sock"),
	}
	d.start(t)
	t.Cleanup(d.stop)
	return d
}

// Restart stops the daemon and starts it again with the same saved state, as
// if the machine rebooted. The fake engine is replaced with a fresh one.
func (d *Daemon) Restart(t testing.TB) {
	t.Helper()
	d.stop()
	d.start(t)
}

func (d *Daemon) start(t testing.TB) {
	t.Helper()
	d.Engine = &enginetest.Fake{}
	dm, err := daemon.New(daemon.Config{
		StateDir: d.stateDir,
		Engine:   d.Engine,
		Hostname: func() (string, error) { return Hostname, nil },
		// Retry quickly so tests that restore the server don't wait.
		AutoLoginRetry: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new daemon: %v", err)
	}
	dm.Start(context.Background())
	l, err := daemon.Listen(d.SocketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	d.d = dm
	d.done = make(chan struct{})
	go func() {
		defer close(d.done)
		_ = dm.Serve(l)
	}()
	d.IPC = ipc.NewClient(d.SocketPath)
}

func (d *Daemon) stop() {
	if d.d == nil {
		return
	}
	d.d.Close()
	<-d.done
	d.d = nil
}
