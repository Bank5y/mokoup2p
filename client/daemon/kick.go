package daemon

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

// handleKick asks the server to kick a member of the current room. The
// server decides whether this device's user owns the room.
func (d *Daemon) handleKick(w http.ResponseWriter, r *http.Request) {
	var req ipc.KickRequest
	if !decode(w, r, &req) {
		return
	}
	d.mu.Lock()
	room := d.room
	d.mu.Unlock()
	if room == nil {
		writeError(w, errNotInRoom)
		return
	}
	path := api.PathRoomMember(url.PathEscape(room.Code), strconv.FormatInt(req.MemberID, 10))
	if err := d.serverCall(r.Context(), http.MethodDelete, path, nil, nil); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
