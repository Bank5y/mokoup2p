package server

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/Bank5y/mokoup2p/api"
)

func init() {
	statusByCode[api.ErrNotRoomMember] = http.StatusForbidden
}

var errNotRoomMember = &api.Error{Code: api.ErrNotRoomMember, Message: "only the room's members, its owner and administrators can see its members"}

// querier is what *sql.DB and *sql.Tx have in common.
type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

const memberColumns = `d.id, u.username, d.name, r.subnet, a.host
	FROM memberships m
	JOIN devices d ON d.id = m.device_pk
	JOIN users u ON u.id = d.user_id
	JOIN rooms r ON r.id = m.room_id
	JOIN ip_allocations a ON a.room_id = m.room_id AND a.device_pk = m.device_pk`

func scanMember(scan func(...any) error) (api.Member, error) {
	var m api.Member
	var subnet string
	var host int
	if err := scan(&m.ID, &m.Username, &m.DeviceName, &subnet, &host); err != nil {
		return m, err
	}
	ip, err := hostAddr(subnet, host)
	if err != nil {
		return m, err
	}
	m.VirtualIP = ip.String()
	return m, nil
}

// roomMembers lists the devices in a room, ordered by virtual IP.
func roomMembers(q querier, roomID int64) ([]api.Member, error) {
	rows, err := q.Query(`SELECT `+memberColumns+` WHERE m.room_id = ? ORDER BY a.host`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []api.Member{}
	for rows.Next() {
		m, err := scanMember(rows.Scan)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// membership is a device in a room.
type membership struct {
	roomID   int64
	roomCode string // as displayed
	member   api.Member
}

// membershipOf returns the device's membership; ok is false when the device
// is in no room.
func membershipOf(q querier, devicePK int64) (ms membership, ok bool, err error) {
	row := q.QueryRow(`SELECT m.room_id, r.code, `+memberColumns+` WHERE m.device_pk = ?`, devicePK)
	ms.member, err = scanMember(func(dest ...any) error {
		return row.Scan(append([]any{&ms.roomID, &ms.roomCode}, dest...)...)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return membership{}, false, nil
	}
	if err != nil {
		return membership{}, false, err
	}
	ms.roomCode = displayRoomCode(ms.roomCode)
	return ms, true, nil
}

// endMemberships ends the memberships of the given devices inside tx and
// returns them, so that announceDepartures can tell the rooms once tx is
// committed. Devices in no room are skipped.
func endMemberships(tx *sql.Tx, devicePKs ...int64) ([]membership, error) {
	var ended []membership
	for _, pk := range devicePKs {
		ms, ok, err := membershipOf(tx, pk)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM memberships WHERE device_pk = ?`, pk); err != nil {
			return nil, err
		}
		ended = append(ended, ms)
	}
	return ended, nil
}

// announceDepartures tells each ended membership's room that the device
// has left.
func (s *Server) announceDepartures(ended []membership, reason api.LeaveReason) {
	for _, ms := range ended {
		s.pushToRoom(ms.roomID, ms.member.ID, api.EventMemberLeft,
			api.MemberLeft{Room: ms.roomCode, Member: ms.member, Reason: reason})
	}
}

// endMembership ends one device's membership, if it has one, and tells the
// room's remaining members.
func (s *Server) endMembership(devicePK int64, reason api.LeaveReason) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ended, err := endMemberships(tx, devicePK)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.announceDepartures(ended, reason)
	return nil
}

func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request, se session) {
	room, err := s.lookupRoom(r.PathValue("code"))
	if err != nil {
		writeError(w, err)
		return
	}
	if room.ownerID != se.user.id && !se.user.isAdmin {
		var inRoom bool
		err := s.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM memberships WHERE device_pk = ? AND room_id = ?)`, se.devicePK, room.id).Scan(&inRoom)
		if err != nil {
			writeError(w, err)
			return
		}
		if !inRoom {
			writeError(w, errNotRoomMember)
			return
		}
	}
	members, err := roomMembers(s.db, room.id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, api.MemberList{Members: members})
}
