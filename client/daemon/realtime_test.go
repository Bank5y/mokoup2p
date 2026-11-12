package daemon_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/daemontest"
	"github.com/Bank5y/mokoup2p/client/ipc"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

func TestLoggedInDaemonKeepsARealtimeConnection(t *testing.T) {
	_, d := loggedInPair(t)

	waitForConnected(t, d, true)

	if _, err := d.IPC.Logout(ctx); err != nil {
		t.Fatalf("logout: %v", err)
	}
	waitForConnected(t, d, false)
}

func TestMembersListsEveryDeviceInTheRoom(t *testing.T) {
	s, alice := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")
	owner := adminDaemon(t, s)
	ownerSt := mustJoinRoom(t, owner, room.Code, "")
	aliceSt := mustJoinRoom(t, alice, room.Code, "")

	members := mustMembers(t, alice)

	want := []ipc.Member{
		{Username: servertest.AdminUsername, DeviceName: daemontest.Hostname, VirtualIP: ownerSt.Room.VirtualIP},
		{Username: "alice", DeviceName: daemontest.Hostname, VirtualIP: aliceSt.Room.VirtualIP, Self: true},
	}
	if len(members) != len(want) {
		t.Fatalf("want %d members, got %+v", len(want), members)
	}
	for i := range want {
		members[i].ID = 0
		if members[i] != want[i] {
			t.Errorf("member %d: want %+v, got %+v", i, want[i], members[i])
		}
	}
}

func TestMembersNeedsARoom(t *testing.T) {
	_, d := loggedInPair(t)

	_, err := d.IPC.Members(ctx)

	if code := servertest.ErrorCode(err); code != ipc.ErrNotInRoom {
		t.Fatalf("want %s, got %v", ipc.ErrNotInRoom, err)
	}
}

func TestMemberChangesArePushedToTheDaemon(t *testing.T) {
	s, alice := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")
	owner := adminDaemon(t, s)
	mustJoinRoom(t, owner, room.Code, "")
	waitForConnected(t, owner, true)

	rev := mustStatus(t, owner).Revision
	mustJoinRoom(t, alice, room.Code, "")
	rev = waitForPushedMembers(t, owner, rev, 2)

	if _, err := alice.IPC.Leave(ctx); err != nil {
		t.Fatalf("leave: %v", err)
	}
	waitForPushedMembers(t, owner, rev, 1)
}

func TestDaemonHeartbeatsKeepItInTheRoom(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")
	mustJoinRoom(t, d, room.Code, "")
	waitForConnected(t, d, true)
	before := mustStatus(t, d)

	for range 3 {
		letHeartbeatsFlow()
		s.Clock.Advance(20 * time.Second)
	}

	if got := serverMembers(t, s, room.Code); len(got) != 1 {
		t.Fatalf("want the daemon's device still in the room, got %+v", got)
	}
	if st := mustStatus(t, d); st.Revision != before.Revision {
		t.Fatalf("want the connection to stay up and nothing else to change, got %+v after %+v", st, before)
	}
}

func TestReconnectingWithinHeartbeatTimeoutKeepsTheMembership(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")
	mustJoinRoom(t, d, room.Code, "")
	waitForConnected(t, d, true)

	s.SetUnreachable(true)
	waitForConnected(t, d, false)
	s.Clock.Advance(20 * time.Second)
	s.SetUnreachable(false)
	waitForConnected(t, d, true)
	letHeartbeatsFlow()
	s.Clock.Advance(20 * time.Second)

	if got := serverMembers(t, s, room.Code); len(got) != 1 {
		t.Fatalf("want the daemon's device still in the room, got %+v", got)
	}
	if st := mustStatus(t, d); st.State != ipc.StateInRoom {
		t.Fatalf("want still in room, got %+v", st)
	}
	if _, running := d.Engine.Running(); !running || d.Engine.StopCalls() != 0 {
		t.Fatalf("engine interrupted (running %v, stops %d)", running, d.Engine.StopCalls())
	}
}

func TestDaemonCutOffPastHeartbeatTimeoutReturnsToIdle(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")
	mustJoinRoom(t, d, room.Code, "")
	waitForConnected(t, d, true)

	s.SetUnreachable(true)
	waitForConnected(t, d, false)
	s.Clock.Advance(31 * time.Second)
	s.SetUnreachable(false)

	waitFor(t, func() bool { return mustStatus(t, d).State == ipc.StateIdle })
	if _, running := d.Engine.Running(); running {
		t.Fatal("engine still running")
	}
	if got := serverMembers(t, s, room.Code); len(got) != 0 {
		t.Fatalf("want no members, got %+v", got)
	}
}

// letHeartbeatsFlow gives the daemon (which heartbeats every few
// milliseconds in tests) time to send some before the fake clock moves.
func letHeartbeatsFlow() { time.Sleep(10 * daemontest.HeartbeatInterval) }

func waitForConnected(t *testing.T, d *daemontest.Daemon, connected bool) {
	t.Helper()
	waitFor(t, func() bool { return mustStatus(t, d).Connected == connected })
}

func mustMembers(t *testing.T, d *daemontest.Daemon) []ipc.Member {
	t.Helper()
	members, err := d.IPC.Members(ctx)
	if err != nil {
		t.Fatalf("members: %v", err)
	}
	return members
}

// waitForPushedMembers waits until the daemon reports a change after
// revision rev and lists n members, and returns the latest revision. It
// fails if the daemon does not report the change on its own: only changes
// wake WaitStatus, and the wait is far shorter than its long-poll timeout.
func waitForPushedMembers(t *testing.T, d *daemontest.Daemon, rev uint64, n int) uint64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for {
		st, err := d.IPC.WaitStatus(ctx, rev)
		if err != nil {
			t.Fatalf("no change pushed while waiting for %d members: %v", n, err)
		}
		rev = st.Revision
		if len(mustMembers(t, d)) == n {
			return rev
		}
	}
}

// serverMembers lists a room's members as the administrator sees them.
func serverMembers(t *testing.T, s *servertest.Server, code string) []api.Member {
	t.Helper()
	admin, err := s.Login(servertest.AdminUsername, servertest.AdminPassword, "inspector")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	var list api.MemberList
	if err := s.Do(http.MethodGet, api.PathRoomMembers(code), admin.AccessToken, nil, &list); err != nil {
		t.Fatalf("list members: %v", err)
	}
	return list.Members
}
