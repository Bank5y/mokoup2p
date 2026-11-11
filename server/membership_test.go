package server_test

import (
	"fmt"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

func TestJoinRoomReturnsNetworkConfig(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	alice := mustLoginAs(t, s, "alice")

	res := mustJoin(t, s, alice, room.Code, "")

	if res.Room != room {
		t.Fatalf("want room %+v, got %+v", room, res.Room)
	}
	n := res.Network
	if n.NetworkName == "" || n.NetworkKey == "" {
		t.Fatalf("network name and key must be set: %+v", n)
	}
	if n.Subnet != room.Subnet || !inSubnet(t, n.VirtualIP, room.Subnet) {
		t.Fatalf("virtual IP %s not a host of room subnet %s (config subnet %s)", n.VirtualIP, room.Subnet, n.Subnet)
	}
	if !slices.Equal(n.RelayPeers, []string{servertest.RelayPeer}) {
		t.Fatalf("want relay peers [%s], got %v", servertest.RelayPeer, n.RelayPeers)
	}
}

func TestJoinPasswordProtectedRoomNeedsTheRightPassword(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoomWithPassword(t, s, owner, "private", "secret")
	alice := mustLoginAs(t, s, "alice")

	for _, wrong := range []string{"", "Secret"} {
		_, err := join(s, alice, room.Code, wrong)
		if code := servertest.ErrorCode(err); code != api.ErrRoomPasswordWrong {
			t.Fatalf("password %q: want %s, got %v", wrong, api.ErrRoomPasswordWrong, err)
		}
	}
	mustJoin(t, s, alice, room.Code, "secret")
}

func TestOwnersDevicesJoinWithoutRoomPassword(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoomWithPassword(t, s, owner, "private", "secret")
	ownerLaptop := mustLoginDevice(t, s, servertest.AdminUsername, servertest.AdminPassword, "laptop")

	mustJoin(t, s, owner, room.Code, "")
	mustJoin(t, s, ownerLaptop, room.Code, "")
}

func TestJoinUnknownRoomIsNotFound(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)

	_, err := join(s, sess, "ABCD-EFGH", "")

	if code := servertest.ErrorCode(err); code != api.ErrRoomNotFound {
		t.Fatalf("want %s, got %v", api.ErrRoomNotFound, err)
	}
}

func TestDisabledAccountCannotJoin(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	alice := mustLoginAs(t, s, "alice")

	s.DisableUser(t, "alice")
	_, err := join(s, alice, room.Code, "")

	if code := servertest.ErrorCode(err); code != api.ErrAccountDisabled {
		t.Fatalf("want %s, got %v", api.ErrAccountDisabled, err)
	}
}

func TestRoomHoldsAtMostDefaultCapacityOfDevices(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoomWithPassword(t, s, owner, "busy", "secret")
	s.AddUser(t, "alice", "alice-password")
	var members []api.Session
	for i := range 32 {
		dev := mustLoginDevice(t, s, "alice", "alice-password", fmt.Sprintf("dev-%d", i))
		mustJoin(t, s, dev, room.Code, "secret")
		members = append(members, dev)
	}
	extra := mustLoginDevice(t, s, "alice", "alice-password", "one-too-many")

	// The password is checked before the capacity.
	if _, err := join(s, extra, room.Code, "wrong"); servertest.ErrorCode(err) != api.ErrRoomPasswordWrong {
		t.Fatalf("wrong password on a full room: want %s, got %v", api.ErrRoomPasswordWrong, err)
	}
	if _, err := join(s, extra, room.Code, "secret"); servertest.ErrorCode(err) != api.ErrRoomFull {
		t.Fatalf("33rd device: want %s, got %v", api.ErrRoomFull, err)
	}
	// A member coming back to the room it is in is not a new member.
	mustJoin(t, s, members[0], room.Code, "secret")

	mustLeave(t, s, members[0])
	mustJoin(t, s, extra, room.Code, "secret")
}

func TestDeviceIsInOneRoomAtATime(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	first := mustCreateRoom(t, s, owner, "first")
	second := mustCreateRoom(t, s, owner, "second")
	alice := mustLoginAs(t, s, "alice")
	mustJoin(t, s, alice, first.Code, "")

	_, err := join(s, alice, second.Code, "")
	if code := servertest.ErrorCode(err); code != api.ErrInOtherRoom {
		t.Fatalf("want %s, got %v", api.ErrInOtherRoom, err)
	}

	mustLeave(t, s, alice)
	mustJoin(t, s, alice, second.Code, "")
}

func TestOtherDevicesOfTheSameUserMayBeInOtherRooms(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	first := mustCreateRoom(t, s, owner, "first")
	second := mustCreateRoom(t, s, owner, "second")
	laptop := mustLoginDevice(t, s, servertest.AdminUsername, servertest.AdminPassword, "laptop")

	mustJoin(t, s, owner, first.Code, "")
	mustJoin(t, s, laptop, second.Code, "")
}

func TestDeviceKeepsItsVirtualIPWhenRejoining(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	alice := mustLoginAs(t, s, "alice")
	bob := mustLoginAs(t, s, "bob")
	first := mustJoin(t, s, alice, room.Code, "").Network.VirtualIP
	mustLeave(t, s, alice)

	// Someone else joining meanwhile must not take alice's address.
	mustJoin(t, s, bob, room.Code, "")
	again := mustJoin(t, s, alice, room.Code, "").Network.VirtualIP

	if again != first {
		t.Fatalf("want virtual IP %s again, got %s", first, again)
	}
}

func TestDevicesInARoomGetDistinctVirtualIPs(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	s.AddUser(t, "alice", "alice-password")

	sessions := []api.Session{owner}
	for i := range 5 {
		sessions = append(sessions, mustLoginDevice(t, s, "alice", "alice-password", fmt.Sprintf("dev-%d", i)))
	}
	seen := map[string]bool{}
	for _, sess := range sessions {
		ip := mustJoin(t, s, sess, room.Code, "").Network.VirtualIP
		if seen[ip] || !inSubnet(t, ip, room.Subnet) {
			t.Fatalf("virtual IP %s is reused or outside %s (seen %v)", ip, room.Subnet, seen)
		}
		seen[ip] = true
	}
}

func TestMembersOfARoomShareItsNetworkButRoomsDoNot(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	other := mustCreateRoom(t, s, owner, "other")
	alice := mustLoginAs(t, s, "alice")
	bob := mustLoginAs(t, s, "bob")

	a := mustJoin(t, s, alice, room.Code, "").Network
	b := mustJoin(t, s, bob, room.Code, "").Network
	o := mustJoin(t, s, owner, other.Code, "").Network

	if a.NetworkName != b.NetworkName || a.NetworkKey != b.NetworkKey {
		t.Fatalf("members of one room got different networks: %+v vs %+v", a, b)
	}
	if o.NetworkName == a.NetworkName || o.NetworkKey == a.NetworkKey {
		t.Fatalf("two rooms share a network name or key: %+v vs %+v", a, o)
	}
	if strings.Contains(strings.ToUpper(a.NetworkName), strings.ReplaceAll(room.Code, "-", "")) {
		t.Fatalf("network name %q reveals the room code %s", a.NetworkName, room.Code)
	}
}

func TestChangingRoomPasswordChangesNetworkKey(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	before := mustJoin(t, s, owner, room.Code, "").Network

	mustUpdateRoom(t, s, owner, room.Code, api.UpdateRoomRequest{Password: ptr("secret")})
	after := mustJoin(t, s, owner, room.Code, "").Network

	if after.NetworkKey == before.NetworkKey || after.NetworkName != before.NetworkName {
		t.Fatalf("want same network with a new key, before %+v after %+v", before, after)
	}
}

func TestLeftDeviceNeedsRoomPasswordAgain(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoomWithPassword(t, s, owner, "private", "secret")
	alice := mustLoginAs(t, s, "alice")
	mustJoin(t, s, alice, room.Code, "secret")

	mustLeave(t, s, alice)
	_, err := join(s, alice, room.Code, "")

	if code := servertest.ErrorCode(err); code != api.ErrRoomPasswordWrong {
		t.Fatalf("want %s, got %v", api.ErrRoomPasswordWrong, err)
	}
}

func TestLeavingWhenNotInARoomIsHarmless(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)

	mustLeave(t, s, sess)
}

func TestServerRestartEndsAllMembershipsButKeepsVirtualIPs(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	first := mustCreateRoom(t, s, owner, "first")
	second := mustCreateRoom(t, s, owner, "second")
	alice := mustLoginAs(t, s, "alice")
	ip := mustJoin(t, s, alice, first.Code, "").Network.VirtualIP

	s.Restart(t, servertest.AdminUsername, servertest.AdminPassword)

	// No longer a member of the first room, so the second one admits alice.
	mustJoin(t, s, alice, second.Code, "")
	mustLeave(t, s, alice)
	if again := mustJoin(t, s, alice, first.Code, "").Network.VirtualIP; again != ip {
		t.Fatalf("want virtual IP %s after restart, got %s", ip, again)
	}
}

func mustLeave(t *testing.T, s *servertest.Server, sess api.Session) {
	t.Helper()
	if err := s.Do(http.MethodDelete, api.PathMembership, sess.AccessToken, nil, nil); err != nil {
		t.Fatalf("leave: %v", err)
	}
}

func mustLoginDevice(t *testing.T, s *servertest.Server, username, password, deviceID string) api.Session {
	t.Helper()
	sess, err := s.Login(username, password, deviceID)
	if err != nil {
		t.Fatalf("login %s on %s: %v", username, deviceID, err)
	}
	return sess
}

func mustCreateRoomWithPassword(t *testing.T, s *servertest.Server, sess api.Session, name, password string) api.Room {
	t.Helper()
	var room api.Room
	if err := s.Do(http.MethodPost, api.PathRooms, sess.AccessToken, api.CreateRoomRequest{Name: name, Password: password}, &room); err != nil {
		t.Fatalf("create room %q: %v", name, err)
	}
	return room
}

func mustJoin(t *testing.T, s *servertest.Server, sess api.Session, code, password string) api.JoinResult {
	t.Helper()
	res, err := join(s, sess, code, password)
	if err != nil {
		t.Fatalf("join %s: %v", code, err)
	}
	return res
}

func join(s *servertest.Server, sess api.Session, code, password string) (api.JoinResult, error) {
	var res api.JoinResult
	err := s.Do(http.MethodPost, api.PathRoomMembers(code), sess.AccessToken, api.JoinRoomRequest{Password: password}, &res)
	return res, err
}

func inSubnet(t *testing.T, ip, subnet string) bool {
	t.Helper()
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	p := netip.MustParsePrefix(subnet)
	// A usable host: neither the network nor the broadcast address.
	return p.Contains(addr) && addr != p.Addr() && addr.As4()[3] != 255
}
