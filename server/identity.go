package server

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/Bank5y/mokoup2p/api"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour

	tokenAccess  = "access"
	tokenRefresh = "refresh"
)

// ensureAdministrator creates the first administrator if none exists.
func (s *Server) ensureAdministrator(username, password string) error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE is_admin = 1`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if username == "" || password == "" {
		return errNoAdminCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO users (username, password_hash, is_admin) VALUES (?, ?, 1)`, username, string(hash))
	if err != nil {
		return fmt.Errorf("create administrator %q: %w", username, err)
	}
	return nil
}

type userRow struct {
	id           int64
	username     string
	passwordHash string
	isAdmin      bool
	disabled     bool
}

func (u userRow) api() api.User {
	return api.User{ID: u.id, Username: u.username, IsAdmin: u.isAdmin}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req api.LoginRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	sess, err := s.login(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) login(req api.LoginRequest) (api.Session, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.DeviceName = strings.TrimSpace(req.DeviceName)
	if req.Username == "" || req.Password == "" || req.DeviceID == "" {
		return api.Session{}, &api.Error{Code: api.ErrBadRequest, Message: "username, password and deviceId are required"}
	}
	if req.DeviceName == "" {
		req.DeviceName = req.DeviceID
	}

	var u userRow
	err := s.db.QueryRow(`SELECT id, username, password_hash, is_admin, disabled FROM users WHERE username = ?`, req.Username).
		Scan(&u.id, &u.username, &u.passwordHash, &u.isAdmin, &u.disabled)
	if errors.Is(err, sql.ErrNoRows) {
		return api.Session{}, errInvalidCredentials
	}
	if err != nil {
		return api.Session{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.passwordHash), []byte(req.Password)) != nil {
		return api.Session{}, errInvalidCredentials
	}
	if u.disabled {
		return api.Session{}, errAccountDisabled
	}

	var devicePK int64
	var deviceName string
	err = s.db.QueryRow(`
		INSERT INTO devices (user_id, device_id, name) VALUES (?, ?, ?)
		ON CONFLICT (user_id, device_id) DO UPDATE SET device_id = excluded.device_id
		RETURNING id, name`, u.id, req.DeviceID, req.DeviceName).Scan(&devicePK, &deviceName)
	if err != nil {
		return api.Session{}, err
	}
	return s.issueSession(u, devicePK, api.Device{ID: req.DeviceID, Name: deviceName})
}

var (
	errInvalidCredentials = &api.Error{Code: api.ErrInvalidCredentials, Message: "wrong username or password"}
	errAccountDisabled    = &api.Error{Code: api.ErrAccountDisabled, Message: "account is disabled"}
)

func (s *Server) issueSession(u userRow, devicePK int64, dev api.Device) (api.Session, error) {
	now := s.clock.Now()
	access, err := s.storeToken(tokenAccess, devicePK, now.Add(accessTokenTTL))
	if err != nil {
		return api.Session{}, err
	}
	refresh, err := s.storeToken(tokenRefresh, devicePK, now.Add(refreshTokenTTL))
	if err != nil {
		return api.Session{}, err
	}
	return api.Session{
		AccessToken:          access,
		AccessTokenExpiresAt: now.Add(accessTokenTTL).UTC(),
		RefreshToken:         refresh,
		User:                 u.api(),
		Device:               dev,
	}, nil
}

// storeToken creates an opaque random token; only its hash is stored.
func (s *Server) storeToken(kind string, devicePK int64, expiresAt time.Time) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	_, err := s.db.Exec(`INSERT INTO tokens (token_hash, kind, device_pk, expires_at) VALUES (?, ?, ?, ?)`,
		hashToken(token), kind, devicePK, expiresAt.UnixMilli())
	return token, err
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

var errTokenInvalid = &api.Error{Code: api.ErrTokenInvalid, Message: "token is invalid or expired"}

// session is the authenticated identity behind a token.
type session struct {
	user     userRow
	devicePK int64
	device   api.Device
}

// lookupToken resolves a token of the given kind that has not expired.
func (s *Server) lookupToken(token, kind string) (session, error) {
	if token == "" {
		return session{}, errTokenInvalid
	}
	var se session
	var expiresAt int64
	err := s.db.QueryRow(`
		SELECT t.expires_at, d.id, d.device_id, d.name,
		       u.id, u.username, u.is_admin, u.disabled
		FROM tokens t
		JOIN devices d ON d.id = t.device_pk
		JOIN users u ON u.id = d.user_id
		WHERE t.token_hash = ? AND t.kind = ?`, hashToken(token), kind).
		Scan(&expiresAt, &se.devicePK, &se.device.ID, &se.device.Name,
			&se.user.id, &se.user.username, &se.user.isAdmin, &se.user.disabled)
	if errors.Is(err, sql.ErrNoRows) {
		return session{}, errTokenInvalid
	}
	if err != nil {
		return session{}, err
	}
	if !s.clock.Now().Before(time.UnixMilli(expiresAt)) {
		return session{}, errTokenInvalid
	}
	if se.user.disabled {
		return session{}, errAccountDisabled
	}
	return se, nil
}

// authenticated wraps a handler that requires a valid access token.
func (s *Server) authenticated(h func(http.ResponseWriter, *http.Request, session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		se, err := s.lookupToken(token, tokenAccess)
		if err != nil {
			writeError(w, err)
			return
		}
		h(w, r, se)
	}
}

func (s *Server) handleMe(w http.ResponseWriter, _ *http.Request, se session) {
	writeJSON(w, http.StatusOK, api.Me{User: se.user.api(), Device: se.device})
}

// handleRefresh exchanges a refresh token for a new session. The refresh
// token is single-use: it is deleted when exchanged.
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req api.RefreshRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	se, err := s.lookupToken(req.RefreshToken, tokenRefresh)
	if err != nil {
		writeError(w, err)
		return
	}
	res, err := s.db.Exec(`DELETE FROM tokens WHERE token_hash = ?`, hashToken(req.RefreshToken))
	if err != nil {
		writeError(w, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeError(w, errTokenInvalid) // lost a race with a concurrent refresh
		return
	}
	sess, err := s.issueSession(se.user, se.devicePK, se.device)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}
