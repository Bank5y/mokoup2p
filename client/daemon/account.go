package daemon

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

func (d *Daemon) accountRoutes(mux *http.ServeMux) {
	mux.HandleFunc("PUT "+ipc.PathPassword, d.handleChangePassword)
	mux.HandleFunc("PUT "+ipc.PathDeviceName, d.handleSetDeviceName)
	mux.HandleFunc("POST "+ipc.PathLogout, d.handleLogout)
	mux.HandleFunc(ipc.PathAdmin+"/", d.handleAdminRelay)
}

// logOutLocally forgets the session; d.mu must be held. A room the device is
// in is left in the background (stopping the engine) without telling the
// server, which no longer accepts this device's tokens; callers that still
// can should leave the room first.
func (d *Daemon) logOutLocally() error {
	if d.room != nil {
		d.leaveRoomInBackground(false)
	}
	d.saved.RefreshToken = ""
	d.loggedIn = false
	d.user = api.User{}
	d.accessToken = ""
	d.serverDeviceName = ""
	d.changed()
	d.wakeRealtime()
	return saveState(d.cfg.StateDir, d.saved)
}

// syncDeviceName tells the server the locally chosen device name if it
// differs, e.g. after renaming the device while logged out. Best effort.
func (d *Daemon) syncDeviceName(ctx context.Context) {
	d.mu.Lock()
	name, current, loggedIn := d.saved.DeviceName, d.serverDeviceName, d.loggedIn
	d.mu.Unlock()
	if !loggedIn || name == current {
		return
	}
	var dev api.Device
	if err := d.serverCall(ctx, http.MethodPut, api.PathMeDevice, api.RenameDeviceRequest{Name: name}, &dev); err != nil {
		log.Printf("sync device name: %v", err)
		return
	}
	d.mu.Lock()
	d.serverDeviceName = dev.Name
	d.mu.Unlock()
}

func (d *Daemon) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req api.ChangePasswordRequest
	if !decode(w, r, &req) {
		return
	}
	if err := d.serverCall(r.Context(), http.MethodPut, api.PathMePassword, req, nil); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *Daemon) handleSetDeviceName(w http.ResponseWriter, r *http.Request) {
	var req ipc.SetDeviceNameRequest
	if !decode(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, &api.Error{Code: api.ErrBadRequest, Message: "device name must not be empty"})
		return
	}
	d.mu.Lock()
	loggedIn := d.loggedIn
	d.mu.Unlock()
	if loggedIn {
		var dev api.Device
		err := d.serverCall(r.Context(), http.MethodPut, api.PathMeDevice, api.RenameDeviceRequest{Name: name}, &dev)
		if err != nil {
			writeError(w, err)
			return
		}
		name = dev.Name
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.saved.DeviceName = name
	d.changed()
	if d.loggedIn {
		d.serverDeviceName = name
	}
	if err := saveState(d.cfg.StateDir, d.saved); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d.status())
}

// handleLogout revokes the session on the server and forgets it locally. The
// local logout happens even if the server cannot be reached.
func (d *Daemon) handleLogout(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	d.leaveRoom(ctx, true)
	if err := d.serverCall(ctx, http.MethodPost, api.PathLogout, nil, nil); err != nil && api.CodeOf(err) != ipc.ErrNotLoggedIn {
		log.Printf("logout on server: %v", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.logOutLocally(); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d.status())
}

// handleAdminRelay forwards administrator requests (users, rooms, settings)
// to the server unchanged; the server enforces every rule.
func (d *Daemon) handleAdminRelay(w http.ResponseWriter, r *http.Request) {
	path := "/api" + r.URL.EscapedPath()
	var in any
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, &api.Error{Code: api.ErrBadRequest, Message: "cannot read body"})
		return
	}
	if len(strings.TrimSpace(string(body))) > 0 {
		if !json.Valid(body) {
			writeError(w, &api.Error{Code: api.ErrBadRequest, Message: "invalid JSON body"})
			return
		}
		in = json.RawMessage(body)
	}
	var out json.RawMessage
	if err := d.serverCall(r.Context(), r.Method, path, in, &out); err != nil {
		writeError(w, err)
		return
	}
	if out == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(out)
}
