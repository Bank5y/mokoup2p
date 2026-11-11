package daemon_test

import (
	"testing"
	"time"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/daemontest"
	"github.com/Bank5y/mokoup2p/client/ipc"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

func loggedInDaemon(t *testing.T, s *servertest.Server) *daemontest.Daemon {
	t.Helper()
	d := daemontest.Start(t)
	mustSetServer(t, d, s)
	mustLogin(t, d)
	return d
}

func TestChangePasswordThroughDaemon(t *testing.T) {
	s := servertest.Start(t)
	d := loggedInDaemon(t, s)

	if err := d.IPC.ChangePassword(ctx, servertest.AdminPassword, "new-password"); err != nil {
		t.Fatalf("change password: %v", err)
	}

	if _, err := s.Login(servertest.AdminUsername, "new-password", "other"); err != nil {
		t.Fatalf("new password should work: %v", err)
	}
}

func TestChangePasswordWithWrongOldPasswordReportsInvalidCredentials(t *testing.T) {
	s := servertest.Start(t)
	d := loggedInDaemon(t, s)

	err := d.IPC.ChangePassword(ctx, "wrong", "new-password")

	if code := servertest.ErrorCode(err); code != api.ErrInvalidCredentials {
		t.Fatalf("want %s, got %v", api.ErrInvalidCredentials, err)
	}
	if st, _ := d.IPC.Status(ctx); st.State != ipc.StateIdle {
		t.Fatalf("a wrong old password must not log the daemon out, got %+v", st)
	}
}

func TestAccountOperationsRequireLogin(t *testing.T) {
	s := servertest.Start(t)
	d := daemontest.Start(t)
	mustSetServer(t, d, s)

	err := d.IPC.ChangePassword(ctx, servertest.AdminPassword, "new-password")

	if code := servertest.ErrorCode(err); code != ipc.ErrNotLoggedIn {
		t.Fatalf("want %s, got %v", ipc.ErrNotLoggedIn, err)
	}
}

func TestRenameDeviceThroughDaemon(t *testing.T) {
	s := servertest.Start(t)
	d := loggedInDaemon(t, s)

	st, err := d.IPC.SetDeviceName(ctx, "客厅电脑")
	if err != nil {
		t.Fatalf("rename device: %v", err)
	}
	if st.DeviceName != "客厅电脑" {
		t.Fatalf("status should show the new name, got %+v", st)
	}

	// The server knows the new name: another login from this device sees it.
	sess, err := s.Login(servertest.AdminUsername, servertest.AdminPassword, st.DeviceID)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if sess.Device.Name != "客厅电脑" {
		t.Fatalf("server should store the new name, got %+v", sess.Device)
	}
	d.Restart(t)
	if st, _ := d.IPC.Status(ctx); st.DeviceName != "客厅电脑" {
		t.Fatalf("device name should persist across restarts, got %+v", st)
	}
}

func TestRenameDeviceWhileLoggedOutIsUsedAtNextLogin(t *testing.T) {
	s := servertest.Start(t)
	d := daemontest.Start(t)
	mustSetServer(t, d, s)

	if _, err := d.IPC.SetDeviceName(ctx, "书房"); err != nil {
		t.Fatalf("rename device: %v", err)
	}
	st := mustLogin(t, d)

	sess, err := s.Login(servertest.AdminUsername, servertest.AdminPassword, st.DeviceID)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if sess.Device.Name != "书房" {
		t.Fatalf("server should know the name chosen before login, got %+v", sess.Device)
	}
}

func TestDeviceRenamedWhileLoggedOutIsSyncedWhenLoggingInAgain(t *testing.T) {
	s := servertest.Start(t)
	d := loggedInDaemon(t, s)
	if _, err := d.IPC.Logout(ctx); err != nil {
		t.Fatalf("logout: %v", err)
	}

	if _, err := d.IPC.SetDeviceName(ctx, "改名后"); err != nil {
		t.Fatalf("rename device: %v", err)
	}
	st := mustLogin(t, d)

	sess, err := s.Login(servertest.AdminUsername, servertest.AdminPassword, st.DeviceID)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if sess.Device.Name != "改名后" {
		t.Fatalf("server should learn the name chosen while logged out, got %+v", sess.Device)
	}
}

func TestEmptyDeviceNameIsRejectedByDaemon(t *testing.T) {
	d := daemontest.Start(t)

	_, err := d.IPC.SetDeviceName(ctx, "   ")

	if code := servertest.ErrorCode(err); code != api.ErrBadRequest {
		t.Fatalf("want %s, got %v", api.ErrBadRequest, err)
	}
}

func TestLogoutReturnsDaemonToLoggedOut(t *testing.T) {
	s := servertest.Start(t)
	d := loggedInDaemon(t, s)

	st, err := d.IPC.Logout(ctx)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	if st.State != ipc.StateLoggedOut || st.Username != "" {
		t.Fatalf("want logged out, got %+v", st)
	}

	d.Restart(t)
	if st, _ := d.IPC.Status(ctx); st.State != ipc.StateLoggedOut {
		t.Fatalf("must not log in automatically after logout, got %+v", st)
	}
}

func TestLogoutWhileServerUnreachableStillLogsOutLocally(t *testing.T) {
	s := servertest.Start(t)
	d := loggedInDaemon(t, s)
	s.SetUnreachable(true)

	st, err := d.IPC.Logout(ctx)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	if st.State != ipc.StateLoggedOut {
		t.Fatalf("want logged out, got %+v", st)
	}
}

func TestExpiredAccessTokenIsRefreshedTransparently(t *testing.T) {
	s := servertest.Start(t)
	d := loggedInDaemon(t, s)

	s.Clock.Advance(20 * time.Minute)

	if _, err := d.IPC.SetDeviceName(ctx, "new-name"); err != nil {
		t.Fatalf("rename after access token expiry: %v", err)
	}
	if st, _ := d.IPC.Status(ctx); st.State != ipc.StateIdle {
		t.Fatalf("want still logged in, got %+v", st)
	}
}
