package daemon

import (
	"context"
	"log"
	"maps"
	"time"

	"github.com/Bank5y/mokoup2p/client/engine"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

// addPaths fills in how the engine reaches each member and the latency,
// matching members to engine peers by virtual IP.
func (d *Daemon) addPaths(ctx context.Context, members []ipc.Member) {
	peers, err := d.cfg.Engine.Peers(ctx)
	if err != nil {
		log.Printf("engine peers: %v", err)
		return
	}
	byIP := make(map[string]engine.Peer, len(peers))
	for _, p := range peers {
		byIP[p.VirtualIP.String()] = p
	}
	for i := range members {
		p, ok := byIP[members[i].VirtualIP]
		if !ok || members[i].Self {
			continue
		}
		members[i].Path = ipc.Path(p.Path)
		members[i].LatencyMs = float64(p.Latency.Microseconds()) / 1000
	}
}

// watchPaths polls the engine while the device is in a room and reports a
// change to the front ends whenever a member switches between direct and
// relay, or the engine starts or stops seeing one. Latency changes alone
// are not reported: front ends refresh those on their own schedule.
func (d *Daemon) watchPaths(ctx context.Context) {
	var last map[string]engine.Path
	ticker := time.NewTicker(d.cfg.PeerPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		d.mu.Lock()
		inRoom := d.room != nil
		d.mu.Unlock()
		if !inRoom {
			last = nil
			continue
		}
		peers, err := d.cfg.Engine.Peers(ctx)
		if err != nil {
			continue
		}
		paths := make(map[string]engine.Path, len(peers))
		for _, p := range peers {
			paths[p.VirtualIP.String()] = p.Path
		}
		if last == nil {
			last = map[string]engine.Path{} // just joined: no member seen yet
		}
		if !maps.Equal(paths, last) {
			d.mu.Lock()
			d.changed()
			d.mu.Unlock()
		}
		last = paths
	}
}
