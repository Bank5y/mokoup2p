// Package enginetest provides a fake engine that records what the daemon
// asked of it.
package enginetest

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/Bank5y/mokoup2p/client/engine"
)

// ErrStartFailed is what Start returns after FailNextStart.
var ErrStartFailed = errors.New("fake engine: start failed")

// Fake is an in-memory engine. The zero value is ready to use.
type Fake struct {
	mu           sync.Mutex
	cleanupCalls int
	running      bool
	current      engine.Config
	starts       []engine.Config
	restarts     []engine.Config
	stops        int
	peers        []engine.Peer
	failStart    bool
	hold         chan struct{}
}

func (f *Fake) Cleanup(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanupCalls++
	return nil
}

func (f *Fake) Start(ctx context.Context, cfg engine.Config) error {
	f.mu.Lock()
	hold := f.hold
	f.mu.Unlock()
	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts = append(f.starts, cfg)
	if f.failStart {
		f.failStart = false
		return ErrStartFailed
	}
	if f.running {
		return errors.New("fake engine: Start while running")
	}
	f.running = true
	f.current = cfg
	return nil
}

func (f *Fake) Stop(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops++
	f.running = false
	f.current = engine.Config{}
	return nil
}

func (f *Fake) Restart(_ context.Context, cfg engine.Config) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.restarts = append(f.restarts, cfg)
	f.running = true
	f.current = cfg
	return nil
}

func (f *Fake) Peers(context.Context) ([]engine.Peer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.running {
		return []engine.Peer{}, nil
	}
	return slices.Clone(f.peers), nil
}

// CleanupCalls reports how many times Cleanup was called.
func (f *Fake) CleanupCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cleanupCalls
}

// Running reports whether the engine is in a network, and with which config.
func (f *Fake) Running() (engine.Config, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.current, f.running
}

// Starts returns the configs of every Start call, failed ones included.
func (f *Fake) Starts() []engine.Config {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.starts)
}

// Restarts returns the configs of every Restart call.
func (f *Fake) Restarts() []engine.Config {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.restarts)
}

// StopCalls reports how many times Stop was called.
func (f *Fake) StopCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stops
}

// SetPeers sets what Peers reports while running.
func (f *Fake) SetPeers(peers []engine.Peer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.peers = slices.Clone(peers)
}

// FailNextStart makes the next Start return ErrStartFailed.
func (f *Fake) FailNextStart() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failStart = true
}

// HoldStart makes Start wait until the returned release function is called,
// so tests can observe the daemon while it is joining.
func (f *Fake) HoldStart() (release func()) {
	ch := make(chan struct{})
	f.mu.Lock()
	f.hold = ch
	f.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			f.mu.Lock()
			f.hold = nil
			f.mu.Unlock()
			close(ch)
		})
	}
}
