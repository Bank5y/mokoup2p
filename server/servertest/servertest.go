// Package servertest starts a real server for tests (seam 1): temporary
// SQLite, injectable fake clock, interaction only over HTTP.
package servertest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/internal/clock"
	"github.com/Bank5y/mokoup2p/server"
)

const (
	AdminUsername = "admin"
	AdminPassword = "admin-password"
	// RelayPeer is the relay node address the test server hands out.
	RelayPeer = "tcp://relay.test:11010"
)

type Server struct {
	URL   string
	Clock *clock.Fake

	dbPath      string
	srv         *server.Server
	http        *httptest.Server
	unreachable atomic.Bool
}

// Start launches a server with a fresh database and the default admin
// credentials. It is shut down when the test ends.
func Start(t testing.TB) *Server {
	t.Helper()
	s := &Server{
		Clock:  clock.NewFake(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		dbPath: filepath.Join(t.TempDir(), "mokoup2p.db"),
	}
	s.start(t, AdminUsername, AdminPassword)
	t.Cleanup(s.stop)
	return s
}

// Restart stops the server and starts it again on the same database, as if
// the container restarted with the given admin environment variables.
func (s *Server) Restart(t testing.TB, adminUsername, adminPassword string) {
	t.Helper()
	s.stop()
	s.start(t, adminUsername, adminPassword)
}

func (s *Server) start(t testing.TB, adminUsername, adminPassword string) {
	t.Helper()
	srv, err := server.New(server.Config{
		DBPath:        s.dbPath,
		Clock:         s.Clock,
		AdminUsername: adminUsername,
		AdminPassword: adminPassword,
		RelayPeers:    []string{RelayPeer},
	})
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	s.srv = srv
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.unreachable.Load() {
			// Drop the connection without a response, like a network failure.
			if conn, _, err := http.NewResponseController(w).Hijack(); err == nil {
				conn.Close()
			}
			return
		}
		srv.Handler().ServeHTTP(w, r)
	})
	if s.http == nil {
		s.http = httptest.NewServer(handler)
		s.URL = s.http.URL
	} else {
		s.http.Config.Handler = handler
	}
}

// SetUnreachable makes every request fail at the network level (true) or
// restores normal service (false).
func (s *Server) SetUnreachable(v bool) { s.unreachable.Store(v) }

func (s *Server) stop() {
	if s.srv != nil {
		s.srv.Close()
		s.srv = nil
	}
}

// Do sends a JSON request. On 2xx the body is decoded into out (if non-nil);
// otherwise the returned error is an *api.Error.
func (s *Server) Do(method, path, accessToken string, in, out any) error {
	var body bytes.Buffer
	if in != nil {
		if err := json.NewEncoder(&body).Encode(in); err != nil {
			return err
		}
	}
	req, err := http.NewRequest(method, s.URL+path, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		apiErr := &api.Error{}
		if err := json.NewDecoder(resp.Body).Decode(apiErr); err != nil {
			return fmt.Errorf("status %d with undecodable body: %w", resp.StatusCode, err)
		}
		return apiErr
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// Login logs in with the given credentials from the given device.
func (s *Server) Login(username, password, deviceID string) (api.Session, error) {
	var sess api.Session
	err := s.Do(http.MethodPost, api.PathLogin, "", api.LoginRequest{
		Username: username, Password: password, DeviceID: deviceID, DeviceName: deviceID + "-host",
	}, &sess)
	return sess, err
}

// AddUser creates an ordinary user through the administrator API and
// returns its id.
func (s *Server) AddUser(t testing.TB, username, password string) int64 {
	t.Helper()
	admin, err := s.Login(AdminUsername, AdminPassword, "servertest-admin")
	if err != nil {
		t.Fatalf("add user %q: log in as administrator: %v", username, err)
	}
	var u api.AdminUser
	err = s.Do(http.MethodPost, api.PathAdminUsers, admin.AccessToken, api.CreateUserRequest{Username: username, Password: password}, &u)
	if err != nil {
		t.Fatalf("add user %q: %v", username, err)
	}
	return u.ID
}

// DisableUser disables a user through the administrator API.
func (s *Server) DisableUser(t testing.TB, username string) {
	t.Helper()
	admin, err := s.Login(AdminUsername, AdminPassword, "servertest-admin")
	if err != nil {
		t.Fatalf("disable user %q: log in as administrator: %v", username, err)
	}
	var users []api.AdminUser
	if err := s.Do(http.MethodGet, api.PathAdminUsers, admin.AccessToken, nil, &users); err != nil {
		t.Fatalf("disable user %q: list users: %v", username, err)
	}
	for _, u := range users {
		if u.Username == username {
			err := s.Do(http.MethodPut, api.PathAdminUser(u.ID)+api.AdminUserDisabled, admin.AccessToken, api.SetDisabledRequest{Disabled: true}, nil)
			if err != nil {
				t.Fatalf("disable user %q: %v", username, err)
			}
			return
		}
	}
	t.Fatalf("disable user %q: no such user", username)
}

// ErrorCode returns the API error code of err, or "" if err is not an API error.
func ErrorCode(err error) api.ErrorCode { return api.CodeOf(err) }
