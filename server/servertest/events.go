package servertest

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Bank5y/mokoup2p/api"
)

// EventWait is how long Expect waits for a message.
const EventWait = 5 * time.Second

// Events is a test device's realtime channel (api.PathEvents). It records
// every message the server sends; Expect takes them out by type.
type Events struct {
	conn    *websocket.Conn
	writeMu sync.Mutex

	mu       sync.Mutex
	changed  chan struct{} // closed and replaced when received or closed changes
	received []api.Message
	closed   error // why the connection ended, nil while open
}

// ConnectEvents opens the realtime channel with an access token. A rejected
// token is returned as an *api.Error.
func (s *Server) ConnectEvents(accessToken string) (*Events, error) {
	url := "ws" + strings.TrimPrefix(s.URL, "http") + api.PathEvents
	header := http.Header{"Authorization": {"Bearer " + accessToken}}
	conn, resp, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		if resp != nil {
			defer resp.Body.Close()
			apiErr := &api.Error{}
			if json.NewDecoder(resp.Body).Decode(apiErr) == nil && apiErr.Code != "" {
				return nil, apiErr
			}
		}
		return nil, err
	}
	e := &Events{conn: conn, changed: make(chan struct{})}
	go e.read()
	return e, nil
}

// MustConnectEvents opens the session's realtime channel; it is closed when
// the test ends.
func (s *Server) MustConnectEvents(t testing.TB, sess api.Session) *Events {
	t.Helper()
	e, err := s.ConnectEvents(sess.AccessToken)
	if err != nil {
		t.Fatalf("connect events: %v", err)
	}
	t.Cleanup(e.Close)
	return e
}

func (e *Events) read() {
	for {
		var m api.Message
		err := e.conn.ReadJSON(&m)
		e.mu.Lock()
		if err != nil {
			e.closed = err
		} else {
			e.received = append(e.received, m)
		}
		close(e.changed)
		e.changed = make(chan struct{})
		e.mu.Unlock()
		if err != nil {
			return
		}
	}
}

// Close drops the connection without saying goodbye, like a crash.
func (e *Events) Close() { e.conn.Close() }

// Send sends a message to the server.
func (e *Events) Send(t testing.TB, m api.Message) {
	t.Helper()
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	if err := e.conn.WriteJSON(m); err != nil {
		t.Fatalf("send %s: %v", m.Type, err)
	}
}

// Heartbeat sends a heartbeat and waits until the server has acknowledged it.
func (e *Events) Heartbeat(t testing.TB) {
	t.Helper()
	e.Send(t, api.Message{Type: api.MsgHeartbeat})
	e.Expect(t, api.MsgHeartbeatAck, nil)
}

// Expect waits for the next message of type typ, decodes its data into out
// (if non-nil) and removes it. Messages of other types stay queued.
func (e *Events) Expect(t testing.TB, typ api.MessageType, out any) {
	t.Helper()
	m, err := e.take(typ, EventWait)
	if err != nil {
		t.Fatalf("expect %s: %v", typ, err)
	}
	if out != nil {
		if err := json.Unmarshal(m.Data, out); err != nil {
			t.Fatalf("decode %s %s: %v", typ, m.Data, err)
		}
	}
}

// ExpectNone fails if a message of type typ arrives within wait.
func (e *Events) ExpectNone(t testing.TB, typ api.MessageType, wait time.Duration) {
	t.Helper()
	if m, err := e.take(typ, wait); err == nil {
		t.Fatalf("unexpected %s: %s", typ, m.Data)
	}
}

// ExpectClosed waits until the server closes the connection.
func (e *Events) ExpectClosed(t testing.TB) {
	t.Helper()
	deadline := time.After(EventWait)
	for {
		e.mu.Lock()
		closed, changed := e.closed, e.changed
		e.mu.Unlock()
		if closed != nil {
			return
		}
		select {
		case <-changed:
		case <-deadline:
			t.Fatal("connection still open")
		}
	}
}

var errTimeout = errors.New("timed out")

func (e *Events) take(typ api.MessageType, wait time.Duration) (api.Message, error) {
	deadline := time.After(wait)
	for {
		e.mu.Lock()
		for i, m := range e.received {
			if m.Type == typ {
				e.received = append(e.received[:i:i], e.received[i+1:]...)
				e.mu.Unlock()
				return m, nil
			}
		}
		closed, changed := e.closed, e.changed
		e.mu.Unlock()
		if closed != nil {
			return api.Message{}, fmt.Errorf("connection closed: %w", closed)
		}
		select {
		case <-changed:
		case <-deadline:
			return api.Message{}, errTimeout
		}
	}
}
