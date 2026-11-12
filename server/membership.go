package server

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/netip"

	"golang.org/x/crypto/bcrypt"

	"github.com/Bank5y/mokoup2p/api"
)

// A membership is a device being in a room right now. It is temporary: the
// table is emptied whenever the server starts. An IP allocation is the
// virtual IP a device has in a room; it outlives memberships so the device
// gets the same address when it comes back, and goes away with the room.
const membershipSchema = `
CREATE TABLE IF NOT EXISTS memberships (
	device_pk INTEGER PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
	room_id   INTEGER NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
	joined_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS ip_allocations (
	room_id   INTEGER NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
	device_pk INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
	host      INTEGER NOT NULL,
	PRIMARY KEY (room_id, device_pk),
	UNIQUE (room_id, host)
);
`

// Hosts of a room's /24 that can be handed out: .1 to .254.
const (
	firstHost = 1
	lastHost  = 254
)

func init() {
	statusByCode[api.ErrRoomPasswordWrong] = http.StatusForbidden
	statusByCode[api.ErrRoomFull] = http.StatusConflict
	statusByCode[api.ErrInOtherRoom] = http.StatusConflict
}

func (s *Server) membershipRoutes() {
	s.mux.HandleFunc("POST "+api.PathRoomMembers("{code}"), s.authenticated(s.handleJoin))
	s.mux.HandleFunc("GET "+api.PathRoomMembers("{code}"), s.authenticated(s.handleListMembers))
	s.mux.HandleFunc("GET "+api.PathMembership, s.authenticated(s.handleMyMembership))
	s.mux.HandleFunc("DELETE "+api.PathMembership, s.authenticated(s.handleLeave))
}

// endAllMemberships runs at startup: memberships never survive a restart.
func (s *Server) endAllMemberships() error {
	_, err := s.db.Exec(`DELETE FROM memberships`)
	return err
}

var (
	errRoomPasswordWrong = &api.Error{Code: api.ErrRoomPasswordWrong, Message: "wrong room password"}
	errInOtherRoom       = &api.Error{Code: api.ErrInOtherRoom, Message: "this device is already in another room; leave it first"}
)

func errRoomFull(capacity int) error {
	return &api.Error{Code: api.ErrRoomFull, Message: fmt.Sprintf("the room is full (at most %d devices)", capacity)}
}

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request, se session) {
	var req api.JoinRoomRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	res, err := s.join(r.PathValue("code"), se, req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// join admits the session's device to a room. The checks run in the order
// the specification lists them; a disabled account was already rejected
// when the access token was checked.
func (s *Server) join(code string, se session, password string) (api.JoinResult, error) {
	room, err := s.lookupRoom(code)
	if err != nil {
		return api.JoinResult{}, err
	}
	if room.ownerID != se.user.id && room.passwordHash != "" &&
		bcrypt.CompareHashAndPassword([]byte(room.passwordHash), []byte(password)) != nil {
		return api.JoinResult{}, errRoomPasswordWrong
	}

	tx, err := s.db.Begin()
	if err != nil {
		return api.JoinResult{}, err
	}
	defer tx.Rollback()

	// The key is read inside the transaction: the room may have been
	// deleted or its key rotated since the lookup.
	var key string
	err = tx.QueryRow(`SELECT network_key FROM rooms WHERE id = ?`, room.id).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return api.JoinResult{}, errRoomNotFound
	}
	if err != nil {
		return api.JoinResult{}, err
	}

	var currentRoom int64
	err = tx.QueryRow(`SELECT room_id FROM memberships WHERE device_pk = ?`, se.devicePK).Scan(&currentRoom)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		currentRoom = 0
	case err != nil:
		return api.JoinResult{}, err
	}
	// Joining the room the device is already in just hands out the
	// configuration again.
	joined := currentRoom != room.id
	if joined {
		capacity, err := getSetting(tx, settingRoomCapacity)
		if err != nil {
			return api.JoinResult{}, err
		}
		var members int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM memberships WHERE room_id = ?`, room.id).Scan(&members); err != nil {
			return api.JoinResult{}, err
		}
		if members >= capacity {
			return api.JoinResult{}, errRoomFull(capacity)
		}
		if currentRoom != 0 {
			return api.JoinResult{}, errInOtherRoom
		}
		if _, err := tx.Exec(`INSERT INTO memberships (device_pk, room_id, joined_at) VALUES (?, ?, ?)`,
			se.devicePK, room.id, s.clock.Now().UnixMilli()); err != nil {
			return api.JoinResult{}, err
		}
	}

	host, err := allocateHost(tx, room.id, se.devicePK)
	if err != nil {
		return api.JoinResult{}, err
	}
	ip, err := hostAddr(room.subnet, host)
	if err != nil {
		return api.JoinResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return api.JoinResult{}, err
	}
	s.alive(se.devicePK)
	if joined {
		s.announceJoin(se.devicePK, room)
	}
	return api.JoinResult{
		Room:    room.api(),
		Network: s.networkConfig(room, key, ip.String()),
	}, nil
}

// allocateHost returns the device's host number in the room, allocating the
// lowest free one the first time the device joins.
func allocateHost(tx *sql.Tx, roomID, devicePK int64) (int, error) {
	var host int
	err := tx.QueryRow(`SELECT host FROM ip_allocations WHERE room_id = ? AND device_pk = ?`, roomID, devicePK).Scan(&host)
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		return host, err
	}
	rows, err := tx.Query(`SELECT host FROM ip_allocations WHERE room_id = ? ORDER BY host`, roomID)
	if err != nil {
		return 0, err
	}
	host = firstHost
	for rows.Next() {
		var taken int
		if err := rows.Scan(&taken); err != nil {
			rows.Close()
			return 0, err
		}
		if taken == host {
			host++
		} else if taken > host {
			break
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if host > lastHost {
		// Every address has been handed to some device at some time.
		return 0, &api.Error{Code: api.ErrRoomFull, Message: "the room has no free virtual IP addresses"}
	}
	_, err = tx.Exec(`INSERT INTO ip_allocations (room_id, device_pk, host) VALUES (?, ?, ?)`, roomID, devicePK, host)
	return host, err
}

func hostAddr(subnet string, host int) (netip.Addr, error) {
	p, err := netip.ParsePrefix(subnet)
	if err != nil {
		return netip.Addr{}, err
	}
	a := p.Addr().As4()
	a[3] = byte(host)
	return netip.AddrFrom4(a), nil
}

// networkName derives the EasyTier network name from the room's internal
// identity. Both the row id and the code go in, so a room created after
// another was deleted never shares its network name.
func networkName(r roomRow) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("mokoup2p-room:%d:%s", r.id, r.code)))
	return "mokoup2p-" + hex.EncodeToString(sum[:8])
}

// announceJoin tells the room's other members that a device has joined.
func (s *Server) announceJoin(devicePK int64, room roomRow) {
	ms, ok, err := membershipOf(s.db, devicePK)
	if err != nil || !ok || ms.roomID != room.id {
		return // gone again already
	}
	s.pushToRoom(room.id, devicePK, api.EventMemberJoined, api.MemberJoined{Room: ms.roomCode, Member: ms.member})
}

func (s *Server) handleLeave(w http.ResponseWriter, _ *http.Request, se session) {
	if err := s.endMembership(se.devicePK, api.LeftByDevice); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
