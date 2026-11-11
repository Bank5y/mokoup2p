package server

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/Bank5y/mokoup2p/api"
)

const roomSchema = `
CREATE TABLE IF NOT EXISTS rooms (
	id            INTEGER PRIMARY KEY,
	code          TEXT NOT NULL UNIQUE,
	name          TEXT NOT NULL,
	owner_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	password_hash TEXT NOT NULL DEFAULT '',
	subnet        TEXT NOT NULL UNIQUE,
	network_key   TEXT NOT NULL
);
`

const maxRoomNameLength = 64

func init() {
	statusByCode[api.ErrRoomNotFound] = http.StatusNotFound
	statusByCode[api.ErrRoomLimitReached] = http.StatusConflict
	statusByCode[api.ErrNotRoomOwner] = http.StatusForbidden
}

func (s *Server) roomRoutes() {
	s.mux.HandleFunc("POST "+api.PathRooms, s.authenticated(s.handleCreateRoom))
	s.mux.HandleFunc("GET "+api.PathRooms, s.authenticated(s.handleListMyRooms))
	s.mux.HandleFunc("GET "+api.PathRoom("{code}"), s.authenticated(s.handleGetRoom))
	s.mux.HandleFunc("PATCH "+api.PathRoom("{code}"), s.authenticated(s.handleUpdateRoom))
	s.mux.HandleFunc("DELETE "+api.PathRoom("{code}"), s.authenticated(s.handleDeleteRoom))
}

var errNotRoomOwner = &api.Error{Code: api.ErrNotRoomOwner, Message: "only the room owner may do this"}

// ownedRoom finds a room the session's user owns.
func (s *Server) ownedRoom(input string, se session) (roomRow, error) {
	room, err := s.lookupRoom(input)
	if err != nil {
		return roomRow{}, err
	}
	if room.ownerID != se.user.id {
		return roomRow{}, errNotRoomOwner
	}
	return room, nil
}

func (s *Server) handleUpdateRoom(w http.ResponseWriter, r *http.Request, se session) {
	var req api.UpdateRoomRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	room, err := s.updateRoom(r.PathValue("code"), se, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, room.api())
}

func (s *Server) updateRoom(code string, se session, req api.UpdateRoomRequest) (roomRow, error) {
	room, err := s.ownedRoom(code, se)
	if err != nil {
		return roomRow{}, err
	}
	if req.Name != nil {
		if room.name, err = roomName(*req.Name); err != nil {
			return roomRow{}, err
		}
		if _, err := s.db.Exec(`UPDATE rooms SET name = ? WHERE id = ?`, room.name, room.id); err != nil {
			return roomRow{}, err
		}
	}
	if req.Password != nil {
		if room.passwordHash, err = hashRoomPassword(*req.Password); err != nil {
			return roomRow{}, err
		}
		// Setting, changing or removing the password rotates the network
		// key, so devices that left cannot reuse the old one.
		key, err := newNetworkKey()
		if err != nil {
			return roomRow{}, err
		}
		if _, err := s.db.Exec(`UPDATE rooms SET password_hash = ?, network_key = ? WHERE id = ?`, room.passwordHash, key, room.id); err != nil {
			return roomRow{}, err
		}
	}
	return room, nil
}

func (s *Server) handleDeleteRoom(w http.ResponseWriter, r *http.Request, se session) {
	room, err := s.ownedRoom(r.PathValue("code"), se)
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.db.Exec(`DELETE FROM rooms WHERE id = ?`, room.id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// roomRow is a stored room. The network key is deliberately not loaded: it
// only leaves the server in a network configuration for an admitted device.
type roomRow struct {
	id           int64
	code         string
	name         string
	ownerID      int64
	owner        string
	passwordHash string
	subnet       string
}

func (r roomRow) api() api.Room {
	return api.Room{
		Code:        displayRoomCode(r.code),
		Name:        r.name,
		HasPassword: r.passwordHash != "",
		Subnet:      r.subnet,
		Owner:       r.owner,
	}
}

const roomColumns = `r.id, r.code, r.name, r.owner_id, u.username, r.password_hash, r.subnet
	FROM rooms r JOIN users u ON u.id = r.owner_id`

func scanRoom(scan func(...any) error) (roomRow, error) {
	var r roomRow
	err := scan(&r.id, &r.code, &r.name, &r.ownerID, &r.owner, &r.passwordHash, &r.subnet)
	return r, err
}

var errRoomNotFound = &api.Error{Code: api.ErrRoomNotFound, Message: "room not found"}

// lookupRoom finds a room by a room code as typed by a user.
func (s *Server) lookupRoom(input string) (roomRow, error) {
	code, ok := normalizeRoomCode(input)
	if !ok {
		return roomRow{}, errRoomNotFound
	}
	r, err := scanRoom(s.db.QueryRow(`SELECT `+roomColumns+` WHERE r.code = ?`, code).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return roomRow{}, errRoomNotFound
	}
	return r, err
}

func (s *Server) handleListMyRooms(w http.ResponseWriter, _ *http.Request, se session) {
	rows, err := s.db.Query(`SELECT `+roomColumns+` WHERE r.owner_id = ? ORDER BY r.id`, se.user.id)
	if err != nil {
		writeError(w, err)
		return
	}
	defer rows.Close()
	list := api.RoomList{Rooms: []api.Room{}}
	for rows.Next() {
		r, err := scanRoom(rows.Scan)
		if err != nil {
			writeError(w, err)
			return
		}
		list.Rooms = append(list.Rooms, r.api())
	}
	if err := rows.Err(); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetRoom(w http.ResponseWriter, r *http.Request, _ session) {
	room, err := s.lookupRoom(r.PathValue("code"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, room.api())
}

func (s *Server) handleCreateRoom(w http.ResponseWriter, r *http.Request, se session) {
	var req api.CreateRoomRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	room, err := s.createRoom(se.user, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, room)
}

func (s *Server) createRoom(owner userRow, req api.CreateRoomRequest) (api.Room, error) {
	name, err := roomName(req.Name)
	if err != nil {
		return api.Room{}, err
	}
	passwordHash, err := hashRoomPassword(req.Password)
	if err != nil {
		return api.Room{}, err
	}
	key, err := newNetworkKey()
	if err != nil {
		return api.Room{}, err
	}

	// The limit check and the insert share a transaction so concurrent
	// creates cannot both slip under the limit.
	tx, err := s.db.Begin()
	if err != nil {
		return api.Room{}, err
	}
	defer tx.Rollback()
	limit, err := getSetting(tx, settingRoomsPerUser)
	if err != nil {
		return api.Room{}, err
	}
	var owned int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM rooms WHERE owner_id = ?`, owner.id).Scan(&owned); err != nil {
		return api.Room{}, err
	}
	if owned >= limit {
		return api.Room{}, &api.Error{Code: api.ErrRoomLimitReached, Message: fmt.Sprintf("a user may own at most %d rooms", limit)}
	}

	// Code and subnet are random; retry on the rare collision with an
	// existing room.
	for range 20 {
		code, err := newRoomCode()
		if err != nil {
			return api.Room{}, err
		}
		subnet, err := newRoomSubnet()
		if err != nil {
			return api.Room{}, err
		}
		_, err = tx.Exec(`INSERT INTO rooms (code, name, owner_id, password_hash, subnet, network_key) VALUES (?, ?, ?, ?, ?, ?)`,
			code, name, owner.id, passwordHash, subnet, key)
		if isUniqueViolation(err) {
			continue
		}
		if err != nil {
			return api.Room{}, err
		}
		if err := tx.Commit(); err != nil {
			return api.Room{}, err
		}
		return api.Room{
			Code:        displayRoomCode(code),
			Name:        name,
			HasPassword: passwordHash != "",
			Subnet:      subnet,
			Owner:       owner.username,
		}, nil
	}
	return api.Room{}, errors.New("could not allocate a free room code and subnet")
}

func roomName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len([]rune(name)) > maxRoomNameLength {
		return "", &api.Error{Code: api.ErrBadRequest, Message: fmt.Sprintf("room name must be 1 to %d characters", maxRoomNameLength)}
	}
	return name, nil
}

// hashRoomPassword returns "" for no password.
func hashRoomPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func newNetworkKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
