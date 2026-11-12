package server

import (
	"database/sql"
	"net/http"

	"github.com/Bank5y/mokoup2p/api"
)

func (s *Server) adminRoomRoutes() {
	s.mux.HandleFunc("GET "+api.PathAdminRooms, s.administrator(s.handleListAllRooms))
	s.mux.HandleFunc("DELETE "+api.PathAdminRoom("{code}"), s.administrator(s.handleAdminDeleteRoom))
	s.mux.HandleFunc("GET "+api.PathAdminSettings, s.administrator(s.handleGetSettings))
	s.mux.HandleFunc("PUT "+api.PathAdminSettings, s.administrator(s.handlePutSettings))
}

func (s *Server) handleListAllRooms(w http.ResponseWriter, _ *http.Request, _ session) {
	rows, err := s.db.Query(`
		SELECT r.code, r.name, u.username, r.password_hash != '', r.subnet,
		       (SELECT COUNT(*) FROM memberships m WHERE m.room_id = r.id)
		FROM rooms r JOIN users u ON u.id = r.owner_id
		ORDER BY r.id`)
	if err != nil {
		writeError(w, err)
		return
	}
	defer rows.Close()
	list := api.AdminRoomList{Rooms: []api.AdminRoom{}}
	for rows.Next() {
		var r api.AdminRoom
		if err := rows.Scan(&r.Code, &r.Name, &r.Owner, &r.HasPassword, &r.Subnet, &r.OnlineMembers); err != nil {
			writeError(w, err)
			return
		}
		r.Code = displayRoomCode(r.Code)
		list.Rooms = append(list.Rooms, r)
	}
	if err := rows.Err(); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleAdminDeleteRoom(w http.ResponseWriter, r *http.Request, _ session) {
	room, err := s.lookupRoom(r.PathValue("code"))
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.deleteRoom(room); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetSettings(w http.ResponseWriter, _ *http.Request, _ session) {
	st, err := s.settings(s.db)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) settings(q queryer) (api.Settings, error) {
	var st api.Settings
	var err error
	if st.RoomsPerUser, err = getSetting(q, settingRoomsPerUser); err != nil {
		return st, err
	}
	st.RoomCapacity, err = getSetting(q, settingRoomCapacity)
	return st, err
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request, _ session) {
	var req api.Settings
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if req.RoomsPerUser < 1 || req.RoomsPerUser > 1000 {
		writeError(w, &api.Error{Code: api.ErrBadRequest, Message: "roomsPerUser must be between 1 and 1000"})
		return
	}
	if req.RoomCapacity < 1 || req.RoomCapacity > 254 {
		writeError(w, &api.Error{Code: api.ErrBadRequest, Message: "roomCapacity must be between 1 and 254"})
		return
	}
	if err := s.setSetting(settingRoomsPerUser, req.RoomsPerUser); err != nil {
		writeError(w, err)
		return
	}
	if err := s.setSetting(settingRoomCapacity, req.RoomCapacity); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, req)
}

// ownedRoomMembers lists the rooms a user owns with their current members.
func ownedRoomMembers(tx *sql.Tx, userID int64) (map[string][]api.Member, error) {
	rows, err := tx.Query(`SELECT id, code FROM rooms WHERE owner_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	type room struct {
		id   int64
		code string
	}
	var rooms []room
	for rows.Next() {
		var r room
		if err := rows.Scan(&r.id, &r.code); err != nil {
			rows.Close()
			return nil, err
		}
		rooms = append(rooms, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := map[string][]api.Member{}
	for _, r := range rooms {
		members, err := roomMembers(tx, r.id)
		if err != nil {
			return nil, err
		}
		out[displayRoomCode(r.code)] = members
	}
	return out, nil
}
