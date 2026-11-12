package server

import (
	"log"
	"net/http"
	"strconv"

	"github.com/Bank5y/mokoup2p/api"
)

func init() {
	statusByCode[api.ErrMemberNotFound] = http.StatusNotFound
}

var errMemberNotFound = &api.Error{Code: api.ErrMemberNotFound, Message: "that device is not a member of this room"}

func (s *Server) kickRoutes() {
	s.mux.HandleFunc("DELETE "+api.PathRoomMember("{code}", "{id}"), s.authenticated(s.handleKick))
}

// handleKick ends a member's membership on the room owner's request. The
// network key stays: the kicked device may come back with the room password.
func (s *Server) handleKick(w http.ResponseWriter, r *http.Request, se session) {
	room, err := s.ownedRoom(r.PathValue("code"), se)
	if err != nil {
		writeError(w, err)
		return
	}
	devicePK, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, errMemberNotFound)
		return
	}
	if err := s.kick(room, devicePK); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) kick(room roomRow, devicePK int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ms, ok, err := membershipOf(tx, devicePK)
	if err != nil {
		return err
	}
	if !ok || ms.roomID != room.id {
		return errMemberNotFound
	}
	ended, err := endMemberships(tx, devicePK)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.push(devicePK, api.EventKicked, api.RoomEvent{Room: displayRoomCode(room.code)})
	s.announceDepartures(ended, api.LeftByKick)
	return nil
}

// pushNetworkChanged hands every member of the room its configuration with
// the room's new network key, so members reconnect instead of being dropped.
func (s *Server) pushNetworkChanged(room roomRow, key string) {
	members, err := roomMembers(s.db, room.id)
	if err != nil {
		log.Printf("realtime: list members of room %d: %v", room.id, err)
		return
	}
	for _, m := range members {
		s.push(m.ID, api.EventNetworkChanged, api.NetworkChanged{
			Room:    displayRoomCode(room.code),
			Network: s.networkConfig(room, key, m.VirtualIP),
		})
	}
}

// networkConfig is what a device in the room needs to start its engine.
func (s *Server) networkConfig(room roomRow, key, virtualIP string) api.NetworkConfig {
	return api.NetworkConfig{
		NetworkName: networkName(room),
		NetworkKey:  key,
		VirtualIP:   virtualIP,
		Subnet:      room.subnet,
		RelayPeers:  append([]string{}, s.relayPeers...),
	}
}

// deleteRoom deletes a room, ending its memberships and IP allocations, and
// tells every member that it is gone.
func (s *Server) deleteRoom(room roomRow) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	members, err := roomMembers(tx, room.id)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM rooms WHERE id = ?`, room.id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, m := range members {
		s.push(m.ID, api.EventRoomDeleted, api.RoomEvent{Room: displayRoomCode(room.code)})
	}
	return nil
}
