package server_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

func TestAdministratorCreatedFromEnvironmentCanLogIn(t *testing.T) {
	s := servertest.Start(t)

	sess, err := s.Login(servertest.AdminUsername, servertest.AdminPassword, "dev-1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if sess.AccessToken == "" || sess.RefreshToken == "" {
		t.Fatalf("expected both tokens, got %+v", sess)
	}
	if sess.User.Username != servertest.AdminUsername || !sess.User.IsAdmin {
		t.Fatalf("expected admin user, got %+v", sess.User)
	}
}

func TestRestartDoesNotRecreateAdministrator(t *testing.T) {
	s := servertest.Start(t)

	s.Restart(t, "other-admin", "other-password")

	if _, err := s.Login(servertest.AdminUsername, servertest.AdminPassword, "dev-1"); err != nil {
		t.Fatalf("original admin should still log in: %v", err)
	}
	_, err := s.Login("other-admin", "other-password", "dev-1")
	if code := servertest.ErrorCode(err); code != api.ErrInvalidCredentials {
		t.Fatalf("second admin should not exist, got err %v", err)
	}
}

func TestLoginWithWrongPasswordIsRejected(t *testing.T) {
	s := servertest.Start(t)

	_, err := s.Login(servertest.AdminUsername, "wrong", "dev-1")

	if code := servertest.ErrorCode(err); code != api.ErrInvalidCredentials {
		t.Fatalf("want %s, got %v", api.ErrInvalidCredentials, err)
	}
}

func TestLoginWithUnknownUserIsRejected(t *testing.T) {
	s := servertest.Start(t)

	_, err := s.Login("nobody", "whatever", "dev-1")

	if code := servertest.ErrorCode(err); code != api.ErrInvalidCredentials {
		t.Fatalf("want %s, got %v", api.ErrInvalidCredentials, err)
	}
}

func TestAccessTokenIdentifiesUserAndDevice(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)

	var me api.Me
	if err := s.Do(http.MethodGet, api.PathMe, sess.AccessToken, nil, &me); err != nil {
		t.Fatalf("me: %v", err)
	}

	if me.User.Username != servertest.AdminUsername || me.Device.ID != "dev-1" || me.Device.Name != "dev-1-host" {
		t.Fatalf("unexpected identity %+v", me)
	}
}

func TestExpiredAccessTokenIsRejected(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)

	s.Clock.Advance(16 * time.Minute)

	err := s.Do(http.MethodGet, api.PathMe, sess.AccessToken, nil, nil)
	if code := servertest.ErrorCode(err); code != api.ErrTokenInvalid {
		t.Fatalf("want %s, got %v", api.ErrTokenInvalid, err)
	}
}

func TestRequestWithoutTokenIsRejected(t *testing.T) {
	s := servertest.Start(t)

	err := s.Do(http.MethodGet, api.PathMe, "", nil, nil)

	if code := servertest.ErrorCode(err); code != api.ErrTokenInvalid {
		t.Fatalf("want %s, got %v", api.ErrTokenInvalid, err)
	}
}

func TestRefreshIssuesNewWorkingSession(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)
	s.Clock.Advance(16 * time.Minute)

	var refreshed api.Session
	if err := s.Do(http.MethodPost, api.PathRefresh, "", api.RefreshRequest{RefreshToken: sess.RefreshToken}, &refreshed); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	var me api.Me
	if err := s.Do(http.MethodGet, api.PathMe, refreshed.AccessToken, nil, &me); err != nil {
		t.Fatalf("new access token should work: %v", err)
	}
	if refreshed.User.Username != servertest.AdminUsername || refreshed.Device.ID != "dev-1" {
		t.Fatalf("unexpected refreshed session %+v", refreshed)
	}
}

func TestRefreshTokenCanOnlyBeUsedOnce(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)
	if err := s.Do(http.MethodPost, api.PathRefresh, "", api.RefreshRequest{RefreshToken: sess.RefreshToken}, nil); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	err := s.Do(http.MethodPost, api.PathRefresh, "", api.RefreshRequest{RefreshToken: sess.RefreshToken}, nil)

	if code := servertest.ErrorCode(err); code != api.ErrTokenInvalid {
		t.Fatalf("want %s, got %v", api.ErrTokenInvalid, err)
	}
}

func TestExpiredRefreshTokenIsRejected(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)

	s.Clock.Advance(31 * 24 * time.Hour)

	err := s.Do(http.MethodPost, api.PathRefresh, "", api.RefreshRequest{RefreshToken: sess.RefreshToken}, nil)
	if code := servertest.ErrorCode(err); code != api.ErrTokenInvalid {
		t.Fatalf("want %s, got %v", api.ErrTokenInvalid, err)
	}
}

func TestAccessTokenCannotBeUsedAsRefreshToken(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)

	err := s.Do(http.MethodPost, api.PathRefresh, "", api.RefreshRequest{RefreshToken: sess.AccessToken}, nil)

	if code := servertest.ErrorCode(err); code != api.ErrTokenInvalid {
		t.Fatalf("want %s, got %v", api.ErrTokenInvalid, err)
	}
}

func mustLogin(t *testing.T, s *servertest.Server) api.Session {
	t.Helper()
	sess, err := s.Login(servertest.AdminUsername, servertest.AdminPassword, "dev-1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	return sess
}
