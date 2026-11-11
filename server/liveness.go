package server

import (
	"log"
	"sync"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/internal/clock"
)

// liveness ends the membership of a device that has given no sign of life
// for api.HeartbeatTimeout: its client crashed, lost power or lost the
// network for too long. Signs of life are heartbeats, (re)connecting the
// realtime channel and joining a room.
type liveness struct {
	mu     sync.Mutex
	timers map[int64]*livenessTimer // by device_pk
	closed bool
}

type livenessTimer struct{ clock.Timer }

func newLiveness() *liveness { return &liveness{timers: map[int64]*livenessTimer{}} }

// alive restarts the device's heartbeat timeout.
func (s *Server) alive(devicePK int64) {
	l := s.liveness
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	if old := l.timers[devicePK]; old != nil {
		old.Stop()
	}
	t := &livenessTimer{}
	l.timers[devicePK] = t
	t.Timer = s.clock.AfterFunc(api.HeartbeatTimeout, func() { s.heartbeatTimedOut(devicePK, t) })
}

func (s *Server) heartbeatTimedOut(devicePK int64, t *livenessTimer) {
	l := s.liveness
	l.mu.Lock()
	current := l.timers[devicePK] == t && !l.closed
	if current {
		delete(l.timers, devicePK)
	}
	l.mu.Unlock()
	if !current {
		return // a later sign of life restarted the timeout
	}
	if err := s.endMembership(devicePK, api.LeftByTimeout); err != nil {
		log.Printf("heartbeat timeout of device %d: %v", devicePK, err)
	}
}

// stopAll cancels every timeout; used when the server shuts down.
func (l *liveness) stopAll() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	for pk, t := range l.timers {
		t.Stop()
		delete(l.timers, pk)
	}
}
