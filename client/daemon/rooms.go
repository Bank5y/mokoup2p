package daemon

import (
	"net/http"
	"net/url"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

// Room management is relayed to the server as the logged-in user; the rules
// live on the server.
func (d *Daemon) roomRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+ipc.PathRooms, d.handleCreateRoom)
	mux.HandleFunc("GET "+ipc.PathRooms, d.handleMyRooms)
	mux.HandleFunc("PATCH "+ipc.PathRoom("{code}"), d.handleUpdateRoom)
	mux.HandleFunc("DELETE "+ipc.PathRoom("{code}"), d.handleDeleteRoom)
}

func serverRoomPath(code string) string { return api.PathRoom(url.PathEscape(code)) }

func (d *Daemon) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	var req api.CreateRoomRequest
	if !decode(w, r, &req) {
		return
	}
	var room api.Room
	if err := d.serverCall(r.Context(), http.MethodPost, api.PathRooms, req, &room); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, room)
}

func (d *Daemon) handleMyRooms(w http.ResponseWriter, r *http.Request) {
	var list api.RoomList
	if err := d.serverCall(r.Context(), http.MethodGet, api.PathRooms, nil, &list); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (d *Daemon) handleUpdateRoom(w http.ResponseWriter, r *http.Request) {
	var req api.UpdateRoomRequest
	if !decode(w, r, &req) {
		return
	}
	var room api.Room
	if err := d.serverCall(r.Context(), http.MethodPatch, serverRoomPath(r.PathValue("code")), req, &room); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, room)
}

func (d *Daemon) handleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	if err := d.serverCall(r.Context(), http.MethodDelete, serverRoomPath(r.PathValue("code")), nil, nil); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
