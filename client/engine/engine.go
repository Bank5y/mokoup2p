// Package engine is the daemon's only door to the data plane (EasyTier).
package engine

import (
	"context"
	"errors"
	"net/netip"
	"time"
)

// Engine drives the virtual network. The production implementation manages
// an easytier-core child process; tests use enginetest.Fake.
//
// The daemon serializes Start, Restart and Stop, but Stop may also be called
// while the engine is not running (it is then a no-op) and Peers may be
// called at any time, concurrently with the others.
type Engine interface {
	// Cleanup removes leftovers (processes, virtual NICs, routes) from a
	// previous abnormal exit. It is called once when the daemon starts.
	Cleanup(ctx context.Context) error
	// Start brings the device into the network described by cfg. It is only
	// called while the engine is stopped.
	Start(ctx context.Context, cfg Config) error
	// Stop leaves the network and removes the virtual NIC and routes. It is
	// a no-op when the engine is not running.
	Stop(ctx context.Context) error
	// Restart replaces the running network with cfg, e.g. when the room's
	// network key changed. Members are expected to reconnect briefly.
	Restart(ctx context.Context, cfg Config) error
	// Peers reports the other devices the engine currently sees. It returns
	// an empty list when the engine is not running.
	Peers(ctx context.Context) ([]Peer, error)
}

// ErrDependencyMissing is wrapped by Start errors when something the engine
// needs is not installed on this machine (e.g. Npcap on Windows).
var ErrDependencyMissing = errors.New("engine dependency missing")

// Config is what the engine needs to join a room's virtual network.
type Config struct {
	// NetworkName and NetworkSecret identify and encrypt the network
	// (EasyTier --network-name / --network-secret).
	NetworkName   string
	NetworkSecret string
	// Address is this device's virtual IP with the room's prefix length,
	// e.g. 10.208.3.1/24.
	Address netip.Prefix
	// RelayPeers are relay node addresses such as "tcp://relay.example.com:11010".
	RelayPeers []string
	// Hostname is the device name shown to peers.
	Hostname string
}

// Path is how traffic reaches a peer.
type Path string

const (
	PathDirect Path = "direct"
	PathRelay  Path = "relay"
)

// Peer is another device in the network as the engine sees it.
type Peer struct {
	VirtualIP netip.Addr
	Hostname  string
	Path      Path
	// Latency is the measured round-trip time; zero when unknown.
	Latency time.Duration
}

// Noop is a placeholder until the EasyTier engine exists: it accepts every
// call and never has peers.
type Noop struct{}

func (Noop) Cleanup(context.Context) error         { return nil }
func (Noop) Start(context.Context, Config) error   { return nil }
func (Noop) Stop(context.Context) error            { return nil }
func (Noop) Restart(context.Context, Config) error { return nil }
func (Noop) Peers(context.Context) ([]Peer, error) { return []Peer{}, nil }
