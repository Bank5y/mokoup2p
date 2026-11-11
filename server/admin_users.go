package server

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Bank5y/mokoup2p/api"
)

var (
	errForbidden    = &api.Error{Code: api.ErrForbidden, Message: "administrator only"}
	errUserNotFound = &api.Error{Code: api.ErrUserNotFound, Message: "user not found"}
	errLastAdmin    = &api.Error{Code: api.ErrLastAdmin, Message: "the last administrator cannot be removed, demoted or disabled"}
)

func (s *Server) adminUserRoutes() {
	user := api.PathAdminUsers + "/{id}"
	s.mux.HandleFunc("GET "+api.PathAdminUsers, s.administrator(s.handleListUsers))
	s.mux.HandleFunc("POST "+api.PathAdminUsers, s.administrator(s.handleCreateUser))
	s.mux.HandleFunc("PUT "+user+api.AdminUserPassword, s.administrator(s.handleResetPassword))
	s.mux.HandleFunc("PUT "+user+api.AdminUserDisabled, s.administrator(s.handleSetDisabled))
	s.mux.HandleFunc("PUT "+user+api.AdminUserAdmin, s.administrator(s.handleSetAdmin))
	s.mux.HandleFunc("DELETE "+user, s.administrator(s.handleDeleteUser))
}

// administrator wraps a handler that only administrators may call.
func (s *Server) administrator(h func(http.ResponseWriter, *http.Request, session)) http.HandlerFunc {
	return s.authenticated(func(w http.ResponseWriter, r *http.Request, se session) {
		if !se.user.isAdmin {
			writeError(w, errForbidden)
			return
		}
		h(w, r, se)
	})
}

func (s *Server) handleListUsers(w http.ResponseWriter, _ *http.Request, _ session) {
	rows, err := s.db.Query(`SELECT id, username, is_admin, disabled FROM users ORDER BY id`)
	if err != nil {
		writeError(w, err)
		return
	}
	defer rows.Close()
	users := []api.AdminUser{}
	for rows.Next() {
		var u api.AdminUser
		if err := rows.Scan(&u.ID, &u.Username, &u.IsAdmin, &u.Disabled); err != nil {
			writeError(w, err)
			return
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request, _ session) {
	var req api.CreateUserRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		writeError(w, &api.Error{Code: api.ErrBadRequest, Message: "username must not be empty"})
		return
	}
	hash, err := hashPassword(req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	u := api.AdminUser{Username: username}
	err = s.db.QueryRow(`
		INSERT INTO users (username, password_hash) VALUES (?, ?)
		ON CONFLICT (username) DO NOTHING
		RETURNING id`, username, hash).Scan(&u.ID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, &api.Error{Code: api.ErrUsernameTaken, Message: "username already exists"})
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request, _ session) {
	id, ok := userID(w, r)
	if !ok {
		return
	}
	var req api.ResetPasswordRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	hash, err := hashPassword(req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	s.respondUser(w, s.updateUser(id, nil, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, id), id)
}

// handleSetDisabled disables or enables a user. Disabling revokes all of the
// user's tokens at once, so every device is logged out.
func (s *Server) handleSetDisabled(w http.ResponseWriter, r *http.Request, _ session) {
	id, ok := userID(w, r)
	if !ok {
		return
	}
	var req api.SetDisabledRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	err := s.updateUser(id, func(tx *sql.Tx) error {
		if !req.Disabled {
			return nil
		}
		if err := ensureOtherAdmin(tx, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM tokens WHERE device_pk IN (SELECT id FROM devices WHERE user_id = ?)`, id); err != nil {
			return err
		}
		// The user's devices leave their rooms. Telling them and the other
		// members is the realtime ticket's job (#7).
		_, err := tx.Exec(`DELETE FROM memberships WHERE device_pk IN (SELECT id FROM devices WHERE user_id = ?)`, id)
		return err
	}, `UPDATE users SET disabled = ? WHERE id = ?`, req.Disabled, id)
	s.respondUser(w, err, id)
}

func (s *Server) handleSetAdmin(w http.ResponseWriter, r *http.Request, _ session) {
	id, ok := userID(w, r)
	if !ok {
		return
	}
	var req api.SetAdminRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	err := s.updateUser(id, func(tx *sql.Tx) error {
		if req.IsAdmin {
			return nil
		}
		return ensureOtherAdmin(tx, id)
	}, `UPDATE users SET is_admin = ? WHERE id = ?`, req.IsAdmin, id)
	s.respondUser(w, err, id)
}

// handleDeleteUser deletes a user together with its devices and tokens.
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request, _ session) {
	id, ok := userID(w, r)
	if !ok {
		return
	}
	err := s.updateUser(id, func(tx *sql.Tx) error { return ensureOtherAdmin(tx, id) },
		`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func userID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, errUserNotFound)
		return 0, false
	}
	return id, true
}

// updateUser runs check (if any) and then the statement in one transaction,
// failing with errUserNotFound if the user does not exist.
func (s *Server) updateUser(id int64, check func(*sql.Tx) error, stmt string, args ...any) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM users WHERE id = ?)`, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errUserNotFound
	}
	if check != nil {
		if err := check(tx); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(stmt, args...); err != nil {
		return err
	}
	return tx.Commit()
}

// ensureOtherAdmin fails unless an enabled administrator other than id exists,
// so the system is never left without one.
func ensureOtherAdmin(tx *sql.Tx, id int64) error {
	var n int
	err := tx.QueryRow(`SELECT COUNT(*) FROM users WHERE is_admin = 1 AND disabled = 0 AND id != ?`, id).Scan(&n)
	if err != nil {
		return err
	}
	if n == 0 {
		var isEnabledAdmin bool
		err := tx.QueryRow(`SELECT is_admin = 1 AND disabled = 0 FROM users WHERE id = ?`, id).Scan(&isEnabledAdmin)
		if err != nil {
			return err
		}
		if isEnabledAdmin {
			return errLastAdmin
		}
	}
	return nil
}

// respondUser answers with the user's current record, or with err.
func (s *Server) respondUser(w http.ResponseWriter, err error, id int64) {
	if err != nil {
		writeError(w, err)
		return
	}
	var u api.AdminUser
	err = s.db.QueryRow(`SELECT id, username, is_admin, disabled FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &u.IsAdmin, &u.Disabled)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}
