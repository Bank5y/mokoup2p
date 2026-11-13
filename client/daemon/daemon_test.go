package daemon_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/daemontest"
	"github.com/Bank5y/mokoup2p/client/ipc"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

var ctx = context.Background()

func TestFreshDaemonIsLoggedOutWithHostnameAsDeviceName(t *testing.T) {
	d := daemontest.Start(t)

	st, err := d.IPC.Status(ctx)
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if st.State != ipc.StateLoggedOut {
		t.Fatalf("want logged out, got %s", st.State)
	}
	if st.DeviceID == "" || st.DeviceName != daemontest.Hostname {
		t.Fatalf("want generated device id and hostname device name, got %+v", st)
	}
}

func TestLoginWithoutServerAddressFails(t *testing.T) {
	d := daemontest.Start(t)

	_, err := d.IPC.Login(ctx, servertest.AdminUsername, servertest.AdminPassword)

	if code := servertest.ErrorCode(err); code != ipc.ErrServerNotConfigured {
		t.Fatalf("want %s, got %v", ipc.ErrServerNotConfigured, err)
	}
}

func TestLoginThroughDaemon(t *testing.T) {
	s := servertest.Start(t)
	d := daemontest.Start(t)

	if _, err := d.IPC.SetServer(ctx, s.URL); err != nil {
		t.Fatalf("set server: %v", err)
	}
	st, err := d.IPC.Login(ctx, servertest.AdminUsername, servertest.AdminPassword)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if st.State != ipc.StateIdle || st.Username != servertest.AdminUsername || !st.IsAdmin || st.ServerURL != s.URL {
		t.Fatalf("unexpected status after login %+v", st)
	}
}

func TestLoginWithWrongPasswordReportsServerErrorCode(t *testing.T) {
	s := servertest.Start(t)
	d := daemontest.Start(t)
	mustSetServer(t, d, s)

	_, err := d.IPC.Login(ctx, servertest.AdminUsername, "wrong")

	if code := servertest.ErrorCode(err); code != api.ErrInvalidCredentials {
		t.Fatalf("want %s, got %v", api.ErrInvalidCredentials, err)
	}
	st, _ := d.IPC.Status(ctx)
	if st.State != ipc.StateLoggedOut {
		t.Fatalf("want logged out, got %s", st.State)
	}
}

func TestLoginWhenServerUnreachable(t *testing.T) {
	d := daemontest.Start(t)
	if _, err := d.IPC.SetServer(ctx, "http://127.0.0.1:1"); err != nil {
		t.Fatalf("set server: %v", err)
	}

	_, err := d.IPC.Login(ctx, servertest.AdminUsername, servertest.AdminPassword)

	if code := servertest.ErrorCode(err); code != ipc.ErrServerUnreachable {
		t.Fatalf("want %s, got %v", ipc.ErrServerUnreachable, err)
	}
}

func TestDaemonLogsInAutomaticallyOnRestart(t *testing.T) {
	s := servertest.Start(t)
	d := daemontest.Start(t)
	mustSetServer(t, d, s)
	before := mustLogin(t, d)

	d.Restart(t)

	st, err := d.IPC.Status(ctx)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.State != ipc.StateIdle || st.Username != servertest.AdminUsername {
		t.Fatalf("want automatically logged in, got %+v", st)
	}
	if st.DeviceID != before.DeviceID || st.ServerURL != s.URL {
		t.Fatalf("device id and server address should persist: before %+v after %+v", before, st)
	}
}

func TestDaemonLogsInAutomaticallyAcrossSeveralRestarts(t *testing.T) {
	s := servertest.Start(t)
	d := daemontest.Start(t)
	mustSetServer(t, d, s)
	mustLogin(t, d)

	d.Restart(t)
	d.Restart(t)

	st, _ := d.IPC.Status(ctx)
	if st.State != ipc.StateIdle {
		t.Fatalf("want automatically logged in after second restart, got %+v", st)
	}
}

func TestAutoLoginWithExpiredRefreshTokenEndsLoggedOut(t *testing.T) {
	s := servertest.Start(t)
	d := daemontest.Start(t)
	mustSetServer(t, d, s)
	mustLogin(t, d)

	s.Clock.Advance(31 * 24 * time.Hour)
	d.Restart(t)

	st, _ := d.IPC.Status(ctx)
	if st.State != ipc.StateLoggedOut {
		t.Fatalf("want logged out, got %+v", st)
	}
}

func TestAutoLoginRetriesUntilServerIsReachable(t *testing.T) {
	s := servertest.Start(t)
	d := daemontest.Start(t)
	mustSetServer(t, d, s)
	mustLogin(t, d)

	s.SetUnreachable(true)
	d.Restart(t)
	if st, _ := d.IPC.Status(ctx); st.State != ipc.StateLoggedOut {
		t.Fatalf("want logged out while server is unreachable, got %+v", st)
	}
	s.SetUnreachable(false)

	waitFor(t, func() bool {
		st, _ := d.IPC.Status(ctx)
		return st.State == ipc.StateIdle
	})
}

func TestDaemonCleansUpEngineOnStart(t *testing.T) {
	d := daemontest.Start(t)

	if n := d.Engine.CleanupCalls(); n != 1 {
		t.Fatalf("want engine cleanup once on start, got %d", n)
	}
}

func TestSocketIsAccessibleOnlyByCurrentUser(t *testing.T) {
	d := daemontest.Start(t)

	info, err := os.Stat(d.SocketPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("socket should not be accessible by group or others, mode %v", perm)
	}
}

func mustSetServer(t *testing.T, d *daemontest.Daemon, s *servertest.Server) {
	t.Helper()
	if _, err := d.IPC.SetServer(ctx, s.URL); err != nil {
		t.Fatalf("set server: %v", err)
	}
}

func mustLogin(t *testing.T, d *daemontest.Daemon) ipc.Status {
	t.Helper()
	st, err := d.IPC.Login(ctx, servertest.AdminUsername, servertest.AdminPassword)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	return st
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met within 5s")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
