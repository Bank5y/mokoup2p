package daemon

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

// The realtime connection: while logged in, the daemon keeps the server's
// event channel (api.PathEvents) open, sends heartbeats on it and reacts to
// the events pushed. When the connection drops it reconnects; if that takes
// longer than api.HeartbeatTimeout the server has ended the membership, which
// the daemon notices after reconnecting and leaves the room locally.

// maxReconnectDelay keeps reconnection attempts well inside the heartbeat
// timeout, so a short outage never costs the room.
const maxReconnectDelay = api.HeartbeatTimeout / 3

// statusWait is how long GET status?wait= blocks when nothing changes.
const statusWait = 25 * time.Second

// changed records that the status (or the current room's members) changed
// and wakes WaitStatus callers; d.mu must be held.
func (d *Daemon) changed() {
	d.revision++
	close(d.changedCh)
	d.changedCh = make(chan struct{})
}

// wakeRealtime makes the realtime loop look at the session again: after a
// login, or to drop the connection after a logout.
func (d *Daemon) wakeRealtime() {
	if d.realtimeCancel != nil {
		d.realtimeCancel()
		d.realtimeCancel = nil
	}
	select {
	case d.realtimeWake <- struct{}{}:
	default:
	}
}

// runRealtime keeps the realtime connection up while logged in, until ctx ends.
func (d *Daemon) runRealtime(ctx context.Context) {
	delay := d.cfg.ReconnectDelay
	for ctx.Err() == nil {
		d.mu.Lock()
		loggedIn := d.loggedIn
		d.mu.Unlock()
		if loggedIn {
			if d.connectRealtime(ctx) {
				delay = d.cfg.ReconnectDelay
			} else {
				delay = min(delay*2, maxReconnectDelay)
			}
		}
		var retry <-chan time.Time
		if loggedIn {
			retry = time.After(delay)
		}
		select {
		case <-ctx.Done():
			return
		case <-d.realtimeWake:
		case <-retry:
		}
	}
}

// connectRealtime runs one connection until it drops. It reports whether
// the connection was established.
func (d *Daemon) connectRealtime(ctx context.Context) bool {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	d.mu.Lock()
	if !d.loggedIn {
		d.mu.Unlock()
		return false
	}
	d.realtimeCancel = cancel
	d.mu.Unlock()

	ws, err := d.dialEvents(ctx)
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("realtime: connect: %v", err)
		}
		return false
	}
	defer ws.Close()
	stop := context.AfterFunc(ctx, func() { ws.Close() })
	defer stop()

	d.setConnected(true)
	defer d.setConnected(false)

	// The connection may have been down for longer than the heartbeat
	// timeout; make sure the server still counts this device in its room.
	d.background.Add(1)
	go func() {
		defer d.background.Done()
		d.checkMembership(ctx)
	}()

	go d.sendHeartbeats(ctx, ws, cancel)
	for {
		_ = ws.SetReadDeadline(time.Now().Add(max(3*d.cfg.HeartbeatInterval, 2*time.Second)))
		var m api.Message
		if err := ws.ReadJSON(&m); err != nil {
			if ctx.Err() == nil {
				log.Printf("realtime: connection lost: %v", err)
			}
			return true
		}
		d.handleEvent(m)
	}
}

func (d *Daemon) dialEvents(ctx context.Context) (*websocket.Conn, error) {
	dial := func(token string) (*websocket.Conn, error) {
		d.mu.Lock()
		base := d.saved.ServerURL
		d.mu.Unlock()
		eventsURL := "ws" + strings.TrimPrefix(base, "http") + api.PathEvents
		dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second, Proxy: http.ProxyFromEnvironment}
		ws, resp, err := dialer.DialContext(ctx, eventsURL, http.Header{"Authorization": {"Bearer " + token}})
		if err != nil && resp != nil {
			defer resp.Body.Close()
			apiErr := &api.Error{}
			if json.NewDecoder(resp.Body).Decode(apiErr) == nil && apiErr.Code != "" {
				return nil, apiErr
			}
		}
		return ws, err
	}
	d.mu.Lock()
	token := d.accessToken
	d.mu.Unlock()
	ws, err := dial(token)
	if api.CodeOf(err) != api.ErrTokenInvalid {
		return ws, err
	}
	if token, err = d.renewSession(ctx, token); err != nil {
		return nil, err
	}
	return dial(token)
}

// sendHeartbeats is the connection's only writer. A failed write ends the
// connection.
func (d *Daemon) sendHeartbeats(ctx context.Context, ws *websocket.Conn, cancel context.CancelFunc) {
	t := time.NewTicker(d.cfg.HeartbeatInterval)
	defer t.Stop()
	for {
		_ = ws.SetWriteDeadline(time.Now().Add(d.cfg.HeartbeatInterval))
		if err := ws.WriteJSON(api.Message{Type: api.MsgHeartbeat}); err != nil {
			cancel()
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (d *Daemon) setConnected(v bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.connected != v {
		d.connected = v
		d.changed()
	}
}

// handleEvent reacts to a pushed message. Unknown types are ignored so the
// server can add events.
func (d *Daemon) handleEvent(m api.Message) {
	switch m.Type {
	case api.EventMemberJoined, api.EventMemberLeft:
		var ev struct {
			Room string `json:"room"`
		}
		if err := json.Unmarshal(m.Data, &ev); err != nil {
			log.Printf("realtime: bad %s event: %v", m.Type, err)
			return
		}
		d.mu.Lock()
		if d.room != nil && d.room.Code == ev.Room {
			d.changed() // front ends reload the member list
		}
		d.mu.Unlock()
	case api.EventKicked:
		d.leftByServer(m, ipc.NoticeKicked)
	case api.EventRoomDeleted:
		d.leftByServer(m, ipc.NoticeRoomDeleted)
	case api.EventNetworkChanged:
		var ev api.NetworkChanged
		if err := json.Unmarshal(m.Data, &ev); err != nil {
			log.Printf("realtime: bad %s event: %v", m.Type, err)
			return
		}
		d.background.Add(1)
		go func() {
			defer d.background.Done()
			d.restartWithNetwork(ev)
		}()
	}
}

// leftByServer handles the server ending this device's membership (kick,
// room deleted): the device leaves locally and remembers why.
func (d *Daemon) leftByServer(m api.Message, notice ipc.Notice) {
	var ev api.RoomEvent
	if err := json.Unmarshal(m.Data, &ev); err != nil {
		log.Printf("realtime: bad %s event: %v", m.Type, err)
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.room == nil || d.room.Code != ev.Room {
		return
	}
	d.notice = notice
	d.changed()
	d.leaveRoomInBackground(false)
}

// restartWithNetwork moves the engine to the room's new network key. The
// membership stays; peers reconnect briefly. If the engine cannot restart,
// the device leaves the room.
func (d *Daemon) restartWithNetwork(ev api.NetworkChanged) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	d.roomMu.Lock()
	defer d.roomMu.Unlock()
	d.mu.Lock()
	inRoom := d.room != nil && d.room.Code == ev.Room
	hostname := d.saved.DeviceName
	d.mu.Unlock()
	if !inRoom {
		return
	}
	cfg, err := engineConfig(ev.Network, hostname)
	if err == nil {
		err = d.cfg.Engine.Restart(ctx, cfg)
	}
	if err != nil {
		log.Printf("engine restart with new network key: %v", err)
		d.leaveRoomLocked(ctx, true)
		return
	}
	d.mu.Lock()
	d.network = ev.Network
	d.changed()
	d.mu.Unlock()
}

// checkMembership leaves the room locally if the server no longer counts
// this device in it (the heartbeat timed out while disconnected).
func (d *Daemon) checkMembership(ctx context.Context) {
	d.roomMu.Lock()
	defer d.roomMu.Unlock()
	d.mu.Lock()
	room := d.room
	d.mu.Unlock()
	if room == nil {
		return
	}
	var ms api.MyMembership
	if err := d.serverCall(ctx, http.MethodGet, api.PathMembership, nil, &ms); err != nil {
		if ctx.Err() == nil {
			log.Printf("realtime: check membership: %v", err)
		}
		return
	}
	if ms.Room == nil || ms.Room.Code != room.Code {
		log.Printf("realtime: the server ended this device's membership in %s", room.Code)
		d.leaveRoomLocked(ctx, false)
	}
}

func (d *Daemon) handleWaitStatus(w http.ResponseWriter, r *http.Request) {
	since, err := strconv.ParseUint(r.URL.Query().Get(ipc.WaitParam), 10, 64)
	if err != nil {
		writeError(w, &api.Error{Code: api.ErrBadRequest, Message: "invalid revision"})
		return
	}
	timeout := time.NewTimer(statusWait)
	defer timeout.Stop()
	d.mu.Lock()
	defer d.mu.Unlock()
	for d.revision == since {
		ch := d.changedCh
		d.mu.Unlock()
		select {
		case <-ch:
			d.mu.Lock()
		case <-timeout.C:
			d.mu.Lock()
			writeJSON(w, http.StatusOK, d.status())
			return
		case <-r.Context().Done():
			d.mu.Lock()
			return
		}
	}
	writeJSON(w, http.StatusOK, d.status())
}

var errNotInRoom = &api.Error{Code: ipc.ErrNotInRoom, Message: "this device is not in a room"}

func (d *Daemon) handleMembers(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	room := d.room
	d.mu.Unlock()
	if room == nil {
		writeError(w, errNotInRoom)
		return
	}
	var list api.MemberList
	if err := d.serverCall(r.Context(), http.MethodGet, api.PathRoomMembers(url.PathEscape(room.Code)), nil, &list); err != nil {
		writeError(w, err)
		return
	}
	out := ipc.MemberList{Members: make([]ipc.Member, 0, len(list.Members))}
	for _, m := range list.Members {
		out.Members = append(out.Members, ipc.Member{
			ID:         m.ID,
			Username:   m.Username,
			DeviceName: m.DeviceName,
			VirtualIP:  m.VirtualIP,
			Self:       m.VirtualIP == room.VirtualIP,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
