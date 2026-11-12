package daemon_test

import (
	"fmt"
	"net/http"
	"net/netip"
	"slices"
	"testing"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/daemontest"
	"github.com/Bank5y/mokoup2p/client/engine"
	"github.com/Bank5y/mokoup2p/client/ipc"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

func TestJoinStartsEngineWithRoomNetwork(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")

	st, err := d.IPC.Join(ctx, room.Code, "")
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	if st.State != ipc.StateInRoom || st.Room == nil {
		t.Fatalf("want in room, got %+v", st)
	}
	if st.Room.Code != room.Code || st.Room.Name != "lan party" || st.Room.Subnet != room.Subnet {
		t.Fatalf("unexpected current room %+v for %+v", st.Room, room)
	}
	cfg, running := d.Engine.Running()
	if !running {
		t.Fatal("engine not running after join")
	}
	subnet := netip.MustParsePrefix(room.Subnet)
	if cfg.Address.Bits() != 24 || !subnet.Contains(cfg.Address.Addr()) || cfg.Address.Addr().String() != st.Room.VirtualIP {
		t.Fatalf("engine address %s does not match virtual IP %s in %s", cfg.Address, st.Room.VirtualIP, room.Subnet)
	}
	if cfg.NetworkName == "" || cfg.NetworkSecret == "" || cfg.Hostname != daemontest.Hostname {
		t.Fatalf("engine config incomplete: %+v", cfg)
	}
	if !slices.Equal(cfg.RelayPeers, []string{servertest.RelayPeer}) {
		t.Fatalf("want relay peers [%s], got %v", servertest.RelayPeer, cfg.RelayPeers)
	}
}

func TestLeaveStopsEngineAndEndsMembership(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")
	other := ownerCreatesRoom(t, s, "other", "")
	mustJoinRoom(t, d, room.Code, "")

	st, err := d.IPC.Leave(ctx)
	if err != nil {
		t.Fatalf("leave: %v", err)
	}

	if st.State != ipc.StateIdle || st.Room != nil {
		t.Fatalf("want idle after leave, got %+v", st)
	}
	if _, running := d.Engine.Running(); running || d.Engine.StopCalls() == 0 {
		t.Fatalf("engine still running after leave (stops: %d)", d.Engine.StopCalls())
	}
	assertDeviceInNoRoom(t, s, d, other)
}

func TestLeaveWhenNotInARoomIsHarmless(t *testing.T) {
	_, d := loggedInPair(t)

	st, err := d.IPC.Leave(ctx)

	if err != nil || st.State != ipc.StateIdle {
		t.Fatalf("want idle, got %+v, %v", st, err)
	}
}

func TestRejoinAfterLeaveNeedsRoomPasswordAndKeepsVirtualIP(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "private", "secret")
	first := mustJoinRoom(t, d, room.Code, "secret")
	if _, err := d.IPC.Leave(ctx); err != nil {
		t.Fatalf("leave: %v", err)
	}

	_, err := d.IPC.Join(ctx, room.Code, "")
	if code := servertest.ErrorCode(err); code != api.ErrRoomPasswordWrong {
		t.Fatalf("rejoin without password: want %s, got %v", api.ErrRoomPasswordWrong, err)
	}
	again := mustJoinRoom(t, d, room.Code, "secret")

	if again.Room.VirtualIP != first.Room.VirtualIP {
		t.Fatalf("want virtual IP %s again, got %s", first.Room.VirtualIP, again.Room.VirtualIP)
	}
}

func TestJoinRejectionsAreRelayedAndLeaveDaemonIdle(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "private", "secret")

	cases := map[string]struct {
		code, password string
		want           api.ErrorCode
	}{
		"unknown room":   {"ABCD-EFGH", "", api.ErrRoomNotFound},
		"wrong password": {room.Code, "nope", api.ErrRoomPasswordWrong},
	}
	for name, c := range cases {
		_, err := d.IPC.Join(ctx, c.code, c.password)
		if code := servertest.ErrorCode(err); code != c.want {
			t.Errorf("%s: want %s, got %v", name, c.want, err)
		}
	}

	if st := mustStatus(t, d); st.State != ipc.StateIdle || st.Room != nil {
		t.Fatalf("want idle after rejected joins, got %+v", st)
	}
	if n := len(d.Engine.Starts()); n != 0 {
		t.Fatalf("engine started %d times for rejected joins", n)
	}
}

func TestJoinRequiresLogin(t *testing.T) {
	d := daemontest.Start(t)

	_, err := d.IPC.Join(ctx, "ABCD-EFGH", "")

	if code := servertest.ErrorCode(err); code != ipc.ErrNotLoggedIn {
		t.Fatalf("want %s, got %v", ipc.ErrNotLoggedIn, err)
	}
}

func TestJoinWhileInARoomIsRefused(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")
	other := ownerCreatesRoom(t, s, "other", "")
	mustJoinRoom(t, d, room.Code, "")

	_, err := d.IPC.Join(ctx, other.Code, "")

	if code := servertest.ErrorCode(err); code != ipc.ErrInRoom {
		t.Fatalf("want %s, got %v", ipc.ErrInRoom, err)
	}
	if st := mustStatus(t, d); st.State != ipc.StateInRoom || st.Room.Code != room.Code {
		t.Fatalf("want still in %s, got %+v", room.Code, st)
	}
	if n := len(d.Engine.Starts()); n != 1 {
		t.Fatalf("want engine started once, got %d", n)
	}
}

func TestStatusShowsJoiningWhileEngineStarts(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")
	release := d.Engine.HoldStart()
	defer release()

	joined := make(chan error, 1)
	go func() {
		_, err := d.IPC.Join(ctx, room.Code, "")
		joined <- err
	}()
	waitFor(t, func() bool { return mustStatus(t, d).State == ipc.StateJoining })
	release()

	if err := <-joined; err != nil {
		t.Fatalf("join: %v", err)
	}
	if st := mustStatus(t, d); st.State != ipc.StateInRoom {
		t.Fatalf("want in room, got %+v", st)
	}
}

// The GUI's tray "退出" calls Leave and then exits, possibly while a join is
// still under way. The daemon must still end up idle, with the engine stopped
// and the membership ended.
func TestLeaveWhileJoiningEndsIdle(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")
	other := ownerCreatesRoom(t, s, "other", "")
	release := d.Engine.HoldStart()
	defer release()

	joined := make(chan error, 1)
	go func() {
		_, err := d.IPC.Join(ctx, room.Code, "")
		joined <- err
	}()
	waitFor(t, func() bool { return mustStatus(t, d).State == ipc.StateJoining })
	left := make(chan ipc.Status, 1)
	go func() {
		st, err := d.IPC.Leave(ctx)
		if err != nil {
			t.Errorf("leave: %v", err)
		}
		left <- st
	}()
	release()

	<-joined
	if st := <-left; st.State != ipc.StateIdle || st.Room != nil {
		t.Fatalf("want idle after leave, got %+v", st)
	}
	if _, running := d.Engine.Running(); running {
		t.Fatal("engine still running after leave")
	}
	assertDeviceInNoRoom(t, s, d, other)
}

func TestEngineStartFailureLeavesTheRoomAgain(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")
	other := ownerCreatesRoom(t, s, "other", "")
	d.Engine.FailNextStart()

	_, err := d.IPC.Join(ctx, room.Code, "")

	if code := servertest.ErrorCode(err); code != ipc.ErrEngineFailed {
		t.Fatalf("want %s, got %v", ipc.ErrEngineFailed, err)
	}
	if st := mustStatus(t, d); st.State != ipc.StateIdle {
		t.Fatalf("want idle, got %+v", st)
	}
	if _, running := d.Engine.Running(); running {
		t.Fatal("engine running after failed start")
	}
	assertDeviceInNoRoom(t, s, d, other)
}

func TestEngineMissingADependencyIsReportedAsSuch(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")
	d.Engine.FailNextStartWith(fmt.Errorf("Packet.dll: %w", engine.ErrDependencyMissing))

	_, err := d.IPC.Join(ctx, room.Code, "")

	if code := servertest.ErrorCode(err); code != ipc.ErrEngineDependencyMissing {
		t.Fatalf("want %s, got %v", ipc.ErrEngineDependencyMissing, err)
	}
	if st := mustStatus(t, d); st.State != ipc.StateIdle {
		t.Fatalf("want idle, got %+v", st)
	}
}

func TestChangingServerWhileInARoomIsRefused(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")
	mustJoinRoom(t, d, room.Code, "")

	_, err := d.IPC.SetServer(ctx, "http://other.example")

	if code := servertest.ErrorCode(err); code != ipc.ErrInRoom {
		t.Fatalf("want %s, got %v", ipc.ErrInRoom, err)
	}
}

func TestDaemonRestartLeavesRoomAndDoesNotRejoin(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")
	other := ownerCreatesRoom(t, s, "other", "")
	mustJoinRoom(t, d, room.Code, "")
	oldEngine := d.Engine

	d.Restart(t)

	if _, running := oldEngine.Running(); running {
		t.Fatal("engine of the stopped daemon still running")
	}
	waitFor(t, func() bool { return mustStatus(t, d).State == ipc.StateIdle })
	if d.Engine.CleanupCalls() != 1 || len(d.Engine.Starts()) != 0 {
		t.Fatalf("want cleanup and no start after restart, got %d cleanups, %d starts", d.Engine.CleanupCalls(), len(d.Engine.Starts()))
	}
	assertDeviceInNoRoom(t, s, d, other)
}

func TestJoinWorksAfterDaemonCouldNotTellServerItLeft(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")
	other := ownerCreatesRoom(t, s, "other", "")
	mustJoinRoom(t, d, room.Code, "")

	// The daemon goes down while the server is unreachable, so the server
	// still counts the device in the room.
	s.SetUnreachable(true)
	d.Restart(t)
	s.SetUnreachable(false)
	waitFor(t, func() bool { return mustStatus(t, d).State == ipc.StateIdle })

	st := mustJoinRoom(t, d, other.Code, "")
	if st.Room.Code != other.Code {
		t.Fatalf("want in %s, got %+v", other.Code, st)
	}
}

func TestEngineStopsWhenSessionIsRejected(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")
	mustJoinRoom(t, d, room.Code, "")

	// Disabling the account revokes its sessions: the next server call is
	// rejected, and so is the attempt to renew the session.
	s.DisableUser(t, "alice")
	if _, err := d.IPC.MyRooms(ctx); err == nil {
		t.Fatal("my rooms succeeded for a disabled account")
	}

	waitFor(t, func() bool {
		_, running := d.Engine.Running()
		return !running
	})
	if st := mustStatus(t, d); st.State != ipc.StateLoggedOut || st.Room != nil {
		t.Fatalf("want logged out and in no room, got %+v", st)
	}
}

func TestLogoutLeavesTheRoom(t *testing.T) {
	s, d := loggedInPair(t)
	room := ownerCreatesRoom(t, s, "lan party", "")
	other := ownerCreatesRoom(t, s, "other", "")
	mustJoinRoom(t, d, room.Code, "")

	st, err := d.IPC.Logout(ctx)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}

	if st.State != ipc.StateLoggedOut || st.Room != nil {
		t.Fatalf("want logged out and in no room, got %+v", st)
	}
	if _, running := d.Engine.Running(); running {
		t.Fatal("engine still running after logout")
	}
	assertDeviceInNoRoom(t, s, d, other)
}

func mustJoinRoom(t *testing.T, d *daemontest.Daemon, code, password string) ipc.Status {
	t.Helper()
	st, err := d.IPC.Join(ctx, code, password)
	if err != nil {
		t.Fatalf("join %s: %v", code, err)
	}
	return st
}

func mustStatus(t *testing.T, d *daemontest.Daemon) ipc.Status {
	t.Helper()
	st, err := d.IPC.Status(ctx)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	return st
}

// assertDeviceInNoRoom checks on the server that the daemon's device is not
// a member of any room: it may join other, which it then leaves again.
func assertDeviceInNoRoom(t *testing.T, s *servertest.Server, d *daemontest.Daemon, other api.Room) {
	t.Helper()
	sess, err := s.Login("alice", "alice-password", mustStatus(t, d).DeviceID)
	if err != nil {
		t.Fatalf("login as the daemon's device: %v", err)
	}
	if err := s.Do(http.MethodPost, api.PathRoomMembers(other.Code), sess.AccessToken, api.JoinRoomRequest{}, nil); err != nil {
		t.Fatalf("device is still in a room on the server: %v", err)
	}
	if err := s.Do(http.MethodDelete, api.PathMembership, sess.AccessToken, nil, nil); err != nil {
		t.Fatalf("leave: %v", err)
	}
}

// loggedInPair starts a server and a daemon logged in as alice, an ordinary
// user; rooms are created by the administrator (see ownerCreatesRoom).
func loggedInPair(t *testing.T) (*servertest.Server, *daemontest.Daemon) {
	t.Helper()
	s := servertest.Start(t)
	s.AddUser(t, "alice", "alice-password")
	d := daemontest.Start(t)
	mustSetServer(t, d, s)
	if _, err := d.IPC.Login(ctx, "alice", "alice-password"); err != nil {
		t.Fatalf("login: %v", err)
	}
	return s, d
}

// ownerCreatesRoom creates a room owned by the administrator, directly on
// the server.
func ownerCreatesRoom(t *testing.T, s *servertest.Server, name, password string) api.Room {
	t.Helper()
	sess, err := s.Login(servertest.AdminUsername, servertest.AdminPassword, "owner-dev")
	if err != nil {
		t.Fatalf("owner login: %v", err)
	}
	var room api.Room
	if err := s.Do(http.MethodPost, api.PathRooms, sess.AccessToken, api.CreateRoomRequest{Name: name, Password: password}, &room); err != nil {
		t.Fatalf("create room: %v", err)
	}
	return room
}
