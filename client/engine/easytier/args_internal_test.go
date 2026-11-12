package easytier

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/Bank5y/mokoup2p/client/engine"
)

var testConfig = engine.Config{
	NetworkName: "mokoup2p-0123456789abcdef",
	Address:     netip.MustParsePrefix("10.208.3.1/24"),
	Hostname:    "pc",
}

func TestBroadcastRelayIsOnlyRequestedWhenEnabled(t *testing.T) {
	on := New(Options{UDPBroadcastRelay: true}).args(testConfig)
	off := New(Options{}).args(testConfig)

	if i := slices.Index(on, "--enable-udp-broadcast-relay"); i < 0 || on[i+1] != "true" {
		t.Fatalf("want --enable-udp-broadcast-relay true, got %q", on)
	}
	if slices.Contains(off, "--enable-udp-broadcast-relay") {
		t.Fatalf("relay requested although disabled: %q", off)
	}
}
