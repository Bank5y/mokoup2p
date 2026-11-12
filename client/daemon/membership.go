package daemon

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"time"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/engine"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

// Room lifecycle: idle → joining → in room → idle. Transitions are
// serialized by d.roomMu; the state itself (d.joining, d.room, d.network) is
// guarded by d.mu so status stays responsive while the engine works. Every
// way out of "in room" goes through leaveRoom, which stops the engine.

func (d *Daemon) membershipRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+ipc.PathJoin, d.handleJoin)
	mux.HandleFunc("POST "+ipc.PathLeave, d.handleLeave)
	mux.HandleFunc("POST "+ipc.PathKick, d.handleKick)
}

var errInRoom = &api.Error{Code: ipc.ErrInRoom, Message: "this device is in a room; leave it first"}

func (d *Daemon) handleJoin(w http.ResponseWriter, r *http.Request) {
	var req ipc.JoinRequest
	if !decode(w, r, &req) {
		return
	}
	if err := d.join(r.Context(), req); err != nil {
		writeError(w, err)
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	writeJSON(w, http.StatusOK, d.status())
}

func (d *Daemon) join(ctx context.Context, req ipc.JoinRequest) error {
	d.roomMu.Lock()
	defer d.roomMu.Unlock()

	d.mu.Lock()
	switch {
	case !d.loggedIn:
		d.mu.Unlock()
		return errNotLoggedIn
	case d.room != nil:
		d.mu.Unlock()
		return errInRoom
	}
	d.joining = true
	d.notice = ""
	d.changed()
	hostname := d.saved.DeviceName
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		d.joining = false
		d.changed()
		d.mu.Unlock()
	}()

	res, err := d.joinOnServer(ctx, req)
	if err != nil {
		return err
	}
	cfg, err := engineConfig(res.Network, hostname)
	if err != nil {
		d.leaveOnServer(ctx)
		return &api.Error{Code: ipc.ErrServerBadResponse, Message: err.Error()}
	}
	if err := d.cfg.Engine.Start(ctx, cfg); err != nil {
		log.Printf("engine start: %v", err)
		d.stopEngine()
		d.leaveOnServer(ctx)
		code := ipc.ErrEngineFailed
		if errors.Is(err, engine.ErrDependencyMissing) {
			code = ipc.ErrEngineDependencyMissing
		}
		return &api.Error{Code: code, Message: err.Error()}
	}

	d.mu.Lock()
	if !d.loggedIn {
		// Logged out (session rejected) while the engine was starting.
		d.mu.Unlock()
		d.stopEngine()
		return errNotLoggedIn
	}
	d.network = res.Network
	d.room = &ipc.CurrentRoom{
		Code:      res.Room.Code,
		Name:      res.Room.Name,
		Owner:     res.Room.Owner,
		VirtualIP: res.Network.VirtualIP,
		Subnet:    res.Network.Subnet,
	}
	d.mu.Unlock()
	return nil
}

// joinOnServer asks the server to admit this device. The daemon only calls
// it while it is in no room, so if the server still counts the device in a
// room (left over from a crash or a restart while the server was
// unreachable), that membership is stale: end it and try once more.
func (d *Daemon) joinOnServer(ctx context.Context, req ipc.JoinRequest) (api.JoinResult, error) {
	var res api.JoinResult
	call := func() error {
		return d.serverCall(ctx, http.MethodPost, api.PathRoomMembers(url.PathEscape(req.Code)), api.JoinRoomRequest{Password: req.Password}, &res)
	}
	err := call()
	if api.CodeOf(err) == api.ErrInOtherRoom {
		if err = d.serverCall(ctx, http.MethodDelete, api.PathMembership, nil, nil); err == nil {
			err = call()
		}
	}
	return res, err
}

func engineConfig(n api.NetworkConfig, hostname string) (engine.Config, error) {
	ip, err := netip.ParseAddr(n.VirtualIP)
	if err != nil {
		return engine.Config{}, fmt.Errorf("invalid virtual IP %q", n.VirtualIP)
	}
	subnet, err := netip.ParsePrefix(n.Subnet)
	if err != nil || !subnet.Contains(ip) {
		return engine.Config{}, fmt.Errorf("invalid subnet %q for %s", n.Subnet, ip)
	}
	return engine.Config{
		NetworkName:   n.NetworkName,
		NetworkSecret: n.NetworkKey,
		Address:       netip.PrefixFrom(ip, subnet.Bits()),
		RelayPeers:    append([]string{}, n.RelayPeers...),
		Hostname:      hostname,
	}, nil
}

func (d *Daemon) handleLeave(w http.ResponseWriter, r *http.Request) {
	d.leaveRoom(r.Context(), true)
	d.mu.Lock()
	defer d.mu.Unlock()
	writeJSON(w, http.StatusOK, d.status())
}

// leaveRoom ends the current room, if any: the engine is stopped and the
// state returns to idle. With tellServer the server is told too; failing
// that is only logged, since the server ends memberships on its own when
// the device's heartbeat stops.
func (d *Daemon) leaveRoom(ctx context.Context, tellServer bool) {
	d.roomMu.Lock()
	defer d.roomMu.Unlock()
	d.leaveRoomLocked(ctx, tellServer)
}

// leaveRoomLocked is leaveRoom for callers holding d.roomMu.
func (d *Daemon) leaveRoomLocked(ctx context.Context, tellServer bool) {
	d.mu.Lock()
	inRoom := d.room != nil
	d.room = nil
	d.network = api.NetworkConfig{}
	if inRoom {
		d.changed()
	}
	d.mu.Unlock()
	if !inRoom {
		return
	}
	d.stopEngine()
	if tellServer {
		d.leaveOnServer(ctx)
	}
}

// leaveRoomInBackground is for code that must not wait for d.roomMu, such
// as session renewal, which may run inside a join.
func (d *Daemon) leaveRoomInBackground(tellServer bool) {
	d.background.Add(1)
	go func() {
		defer d.background.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		d.leaveRoom(ctx, tellServer)
	}()
}

func (d *Daemon) stopEngine() {
	// Stopping must not be cut short by the caller going away.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := d.cfg.Engine.Stop(ctx); err != nil {
		log.Printf("engine stop: %v", err)
	}
}

func (d *Daemon) leaveOnServer(ctx context.Context) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := d.serverCall(ctx, http.MethodDelete, api.PathMembership, nil, nil); err != nil {
		log.Printf("leave room on server: %v", err)
	}
}
