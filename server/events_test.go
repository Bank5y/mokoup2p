package server_test

import (
	"testing"
	"time"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

// quiet is how long tests wait to be sure an event is not coming.
const quiet = 200 * time.Millisecond

func TestEventsNeedAValidAccessToken(t *testing.T) {
	s := servertest.Start(t)

	_, err := s.ConnectEvents("not-a-token")

	if code := servertest.ErrorCode(err); code != api.ErrTokenInvalid {
		t.Fatalf("want %s, got %v", api.ErrTokenInvalid, err)
	}
}

func TestHeartbeatsAreAcknowledged(t *testing.T) {
	s := servertest.Start(t)
	events := s.MustConnectEvents(t, mustLogin(t, s))

	events.Heartbeat(t)
	events.Heartbeat(t)
}

func TestJoiningIsPushedToTheOtherMembers(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	ownerEvents := s.MustConnectEvents(t, owner)
	mustJoin(t, s, owner, room.Code, "")
	alice := mustLoginAs(t, s, "alice")
	aliceEvents := s.MustConnectEvents(t, alice)

	res := mustJoin(t, s, alice, room.Code, "")

	var ev api.MemberJoined
	ownerEvents.Expect(t, api.EventMemberJoined, &ev)
	if ev.Room != room.Code || ev.Member.Username != "alice" || ev.Member.DeviceName != "alice-dev-host" ||
		ev.Member.VirtualIP != res.Network.VirtualIP || ev.Member.ID == 0 {
		t.Fatalf("unexpected event %+v (virtual IP %s)", ev, res.Network.VirtualIP)
	}
	aliceEvents.ExpectNone(t, api.EventMemberJoined, quiet)
}

func TestLeavingIsPushedToTheRemainingMembers(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	ownerEvents := s.MustConnectEvents(t, owner)
	mustJoin(t, s, owner, room.Code, "")
	alice := mustLoginAs(t, s, "alice")
	res := mustJoin(t, s, alice, room.Code, "")

	mustLeave(t, s, alice)

	var ev api.MemberLeft
	ownerEvents.Expect(t, api.EventMemberLeft, &ev)
	if ev.Room != room.Code || ev.Reason != api.LeftByDevice || ev.Member.Username != "alice" ||
		ev.Member.VirtualIP != res.Network.VirtualIP {
		t.Fatalf("unexpected event %+v", ev)
	}
}

func TestLeavingWhenInNoRoomPushesNothing(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	ownerEvents := s.MustConnectEvents(t, owner)
	mustJoin(t, s, owner, room.Code, "")
	alice := mustLoginAs(t, s, "alice")
	mustJoin(t, s, alice, room.Code, "")
	mustLeave(t, s, alice)
	ownerEvents.Expect(t, api.EventMemberLeft, nil)

	mustLeave(t, s, alice)

	ownerEvents.ExpectNone(t, api.EventMemberLeft, quiet)
}

func TestDisablingAUserIsPushedAsItsDevicesLeaving(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	ownerEvents := s.MustConnectEvents(t, owner)
	mustJoin(t, s, owner, room.Code, "")
	alice := mustLoginAs(t, s, "alice")
	mustJoin(t, s, alice, room.Code, "")

	s.DisableUser(t, "alice")

	var ev api.MemberLeft
	ownerEvents.Expect(t, api.EventMemberLeft, &ev)
	if ev.Reason != api.LeftWhenDisabled || ev.Member.Username != "alice" {
		t.Fatalf("unexpected event %+v", ev)
	}
}

func TestSilentDeviceLeavesAfterHeartbeatTimeout(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	ownerEvents := s.MustConnectEvents(t, owner)
	mustJoin(t, s, owner, room.Code, "")
	alice := mustLoginAs(t, s, "alice")
	aliceEvents := s.MustConnectEvents(t, alice)
	mustJoin(t, s, alice, room.Code, "")
	aliceEvents.Close() // alice's machine loses power

	s.Clock.Advance(20 * time.Second)
	ownerEvents.Heartbeat(t)
	s.Clock.Advance(11 * time.Second)

	var ev api.MemberLeft
	ownerEvents.Expect(t, api.EventMemberLeft, &ev)
	if ev.Reason != api.LeftByTimeout || ev.Member.Username != "alice" || ev.Room != room.Code {
		t.Fatalf("unexpected event %+v", ev)
	}
	if got := mustListMembers(t, s, owner, room.Code); len(got) != 1 || got[0].Username != servertest.AdminUsername {
		t.Fatalf("want only the owner's device left, got %+v", got)
	}
}

func TestHeartbeatsKeepTheMembership(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	ownerEvents := s.MustConnectEvents(t, owner)
	mustJoin(t, s, owner, room.Code, "")
	alice := mustLoginAs(t, s, "alice")
	aliceEvents := s.MustConnectEvents(t, alice)
	mustJoin(t, s, alice, room.Code, "")

	for range 5 {
		s.Clock.Advance(20 * time.Second)
		ownerEvents.Heartbeat(t)
		aliceEvents.Heartbeat(t)
	}

	ownerEvents.ExpectNone(t, api.EventMemberLeft, quiet)
	if got := mustListMembers(t, s, owner, room.Code); len(got) != 2 {
		t.Fatalf("want both devices still in the room, got %+v", got)
	}
}

func TestReconnectingWithinTheTimeoutKeepsTheMembership(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	ownerEvents := s.MustConnectEvents(t, owner)
	mustJoin(t, s, owner, room.Code, "")
	alice := mustLoginAs(t, s, "alice")
	aliceEvents := s.MustConnectEvents(t, alice)
	mustJoin(t, s, alice, room.Code, "")

	aliceEvents.Close()
	s.Clock.Advance(20 * time.Second)
	ownerEvents.Heartbeat(t)
	s.MustConnectEvents(t, alice).Heartbeat(t)
	s.Clock.Advance(20 * time.Second)
	ownerEvents.Heartbeat(t)

	ownerEvents.ExpectNone(t, api.EventMemberLeft, quiet)
	if got := mustListMembers(t, s, owner, room.Code); len(got) != 2 {
		t.Fatalf("want alice still in the room, got %+v", got)
	}
}

func TestDeviceThatTimedOutCanJoinAgain(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	alice := mustLoginAs(t, s, "alice")
	first := mustJoin(t, s, alice, room.Code, "")
	s.Clock.Advance(31 * time.Second)
	if got := mustListMembers(t, s, owner, room.Code); len(got) != 0 {
		t.Fatalf("want alice timed out, got %+v", got)
	}

	again := mustJoin(t, s, alice, room.Code, "")

	if again.Network.VirtualIP != first.Network.VirtualIP {
		t.Fatalf("want virtual IP %s again, got %s", first.Network.VirtualIP, again.Network.VirtualIP)
	}
}

func TestOutsidersGetNoMemberEvents(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	other := mustCreateRoom(t, s, owner, "other")
	bob := mustLoginAs(t, s, "bob")
	bobEvents := s.MustConnectEvents(t, bob)
	mustJoin(t, s, bob, other.Code, "")
	alice := mustLoginAs(t, s, "alice")

	mustJoin(t, s, alice, room.Code, "")

	bobEvents.ExpectNone(t, api.EventMemberJoined, quiet)
}
