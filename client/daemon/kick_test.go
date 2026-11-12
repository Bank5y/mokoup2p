package daemon_test

import (
	"net/http"
	"testing"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/daemontest"
	"github.com/Bank5y/mokoup2p/client/ipc"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

// roomWithOwnerAndAlice has the administrator's daemon create a room with
// the given password and join it, then alice's daemon join it too.
func roomWithOwnerAndAlice(t *testing.T, password string) (s *servertest.Server, owner, alice *daemontest.Daemon, room api.Room) {
	t.Helper()
	s, alice = loggedInPair(t)
	owner = adminDaemon(t, s)
	room, err := owner.IPC.CreateRoom(ctx, api.CreateRoomRequest{Name: "lan party", Password: password})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	mustJoinRoom(t, owner, room.Code, "")
	mustJoinRoom(t, alice, room.Code, password)
	waitForConnected(t, alice, true)
	return s, owner, alice, room
}

// memberIDOf finds a daemon's device in the member list another daemon sees.
func memberIDOf(t *testing.T, viewer, target *daemontest.Daemon) int64 {
	t.Helper()
	ip := mustStatus(t, target).Room.VirtualIP
	for _, m := range mustMembers(t, viewer) {
		if m.VirtualIP == ip {
			return m.ID
		}
	}
	t.Fatalf("member %s not listed", ip)
	return 0
}

func waitForIdleWithNotice(t *testing.T, d *daemontest.Daemon, notice ipc.Notice) {
	t.Helper()
	waitFor(t, func() bool { return mustStatus(t, d).State == ipc.StateIdle })
	if st := mustStatus(t, d); st.Notice != notice {
		t.Fatalf("want notice %q, got %+v", notice, st)
	}
	if _, running := d.Engine.Running(); running {
		t.Fatal("engine still running")
	}
}

func TestOwnerKicksAMemberThroughDaemon(t *testing.T) {
	_, owner, alice, _ := roomWithOwnerAndAlice(t, "")

	if err := owner.IPC.Kick(ctx, memberIDOf(t, owner, alice)); err != nil {
		t.Fatalf("kick: %v", err)
	}

	waitForIdleWithNotice(t, alice, ipc.NoticeKicked)
	if n := len(mustMembers(t, owner)); n != 1 {
		t.Fatalf("owner should be alone in the room, got %d members", n)
	}
}

func TestKickedDaemonCanJoinAgainAndTheNoticeClears(t *testing.T) {
	_, owner, alice, room := roomWithOwnerAndAlice(t, "secret")
	if err := owner.IPC.Kick(ctx, memberIDOf(t, owner, alice)); err != nil {
		t.Fatalf("kick: %v", err)
	}
	waitForIdleWithNotice(t, alice, ipc.NoticeKicked)

	st := mustJoinRoom(t, alice, room.Code, "secret")

	if st.State != ipc.StateInRoom || st.Notice != "" {
		t.Fatalf("want back in the room without a notice, got %+v", st)
	}
}

func TestOnlyTheOwnerCanKickThroughDaemon(t *testing.T) {
	_, owner, alice, _ := roomWithOwnerAndAlice(t, "")

	err := alice.IPC.Kick(ctx, memberIDOf(t, alice, owner))

	if code := servertest.ErrorCode(err); code != api.ErrNotRoomOwner {
		t.Fatalf("want %s, got %v", api.ErrNotRoomOwner, err)
	}
}

func TestKickingWhenInNoRoomFails(t *testing.T) {
	_, d := loggedInPair(t)

	err := d.IPC.Kick(ctx, 1)

	if code := servertest.ErrorCode(err); code != ipc.ErrNotInRoom {
		t.Fatalf("want %s, got %v", ipc.ErrNotInRoom, err)
	}
}

func TestChangedRoomPasswordRestartsTheEngineWithTheNewKey(t *testing.T) {
	_, owner, alice, room := roomWithOwnerAndAlice(t, "old")
	before, _ := alice.Engine.Running()

	if _, err := owner.IPC.UpdateRoom(ctx, room.Code, api.UpdateRoomRequest{Password: ptr("new")}); err != nil {
		t.Fatalf("change password: %v", err)
	}

	waitFor(t, func() bool { return len(alice.Engine.Restarts()) == 1 })
	after := alice.Engine.Restarts()[0]
	if after.NetworkSecret == before.NetworkSecret || after.Address != before.Address || after.NetworkName != before.NetworkName {
		t.Fatalf("only the key should change: before %+v after %+v", before, after)
	}
	if st := mustStatus(t, alice); st.State != ipc.StateInRoom || alice.Engine.StopCalls() != 0 {
		t.Fatalf("member should stay in the room (state %s, stops %d)", st.State, alice.Engine.StopCalls())
	}
}

func TestDeletedRoomStopsTheEngineAndShowsANotice(t *testing.T) {
	s, _, alice, room := roomWithOwnerAndAlice(t, "")
	admin, err := s.Login(servertest.AdminUsername, servertest.AdminPassword, "owner-elsewhere")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Do(http.MethodDelete, api.PathRoom(room.Code), admin.AccessToken, nil, nil); err != nil {
		t.Fatalf("delete room: %v", err)
	}

	waitForIdleWithNotice(t, alice, ipc.NoticeRoomDeleted)
}

func ptr[T any](v T) *T { return &v }
