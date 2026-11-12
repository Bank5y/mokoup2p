package server_test

import (
	"net/http"
	"testing"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

func loginAs(t *testing.T, s *servertest.Server, username, password, deviceID string) api.Session {
	t.Helper()
	sess, err := s.Login(username, password, deviceID)
	if err != nil {
		t.Fatalf("login %s: %v", username, err)
	}
	return sess
}

func TestUserChangesOwnPassword(t *testing.T) {
	s := servertest.Start(t)
	sess := loginAs(t, s, servertest.AdminUsername, servertest.AdminPassword, "dev-1")

	err := s.Do(http.MethodPut, api.PathMePassword, sess.AccessToken,
		api.ChangePasswordRequest{OldPassword: servertest.AdminPassword, NewPassword: "new-password"}, nil)
	if err != nil {
		t.Fatalf("change password: %v", err)
	}

	if _, err := s.Login(servertest.AdminUsername, servertest.AdminPassword, "dev-1"); servertest.ErrorCode(err) != api.ErrInvalidCredentials {
		t.Fatalf("old password should no longer work, got %v", err)
	}
	loginAs(t, s, servertest.AdminUsername, "new-password", "dev-1")
}

func TestChangePasswordRequiresCorrectOldPassword(t *testing.T) {
	s := servertest.Start(t)
	sess := loginAs(t, s, servertest.AdminUsername, servertest.AdminPassword, "dev-1")

	err := s.Do(http.MethodPut, api.PathMePassword, sess.AccessToken,
		api.ChangePasswordRequest{OldPassword: "wrong", NewPassword: "new-password"}, nil)

	if code := servertest.ErrorCode(err); code != api.ErrInvalidCredentials {
		t.Fatalf("want %s, got %v", api.ErrInvalidCredentials, err)
	}
	loginAs(t, s, servertest.AdminUsername, servertest.AdminPassword, "dev-1")
}

func TestUserRenamesDevice(t *testing.T) {
	s := servertest.Start(t)
	sess := loginAs(t, s, servertest.AdminUsername, servertest.AdminPassword, "dev-1")

	var dev api.Device
	err := s.Do(http.MethodPut, api.PathMeDevice, sess.AccessToken, api.RenameDeviceRequest{Name: " 客厅电脑 "}, &dev)
	if err != nil {
		t.Fatalf("rename device: %v", err)
	}
	if dev.Name != "客厅电脑" || dev.ID != "dev-1" {
		t.Fatalf("unexpected device %+v", dev)
	}

	var me api.Me
	if err := s.Do(http.MethodGet, api.PathMe, sess.AccessToken, nil, &me); err != nil {
		t.Fatalf("me: %v", err)
	}
	if me.Device.Name != "客厅电脑" {
		t.Fatalf("device name not saved, got %+v", me.Device)
	}
}

func TestEmptyDeviceNameIsRejected(t *testing.T) {
	s := servertest.Start(t)
	sess := loginAs(t, s, servertest.AdminUsername, servertest.AdminPassword, "dev-1")

	err := s.Do(http.MethodPut, api.PathMeDevice, sess.AccessToken, api.RenameDeviceRequest{Name: "  "}, nil)

	if code := servertest.ErrorCode(err); code != api.ErrBadRequest {
		t.Fatalf("want %s, got %v", api.ErrBadRequest, err)
	}
}

func TestLogoutRevokesTheDevicesTokensOnly(t *testing.T) {
	s := servertest.Start(t)
	sess := loginAs(t, s, servertest.AdminUsername, servertest.AdminPassword, "dev-1")
	other := loginAs(t, s, servertest.AdminUsername, servertest.AdminPassword, "dev-2")

	if err := s.Do(http.MethodPost, api.PathLogout, sess.AccessToken, nil, nil); err != nil {
		t.Fatalf("logout: %v", err)
	}

	if err := s.Do(http.MethodGet, api.PathMe, sess.AccessToken, nil, nil); servertest.ErrorCode(err) != api.ErrTokenInvalid {
		t.Fatalf("access token should be revoked, got %v", err)
	}
	err := s.Do(http.MethodPost, api.PathRefresh, "", api.RefreshRequest{RefreshToken: sess.RefreshToken}, nil)
	if servertest.ErrorCode(err) != api.ErrTokenInvalid {
		t.Fatalf("refresh token should be revoked, got %v", err)
	}
	if err := s.Do(http.MethodGet, api.PathMe, other.AccessToken, nil, nil); err != nil {
		t.Fatalf("other device should stay logged in: %v", err)
	}
}
