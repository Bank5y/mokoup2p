package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Bank5y/mokoup2p/api"
)

// The realtime channel (api.PathEvents): one WebSocket per logged-in
// device. It carries the device's heartbeats and the events pushed to it.
// See api/events.go for the protocol.

const (
	// outboxSize bounds the messages queued for a device; a device that
	// falls this far behind is disconnected and resynchronizes when it
	// reconnects.
	outboxSize   = 64
	writeTimeout = 10 * time.Second
	// maxMessageSize bounds what a device may send.
	maxMessageSize = 4096
)

var upgrader = websocket.Upgrader{
	// Devices are not browsers; the access token is what authenticates.
	CheckOrigin: func(*http.Request) bool { return true },
}

// hub knows every device's open realtime connection.
type hub struct {
	mu    sync.Mutex
	conns map[int64]*deviceConn // by device_pk
}

type deviceConn struct {
	ws     *websocket.Conn
	outbox chan api.Message
	done   chan struct{} // closed when the connection is shut down
	once   sync.Once
}

func newHub() *hub { return &hub{conns: map[int64]*deviceConn{}} }

// attach registers a device's new connection, replacing (and closing) an
// older one.
func (h *hub) attach(devicePK int64, c *deviceConn) {
	h.mu.Lock()
	old := h.conns[devicePK]
	h.conns[devicePK] = c
	h.mu.Unlock()
	if old != nil {
		old.shutdown()
	}
}

// detach forgets c if it is still the device's connection.
func (h *hub) detach(devicePK int64, c *deviceConn) {
	h.mu.Lock()
	if h.conns[devicePK] == c {
		delete(h.conns, devicePK)
	}
	h.mu.Unlock()
}

// send queues m for the device if it is connected. A device whose queue is
// full is disconnected.
func (h *hub) send(devicePK int64, m api.Message) {
	h.mu.Lock()
	c := h.conns[devicePK]
	h.mu.Unlock()
	if c != nil {
		c.send(m)
	}
}

func (h *hub) closeAll() {
	h.mu.Lock()
	conns := h.conns
	h.conns = map[int64]*deviceConn{}
	h.mu.Unlock()
	for _, c := range conns {
		c.shutdown()
	}
}

func (c *deviceConn) send(m api.Message) {
	select {
	case c.outbox <- m:
	case <-c.done:
	default:
		log.Printf("realtime: device too slow, disconnecting")
		c.shutdown()
	}
}

func (c *deviceConn) shutdown() {
	c.once.Do(func() {
		close(c.done)
		c.ws.Close()
	})
}

// writeLoop sends queued messages until the connection is shut down.
func (c *deviceConn) writeLoop() {
	for {
		select {
		case <-c.done:
			return
		case m := <-c.outbox:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.ws.WriteJSON(m); err != nil {
				c.shutdown()
				return
			}
		}
	}
}

func (s *Server) realtimeRoutes() {
	s.mux.HandleFunc("GET "+api.PathEvents, s.authenticated(s.handleEvents))
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request, se session) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // the upgrader has answered already
	}
	ws.SetReadLimit(maxMessageSize)
	c := &deviceConn{ws: ws, outbox: make(chan api.Message, outboxSize), done: make(chan struct{})}
	s.hub.attach(se.devicePK, c)
	s.alive(se.devicePK)
	defer s.hub.detach(se.devicePK, c)
	defer c.shutdown()
	go c.writeLoop()

	for {
		// A device that stays silent this long is gone (a half-open
		// connection); its membership has ended by then anyway.
		_ = ws.SetReadDeadline(time.Now().Add(2 * api.HeartbeatTimeout))
		var m api.Message
		if err := ws.ReadJSON(&m); err != nil {
			return
		}
		switch m.Type {
		case api.MsgHeartbeat:
			s.alive(se.devicePK)
			c.send(api.Message{Type: api.MsgHeartbeatAck})
		}
	}
}

// push sends an event to one device, if it is connected.
func (s *Server) push(devicePK int64, t api.MessageType, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		log.Printf("realtime: encode %s: %v", t, err)
		return
	}
	s.hub.send(devicePK, api.Message{Type: t, Data: raw})
}

// pushToRoom sends an event to every member of a room except one device.
func (s *Server) pushToRoom(roomID, exceptDevicePK int64, t api.MessageType, data any) {
	members, err := roomMembers(s.db, roomID)
	if err != nil {
		log.Printf("realtime: list members of room %d: %v", roomID, err)
		return
	}
	for _, m := range members {
		if m.ID != exceptDevicePK {
			s.push(m.ID, t, data)
		}
	}
}
