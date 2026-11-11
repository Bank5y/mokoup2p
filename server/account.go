package server

import (
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/Bank5y/mokoup2p/api"
)

func init() {
	statusByCode[api.ErrForbidden] = http.StatusForbidden
	statusByCode[api.ErrUserNotFound] = http.StatusNotFound
	statusByCode[api.ErrUsernameTaken] = http.StatusConflict
	statusByCode[api.ErrLastAdmin] = http.StatusConflict
}

// accountRoutes registers a user's own account operations and the
// administrator's user management.
func (s *Server) accountRoutes() {
	s.mux.HandleFunc("PUT "+api.PathMePassword, s.authenticated(s.handleChangePassword))
	s.mux.HandleFunc("PUT "+api.PathMeDevice, s.authenticated(s.handleRenameDevice))
	s.mux.HandleFunc("POST "+api.PathLogout, s.authenticated(s.handleLogout))
	s.adminUserRoutes()
}

func (s *Server) handleRenameDevice(w http.ResponseWriter, r *http.Request, se session) {
	var req api.RenameDeviceRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, &api.Error{Code: api.ErrBadRequest, Message: "device name must not be empty"})
		return
	}
	if _, err := s.db.Exec(`UPDATE devices SET name = ? WHERE id = ?`, name, se.devicePK); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, api.Device{ID: se.device.ID, Name: name})
}

// handleLogout revokes every token of the calling device; the user's other
// devices stay logged in.
func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request, se session) {
	if _, err := s.db.Exec(`DELETE FROM tokens WHERE device_pk = ?`, se.devicePK); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func hashPassword(password string) (string, error) {
	if password == "" {
		return "", &api.Error{Code: api.ErrBadRequest, Message: "password must not be empty"}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request, se session) {
	var req api.ChangePasswordRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	var current string
	if err := s.db.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, se.user.id).Scan(&current); err != nil {
		writeError(w, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(current), []byte(req.OldPassword)) != nil {
		writeError(w, errInvalidCredentials)
		return
	}
	hash, err := hashPassword(req.NewPassword)
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, se.user.id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
