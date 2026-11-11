package daemon_test

import (
	"testing"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/daemontest"
	"github.com/Bank5y/mokoup2p/client/ipc"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

func daemonLoggedInAs(t *testing.T, s *servertest.Server, username, password string) *daemontest.Daemon {
	t.Helper()
	d := daemontest.Start(t)
	mustSetServer(t, d, s)
	if _, err := d.IPC.Login(ctx, username, password); err != nil {
		t.Fatalf("login %s: %v", username, err)
	}
	return d
}

func TestAdministratorManagesUsersThroughDaemon(t *testing.T) {
	s := servertest.Start(t)
	admin := adminDaemon(t, s)

	alice, err := admin.IPC.CreateUser(ctx, "alice", "alice-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := admin.IPC.CreateUser(ctx, "alice", "x"); servertest.ErrorCode(err) != api.ErrUsernameTaken {
		t.Fatalf("duplicate: want %s, got %v", api.ErrUsernameTaken, err)
	}
	if _, err := admin.IPC.ResetUserPassword(ctx, alice.ID, "reset-password"); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	if _, err := s.Login("alice", "reset-password", "alice-dev"); err != nil {
		t.Fatalf("reset password should work: %v", err)
	}
	u, err := admin.IPC.SetUserAdmin(ctx, alice.ID, true)
	if err != nil || !u.IsAdmin {
		t.Fatalf("promote: %+v %v", u, err)
	}
	u, err = admin.IPC.SetUserDisabled(ctx, alice.ID, true)
	if err != nil || !u.Disabled {
		t.Fatalf("disable: %+v %v", u, err)
	}
	if _, err := s.Login("alice", "reset-password", "alice-dev"); servertest.ErrorCode(err) != api.ErrAccountDisabled {
		t.Fatalf("disabled user login: want %s, got %v", api.ErrAccountDisabled, err)
	}
	users, err := admin.IPC.ListUsers(ctx)
	if err != nil || len(users) != 2 || users[1].Username != "alice" || !users[1].Disabled || !users[1].IsAdmin {
		t.Fatalf("list users: %+v %v", users, err)
	}
	if err := admin.IPC.DeleteUser(ctx, alice.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if users, _ := admin.IPC.ListUsers(ctx); len(users) != 1 {
		t.Fatalf("want only the administrator left, got %+v", users)
	}
}

func TestLastAdministratorErrorIsRelayedByDaemon(t *testing.T) {
	s := servertest.Start(t)
	admin := adminDaemon(t, s)
	users, err := admin.IPC.ListUsers(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	err = admin.IPC.DeleteUser(ctx, users[0].ID)

	if code := servertest.ErrorCode(err); code != api.ErrLastAdmin {
		t.Fatalf("want %s, got %v", api.ErrLastAdmin, err)
	}
}

func TestNonAdministratorDaemonIsForbiddenFromAdministratorAPI(t *testing.T) {
	s := servertest.Start(t)
	admin := adminDaemon(t, s)
	if _, err := admin.IPC.CreateUser(ctx, "alice", "alice-password"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	alice := daemonLoggedInAs(t, s, "alice", "alice-password")

	if st, _ := alice.IPC.Status(ctx); st.IsAdmin {
		t.Fatalf("alice should not be reported as administrator: %+v", st)
	}
	_, err := alice.IPC.ListUsers(ctx)
	if code := servertest.ErrorCode(err); code != api.ErrForbidden {
		t.Fatalf("want %s, got %v", api.ErrForbidden, err)
	}
}

func TestDisabledUsersDaemonEndsLoggedOut(t *testing.T) {
	s := servertest.Start(t)
	admin := adminDaemon(t, s)
	created, err := admin.IPC.CreateUser(ctx, "alice", "alice-password")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	alice := daemonLoggedInAs(t, s, "alice", "alice-password")

	if _, err := admin.IPC.SetUserDisabled(ctx, created.ID, true); err != nil {
		t.Fatalf("disable: %v", err)
	}

	if _, err := alice.IPC.SetDeviceName(ctx, "x"); err == nil {
		t.Fatalf("disabled user's request should fail")
	}
	if st, _ := alice.IPC.Status(ctx); st.State != ipc.StateLoggedOut {
		t.Fatalf("want logged out, got %+v", st)
	}
	_, err = alice.IPC.Login(ctx, "alice", "alice-password")
	if code := servertest.ErrorCode(err); code != api.ErrAccountDisabled {
		t.Fatalf("want %s, got %v", api.ErrAccountDisabled, err)
	}
}
