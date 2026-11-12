package daemon_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/Bank5y/mokoup2p/client/daemontest"
	"github.com/Bank5y/mokoup2p/client/engine"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

func memberAt(t *testing.T, d *daemontest.Daemon, ip string) ipc.Member {
	t.Helper()
	for _, m := range mustMembers(t, d) {
		if m.VirtualIP == ip {
			return m
		}
	}
	t.Fatalf("no member %s", ip)
	return ipc.Member{}
}

func TestMembersShowPathAndLatencyFromTheEngine(t *testing.T) {
	_, owner, alice, _ := roomWithOwnerAndAlice(t, "")
	ownerIP := mustStatus(t, owner).Room.VirtualIP
	aliceIP := mustStatus(t, alice).Room.VirtualIP

	alice.Engine.SetPeers([]engine.Peer{
		{VirtualIP: netip.MustParseAddr(ownerIP), Path: engine.PathRelay, Latency: 42500 * time.Microsecond},
	})

	got := memberAt(t, alice, ownerIP)
	if got.Path != ipc.PathRelay || got.LatencyMs != 42.5 {
		t.Fatalf("want relay 42.5 ms for the owner, got %+v", got)
	}
	if self := memberAt(t, alice, aliceIP); !self.Self || self.Path != "" || self.LatencyMs != 0 {
		t.Fatalf("this device has no path or latency, got %+v", self)
	}
}

func TestMemberTheEngineDoesNotSeeYetHasNoPath(t *testing.T) {
	_, owner, alice, _ := roomWithOwnerAndAlice(t, "")

	got := memberAt(t, alice, mustStatus(t, owner).Room.VirtualIP)

	if got.Path != "" || got.LatencyMs != 0 {
		t.Fatalf("want unknown path, got %+v", got)
	}
}

func TestSwitchingBetweenDirectAndRelayIsPushedToFrontEnds(t *testing.T) {
	_, owner, alice, _ := roomWithOwnerAndAlice(t, "")
	ownerAddr := netip.MustParseAddr(mustStatus(t, owner).Room.VirtualIP)
	alice.Engine.SetPeers([]engine.Peer{{VirtualIP: ownerAddr, Path: engine.PathDirect, Latency: time.Millisecond}})
	waitFor(t, func() bool { return memberAt(t, alice, ownerAddr.String()).Path == ipc.PathDirect })
	rev := mustStatus(t, alice).Revision

	alice.Engine.SetPeers([]engine.Peer{{VirtualIP: ownerAddr, Path: engine.PathRelay, Latency: 30 * time.Millisecond}})

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if _, err := alice.IPC.WaitStatus(ctx, rev); err != nil {
		t.Fatalf("no change reported after the path switched: %v", err)
	}
	if got := memberAt(t, alice, ownerAddr.String()); got.Path != ipc.PathRelay {
		t.Fatalf("want relay after the switch, got %+v", got)
	}
}

func TestLatencyChangesAloneDoNotWakeFrontEnds(t *testing.T) {
	_, owner, alice, _ := roomWithOwnerAndAlice(t, "")
	ownerAddr := netip.MustParseAddr(mustStatus(t, owner).Room.VirtualIP)
	alice.Engine.SetPeers([]engine.Peer{{VirtualIP: ownerAddr, Path: engine.PathDirect, Latency: time.Millisecond}})
	waitFor(t, func() bool { return memberAt(t, alice, ownerAddr.String()).Path == ipc.PathDirect })
	time.Sleep(10 * daemontest.PeerPollInterval)
	rev := mustStatus(t, alice).Revision

	alice.Engine.SetPeers([]engine.Peer{{VirtualIP: ownerAddr, Path: engine.PathDirect, Latency: 9 * time.Millisecond}})
	time.Sleep(10 * daemontest.PeerPollInterval)

	if st := mustStatus(t, alice); st.Revision != rev {
		t.Fatalf("revision moved from %d to %d on a latency change", rev, st.Revision)
	}
}
