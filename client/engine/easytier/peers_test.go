package easytier_test

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/Bank5y/mokoup2p/client/engine"
	"github.com/Bank5y/mokoup2p/client/engine/easytier"
)

// peerListV264 is real `easytier-cli -o json peer list` output from v2.6.4
// on node A: the relay, B behind the relay, C directly connected, and B's
// relayed latency still at its 1000 ms placeholder.
const peerListV264 = `[
{"cidr":"10.144.0.1/24","ipv4":"10.144.0.1","hostname":"nodeA","cost":"Local","lat_ms":"-","loss_rate":"-","rx_bytes":"-","tx_bytes":"-","tunnel_proto":"-","nat_type":"Unknown","id":"1","version":"2.6.4-8428a89d"},
{"cidr":"","ipv4":"","hostname":"PublicServer_192.168.1.3","cost":"p2p","lat_ms":"0.11","loss_rate":"0.0%","rx_bytes":"41.86 kB","tx_bytes":"46.41 kB","tunnel_proto":"tcp","nat_type":"Unknown","id":"2","version":"2.6.4-8428a89d"},
{"cidr":"10.144.0.2/24","ipv4":"10.144.0.2","hostname":"nodeB","cost":"relay(2)","lat_ms":"1000.00","loss_rate":"0.0%","rx_bytes":"0 B","tx_bytes":"0 B","tunnel_proto":"","nat_type":"Unknown","id":"2435941394","version":"2.6.4-8428a89d"},
{"cidr":"10.144.0.3/24","ipv4":"10.144.0.3","hostname":"nodeC","cost":"p2p","lat_ms":"0.65","loss_rate":"0.0%","rx_bytes":"863 B","tx_bytes":"1.25 kB","tunnel_proto":"tcp","nat_type":"Unknown","id":"3","version":"2.6.4-8428a89d"}
]`

func TestParsePeerListReportsMembersWithPathAndLatency(t *testing.T) {
	got, err := easytier.ParsePeerList([]byte(peerListV264))
	if err != nil {
		t.Fatal(err)
	}
	want := []engine.Peer{
		{VirtualIP: netip.MustParseAddr("10.144.0.2"), Hostname: "nodeB", Path: engine.PathRelay},
		{VirtualIP: netip.MustParseAddr("10.144.0.3"), Hostname: "nodeC", Path: engine.PathDirect, Latency: 650 * time.Microsecond},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("want %+v, got %+v", want, got)
	}
}

func TestParsePeerListOfLoneNodeIsEmpty(t *testing.T) {
	got, err := easytier.ParsePeerList([]byte(`[]`))
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("want empty non-nil list, got %#v, %v", got, err)
	}
}
