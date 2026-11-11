package server_test

import (
	"net/http"
	"testing"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

func adminSession(t *testing.T, s *servertest.Server) api.Session {
	t.Helper()
	return loginAs(t, s, servertest.AdminUsername, servertest.AdminPassword, "admin-dev")
}

func createUser(t *testing.T, s *servertest.Server, adminToken, username, password string) api.AdminUser {
	t.Helper()
	var u api.AdminUser
	err := s.Do(http.MethodPost, api.PathAdminUsers, adminToken, api.CreateUserRequest{Username: username, Password: password}, &u)
	if err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return u
}

func TestAdministratorCreatesUserWhoCanLogIn(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)

	u := createUser(t, s, admin.AccessToken, "alice", "alice-password")
	if u.Username != "alice" || u.IsAdmin || u.Disabled {
		t.Fatalf("unexpected user %+v", u)
	}

	sess := loginAs(t, s, "alice", "alice-password", "alice-dev")
	if sess.User.IsAdmin {
		t.Fatalf("new user should not be an administrator")
	}
	var users []api.AdminUser
	if err := s.Do(http.MethodGet, api.PathAdminUsers, admin.AccessToken, nil, &users); err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 2 || users[0].Username != servertest.AdminUsername || users[1].Username != "alice" {
		t.Fatalf("unexpected user list %+v", users)
	}
}

func TestCreatingDuplicateUsernameIsRejected(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)
	createUser(t, s, admin.AccessToken, "alice", "alice-password")

	err := s.Do(http.MethodPost, api.PathAdminUsers, admin.AccessToken, api.CreateUserRequest{Username: "alice", Password: "x"}, nil)

	if code := servertest.ErrorCode(err); code != api.ErrUsernameTaken {
		t.Fatalf("want %s, got %v", api.ErrUsernameTaken, err)
	}
}

func TestNonAdministratorCannotUseAdministratorAPI(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)
	alice := createUser(t, s, admin.AccessToken, "alice", "alice-password")
	sess := loginAs(t, s, "alice", "alice-password", "alice-dev")

	calls := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, api.PathAdminUsers, nil},
		{http.MethodPost, api.PathAdminUsers, api.CreateUserRequest{Username: "bob", Password: "x"}},
		{http.MethodPut, api.PathAdminUser(alice.ID) + api.AdminUserAdmin, api.SetAdminRequest{IsAdmin: true}},
		{http.MethodPut, api.PathAdminUser(alice.ID) + api.AdminUserPassword, api.ResetPasswordRequest{Password: "x"}},
		{http.MethodPut, api.PathAdminUser(alice.ID) + api.AdminUserDisabled, api.SetDisabledRequest{Disabled: true}},
		{http.MethodDelete, api.PathAdminUser(alice.ID), nil},
	}
	for _, c := range calls {
		err := s.Do(c.method, c.path, sess.AccessToken, c.body, nil)
		if code := servertest.ErrorCode(err); code != api.ErrForbidden {
			t.Errorf("%s %s: want %s, got %v", c.method, c.path, api.ErrForbidden, err)
		}
	}
	loginAs(t, s, "alice", "alice-password", "alice-dev")
}

func TestAdministratorResetsUserPassword(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)
	alice := createUser(t, s, admin.AccessToken, "alice", "alice-password")

	err := s.Do(http.MethodPut, api.PathAdminUser(alice.ID)+api.AdminUserPassword, admin.AccessToken,
		api.ResetPasswordRequest{Password: "reset-password"}, nil)
	if err != nil {
		t.Fatalf("reset password: %v", err)
	}

	if _, err := s.Login("alice", "alice-password", "alice-dev"); servertest.ErrorCode(err) != api.ErrInvalidCredentials {
		t.Fatalf("old password should no longer work, got %v", err)
	}
	loginAs(t, s, "alice", "reset-password", "alice-dev")
}

func TestDisablingUserRevokesAllTokensAndBlocksLogin(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)
	alice := createUser(t, s, admin.AccessToken, "alice", "alice-password")
	laptop := loginAs(t, s, "alice", "alice-password", "laptop")
	desktop := loginAs(t, s, "alice", "alice-password", "desktop")

	var u api.AdminUser
	err := s.Do(http.MethodPut, api.PathAdminUser(alice.ID)+api.AdminUserDisabled, admin.AccessToken,
		api.SetDisabledRequest{Disabled: true}, &u)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if !u.Disabled {
		t.Fatalf("want disabled user, got %+v", u)
	}

	for _, sess := range []api.Session{laptop, desktop} {
		if err := s.Do(http.MethodGet, api.PathMe, sess.AccessToken, nil, nil); servertest.ErrorCode(err) != api.ErrTokenInvalid {
			t.Errorf("access token should be revoked, got %v", err)
		}
		err := s.Do(http.MethodPost, api.PathRefresh, "", api.RefreshRequest{RefreshToken: sess.RefreshToken}, nil)
		if servertest.ErrorCode(err) != api.ErrTokenInvalid {
			t.Errorf("refresh token should be revoked, got %v", err)
		}
	}
	if _, err := s.Login("alice", "alice-password", "laptop"); servertest.ErrorCode(err) != api.ErrAccountDisabled {
		t.Fatalf("want %s, got %v", api.ErrAccountDisabled, err)
	}
}

func TestEnablingUserAllowsLoginAgain(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)
	alice := createUser(t, s, admin.AccessToken, "alice", "alice-password")
	path := api.PathAdminUser(alice.ID) + api.AdminUserDisabled
	if err := s.Do(http.MethodPut, path, admin.AccessToken, api.SetDisabledRequest{Disabled: true}, nil); err != nil {
		t.Fatalf("disable: %v", err)
	}

	if err := s.Do(http.MethodPut, path, admin.AccessToken, api.SetDisabledRequest{Disabled: false}, nil); err != nil {
		t.Fatalf("enable: %v", err)
	}

	loginAs(t, s, "alice", "alice-password", "laptop")
}

func TestAdministratorDeletesUser(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)
	alice := createUser(t, s, admin.AccessToken, "alice", "alice-password")
	sess := loginAs(t, s, "alice", "alice-password", "laptop")

	if err := s.Do(http.MethodDelete, api.PathAdminUser(alice.ID), admin.AccessToken, nil, nil); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if err := s.Do(http.MethodGet, api.PathMe, sess.AccessToken, nil, nil); servertest.ErrorCode(err) != api.ErrTokenInvalid {
		t.Fatalf("deleted user's token should be invalid, got %v", err)
	}
	if _, err := s.Login("alice", "alice-password", "laptop"); servertest.ErrorCode(err) != api.ErrInvalidCredentials {
		t.Fatalf("deleted user should not log in, got %v", err)
	}
	// The username is free again.
	createUser(t, s, admin.AccessToken, "alice", "another-password")
}

func TestDeletingUserDeletesTheRoomsTheyOwn(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)
	alice := createUser(t, s, admin.AccessToken, "alice", "alice-password")
	sess := loginAs(t, s, "alice", "alice-password", "laptop")
	var room api.Room
	if err := s.Do(http.MethodPost, api.PathRooms, sess.AccessToken, api.CreateRoomRequest{Name: "开黑"}, &room); err != nil {
		t.Fatalf("create room: %v", err)
	}

	if err := s.Do(http.MethodDelete, api.PathAdminUser(alice.ID), admin.AccessToken, nil, nil); err != nil {
		t.Fatalf("delete: %v", err)
	}

	err := s.Do(http.MethodGet, api.PathRoom(room.Code), admin.AccessToken, nil, nil)
	if code := servertest.ErrorCode(err); code != api.ErrRoomNotFound {
		t.Fatalf("the deleted user's room should be gone: want %s, got %v", api.ErrRoomNotFound, err)
	}
}

func TestOperationsOnUnknownUserReportUserNotFound(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)

	err := s.Do(http.MethodDelete, api.PathAdminUser(999), admin.AccessToken, nil, nil)

	if code := servertest.ErrorCode(err); code != api.ErrUserNotFound {
		t.Fatalf("want %s, got %v", api.ErrUserNotFound, err)
	}
}

func TestPromotedUserCanUseAdministratorAPIAndDemotedCannot(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)
	alice := createUser(t, s, admin.AccessToken, "alice", "alice-password")
	path := api.PathAdminUser(alice.ID) + api.AdminUserAdmin

	if err := s.Do(http.MethodPut, path, admin.AccessToken, api.SetAdminRequest{IsAdmin: true}, nil); err != nil {
		t.Fatalf("promote: %v", err)
	}
	sess := loginAs(t, s, "alice", "alice-password", "laptop")
	if !sess.User.IsAdmin {
		t.Fatalf("alice should be an administrator")
	}
	if err := s.Do(http.MethodGet, api.PathAdminUsers, sess.AccessToken, nil, nil); err != nil {
		t.Fatalf("promoted user should use the administrator API: %v", err)
	}

	if err := s.Do(http.MethodPut, path, admin.AccessToken, api.SetAdminRequest{IsAdmin: false}, nil); err != nil {
		t.Fatalf("demote: %v", err)
	}
	if err := s.Do(http.MethodGet, api.PathAdminUsers, sess.AccessToken, nil, nil); servertest.ErrorCode(err) != api.ErrForbidden {
		t.Fatalf("demoted user should be forbidden, got %v", err)
	}
}

func TestLastAdministratorCannotBeDemotedDeletedOrDisabled(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)
	id := admin.User.ID

	calls := []struct {
		method, path string
		body         any
	}{
		{http.MethodPut, api.PathAdminUser(id) + api.AdminUserAdmin, api.SetAdminRequest{IsAdmin: false}},
		{http.MethodDelete, api.PathAdminUser(id), nil},
		{http.MethodPut, api.PathAdminUser(id) + api.AdminUserDisabled, api.SetDisabledRequest{Disabled: true}},
	}
	for _, c := range calls {
		err := s.Do(c.method, c.path, admin.AccessToken, c.body, nil)
		if code := servertest.ErrorCode(err); code != api.ErrLastAdmin {
			t.Errorf("%s %s: want %s, got %v", c.method, c.path, api.ErrLastAdmin, err)
		}
	}
	if err := s.Do(http.MethodGet, api.PathAdminUsers, admin.AccessToken, nil, nil); err != nil {
		t.Fatalf("administrator should be unaffected: %v", err)
	}
}

func TestAdministratorCanBeDemotedWhileAnotherRemains(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)
	alice := createUser(t, s, admin.AccessToken, "alice", "alice-password")
	if err := s.Do(http.MethodPut, api.PathAdminUser(alice.ID)+api.AdminUserAdmin, admin.AccessToken, api.SetAdminRequest{IsAdmin: true}, nil); err != nil {
		t.Fatalf("promote: %v", err)
	}

	err := s.Do(http.MethodPut, api.PathAdminUser(admin.User.ID)+api.AdminUserAdmin, admin.AccessToken, api.SetAdminRequest{IsAdmin: false}, nil)
	if err != nil {
		t.Fatalf("demote self: %v", err)
	}

	err = s.Do(http.MethodDelete, api.PathAdminUser(alice.ID), loginAs(t, s, "alice", "alice-password", "laptop").AccessToken, nil, nil)
	if code := servertest.ErrorCode(err); code != api.ErrLastAdmin {
		t.Fatalf("alice is now the last administrator: want %s, got %v", api.ErrLastAdmin, err)
	}
}
