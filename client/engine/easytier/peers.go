package easytier

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/Bank5y/mokoup2p/client/engine"
)

// cliPeer is one row of `easytier-cli -o json peer list` (v2.6.4). Every
// value is a string.
type cliPeer struct {
	IPv4     string `json:"ipv4"`
	Hostname string `json:"hostname"`
	Cost     string `json:"cost"`
	LatMS    string `json:"lat_ms"`
}

// ParsePeerList turns `easytier-cli -o json peer list` output into peers.
// Rows without a virtual IP (relay nodes) and the local node are dropped.
// cost "p2p" means a direct path, "relay(N)" a relayed one. A latency of
// 1000 ms or more is the routing table's placeholder and reported as
// unknown (zero).
func ParsePeerList(out []byte) ([]engine.Peer, error) {
	var rows []cliPeer
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("parse easytier-cli peer list: %w", err)
	}
	peers := []engine.Peer{}
	for _, r := range rows {
		if r.IPv4 == "" || r.Cost == "Local" {
			continue
		}
		ip, err := netip.ParseAddr(r.IPv4)
		if err != nil {
			continue
		}
		p := engine.Peer{VirtualIP: ip, Hostname: r.Hostname, Path: engine.PathRelay}
		if r.Cost == "p2p" {
			p.Path = engine.PathDirect
		}
		if ms, err := strconv.ParseFloat(strings.TrimSpace(r.LatMS), 64); err == nil && ms > 0 && ms < 1000 {
			p.Latency = time.Duration(ms * float64(time.Millisecond))
		}
		peers = append(peers, p)
	}
	return peers, nil
}
