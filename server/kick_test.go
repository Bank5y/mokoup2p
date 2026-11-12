package server_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

func kick(s *servertest.Server, sess api.Session, code string, memberID int64) error {
	return s.Do(http.MethodDelete, api.PathRoomMember(code, strconv.FormatInt(memberID, 10)), sess.AccessToken, nil, nil)
}

func memberID(t *testing.T, s *servertest.Server, sess api.Session, code, virtualIP string) int64 {
	t.Helper()
	for _, m := range mustListMembers(t, s, sess, code) {
		if m.VirtualIP == virtualIP {
			return m.ID
		}
	}
	t.Fatalf("no member with virtual IP %s in %s", virtualIP, code)
	return 0
}

func TestOwnerKicksAMember(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	mustJoin(t, s, owner, room.Code, "")
	ownerEvents := s.MustConnectEvents(t, owner)
	alice := mustLoginAs(t, s, "alice")
	aliceEvents := s.MustConnectEvents(t, alice)
	res := mustJoin(t, s, alice, room.Code, "")

	if err := kick(s, owner, room.Code, memberID(t, s, owner, room.Code, res.Network.VirtualIP)); err != nil {
		t.Fatalf("kick: %v", err)
	}

	var kicked api.RoomEvent
	aliceEvents.Expect(t, api.EventKicked, &kicked)
	if kicked.Room != room.Code {
		t.Fatalf("kicked event for %q, want %q", kicked.Room, room.Code)
	}
	var left api.MemberLeft
	ownerEvents.Expect(t, api.EventMemberLeft, &left)
	if left.Reason != api.LeftByKick || left.Member.Username != "alice" {
		t.Fatalf("unexpected member_left %+v", left)
	}
	if got := mustMyMembership(t, s, alice); got.Room != nil {
		t.Fatalf("kicked device should be in no room, got %+v", got.Room)
	}
}

func TestKickedMemberCanJoinAgainWithThePassword(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoomWithPassword(t, s, owner, "lan party", "secret")
	alice := mustLoginAs(t, s, "alice")
	res := mustJoin(t, s, alice, room.Code, "secret")
	if err := kick(s, owner, room.Code, memberID(t, s, owner, room.Code, res.Network.VirtualIP)); err != nil {
		t.Fatalf("kick: %v", err)
	}

	again := mustJoin(t, s, alice, room.Code, "secret")

	if again.Network.VirtualIP != res.Network.VirtualIP {
		t.Fatalf("virtual IP changed after rejoin: %s -> %s", res.Network.VirtualIP, again.Network.VirtualIP)
	}
}

func TestKickingKeepsTheNetworkKey(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	before := mustJoin(t, s, owner, room.Code, "")
	alice := mustLoginAs(t, s, "alice")
	res := mustJoin(t, s, alice, room.Code, "")

	if err := kick(s, owner, room.Code, memberID(t, s, owner, room.Code, res.Network.VirtualIP)); err != nil {
		t.Fatalf("kick: %v", err)
	}

	after := mustJoin(t, s, owner, room.Code, "")
	if after.Network.NetworkKey != before.Network.NetworkKey {
		t.Fatal("kicking must not change the network key")
	}
}

func TestOnlyTheOwnerCanKick(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	ownerRes := mustJoin(t, s, owner, room.Code, "")
	alice := mustLoginAs(t, s, "alice")
	mustJoin(t, s, alice, room.Code, "")

	err := kick(s, alice, room.Code, memberID(t, s, alice, room.Code, ownerRes.Network.VirtualIP))

	if code := servertest.ErrorCode(err); code != api.ErrNotRoomOwner {
		t.Fatalf("want %s, got %v", api.ErrNotRoomOwner, err)
	}
}

func TestKickingADeviceNotInTheRoomFails(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	other := mustCreateRoom(t, s, owner, "other")
	alice := mustLoginAs(t, s, "alice")
	res := mustJoin(t, s, alice, other.Code, "")
	aliceID := memberID(t, s, owner, other.Code, res.Network.VirtualIP)

	err := kick(s, owner, room.Code, aliceID)

	if code := servertest.ErrorCode(err); code != api.ErrMemberNotFound {
		t.Fatalf("want %s, got %v", api.ErrMemberNotFound, err)
	}
	if got := mustMyMembership(t, s, alice); got.Room == nil || got.Room.Code != other.Code {
		t.Fatalf("alice should still be in %s, got %+v", other.Code, got.Room)
	}
}

func TestChangingThePasswordPushesANewNetworkConfigToMembers(t *testing.T) {
	for name, password := range map[string]string{"set": "secret", "remove": ""} {
		t.Run(name, func(t *testing.T) {
			s := servertest.Start(t)
			owner := mustLogin(t, s)
			room := mustCreateRoomWithPassword(t, s, owner, "lan party", "old")
			alice := mustLoginAs(t, s, "alice")
			aliceEvents := s.MustConnectEvents(t, alice)
			before := mustJoin(t, s, alice, room.Code, "old")

			mustUpdateRoom(t, s, owner, room.Code, api.UpdateRoomRequest{Password: ptr(password)})

			var ev api.NetworkChanged
			aliceEvents.Expect(t, api.EventNetworkChanged, &ev)
			if ev.Room != room.Code {
				t.Fatalf("event for %q, want %q", ev.Room, room.Code)
			}
			if ev.Network.NetworkKey == "" || ev.Network.NetworkKey == before.Network.NetworkKey {
				t.Fatal("pushed config should carry a new network key")
			}
			if ev.Network.VirtualIP != before.Network.VirtualIP || ev.Network.NetworkName != before.Network.NetworkName {
				t.Fatalf("only the key should change: before %+v after %+v", before.Network, ev.Network)
			}
			if got := mustMyMembership(t, s, alice); got.Room == nil {
				t.Fatal("changing the password must not end memberships")
			}
		})
	}
}

func TestRenamingARoomPushesNoNetworkChange(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	alice := mustLoginAs(t, s, "alice")
	aliceEvents := s.MustConnectEvents(t, alice)
	mustJoin(t, s, alice, room.Code, "")

	mustUpdateRoom(t, s, owner, room.Code, api.UpdateRoomRequest{Name: ptr("renamed")})

	aliceEvents.ExpectNone(t, api.EventNetworkChanged, quiet)
}

func TestDeletingARoomPushesRoomDeletedToEveryMember(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	mustJoin(t, s, owner, room.Code, "")
	ownerEvents := s.MustConnectEvents(t, owner)
	alice := mustLoginAs(t, s, "alice")
	aliceEvents := s.MustConnectEvents(t, alice)
	mustJoin(t, s, alice, room.Code, "")

	if err := s.Do(http.MethodDelete, api.PathRoom(room.Code), owner.AccessToken, nil, nil); err != nil {
		t.Fatalf("delete room: %v", err)
	}

	for who, events := range map[string]*servertest.Events{"owner": ownerEvents, "alice": aliceEvents} {
		var ev api.RoomEvent
		events.Expect(t, api.EventRoomDeleted, &ev)
		if ev.Room != room.Code {
			t.Fatalf("%s: room_deleted for %q, want %q", who, ev.Room, room.Code)
		}
	}
	if got := mustMyMembership(t, s, alice); got.Room != nil {
		t.Fatalf("alice should be in no room, got %+v", got.Room)
	}
}
