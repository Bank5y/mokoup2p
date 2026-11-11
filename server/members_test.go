package server_test

import (
	"net/http"
	"testing"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

func TestMemberListShowsEveryMembersUserDeviceAndVirtualIP(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	alice := mustLoginAs(t, s, "alice")
	ownerRes := mustJoin(t, s, owner, room.Code, "")
	aliceRes := mustJoin(t, s, alice, room.Code, "")

	got := mustListMembers(t, s, alice, room.Code)

	want := []api.Member{
		{Username: servertest.AdminUsername, DeviceName: "dev-1-host", VirtualIP: ownerRes.Network.VirtualIP},
		{Username: "alice", DeviceName: "alice-dev-host", VirtualIP: aliceRes.Network.VirtualIP},
	}
	if len(got) != len(want) {
		t.Fatalf("want %d members, got %+v", len(want), got)
	}
	for i := range want {
		if got[i].ID == 0 {
			t.Errorf("member %d has no id: %+v", i, got[i])
		}
		got[i].ID = 0
		if got[i] != want[i] {
			t.Errorf("member %d: want %+v, got %+v", i, want[i], got[i])
		}
	}
}

func TestMemberListDropsDevicesThatLeft(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	alice := mustLoginAs(t, s, "alice")
	mustJoin(t, s, owner, room.Code, "")
	mustJoin(t, s, alice, room.Code, "")

	mustLeave(t, s, alice)

	got := mustListMembers(t, s, owner, room.Code)
	if len(got) != 1 || got[0].Username != servertest.AdminUsername {
		t.Fatalf("want only the owner's device, got %+v", got)
	}
}

func TestOwnerAndAdministratorsSeeMembersWithoutBeingInTheRoom(t *testing.T) {
	s := servertest.Start(t)
	alice := mustLoginAs(t, s, "alice")
	room := mustCreateRoom(t, s, alice, "alice's room")
	aliceLaptop := mustLoginDevice(t, s, "alice", "alice-password", "laptop")
	mustJoin(t, s, aliceLaptop, room.Code, "")
	admin := mustLogin(t, s)

	for name, sess := range map[string]api.Session{"owner": alice, "administrator": admin} {
		if got := mustListMembers(t, s, sess, room.Code); len(got) != 1 {
			t.Errorf("%s: want 1 member, got %+v", name, got)
		}
	}
}

func TestOutsidersCannotSeeMembers(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	mustJoin(t, s, owner, room.Code, "")
	bob := mustLoginAs(t, s, "bob")

	_, err := listMembers(s, bob, room.Code)

	if code := servertest.ErrorCode(err); code != api.ErrNotRoomMember {
		t.Fatalf("want %s, got %v", api.ErrNotRoomMember, err)
	}
}

func TestMemberListOfUnknownRoomIsNotFound(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)

	_, err := listMembers(s, sess, "ABCD-EFGH")

	if code := servertest.ErrorCode(err); code != api.ErrRoomNotFound {
		t.Fatalf("want %s, got %v", api.ErrRoomNotFound, err)
	}
}

func TestDeviceSeesWhichRoomItIsIn(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "lan party")
	alice := mustLoginAs(t, s, "alice")

	if got := mustMyMembership(t, s, alice); got.Room != nil {
		t.Fatalf("want no room before joining, got %+v", got.Room)
	}
	mustJoin(t, s, alice, room.Code, "")
	if got := mustMyMembership(t, s, alice); got.Room == nil || *got.Room != room {
		t.Fatalf("want room %+v, got %+v", room, got.Room)
	}
	mustLeave(t, s, alice)
	if got := mustMyMembership(t, s, alice); got.Room != nil {
		t.Fatalf("want no room after leaving, got %+v", got.Room)
	}
}

func mustMyMembership(t *testing.T, s *servertest.Server, sess api.Session) api.MyMembership {
	t.Helper()
	var ms api.MyMembership
	if err := s.Do(http.MethodGet, api.PathMembership, sess.AccessToken, nil, &ms); err != nil {
		t.Fatalf("get membership: %v", err)
	}
	return ms
}

func mustListMembers(t *testing.T, s *servertest.Server, sess api.Session, code string) []api.Member {
	t.Helper()
	members, err := listMembers(s, sess, code)
	if err != nil {
		t.Fatalf("list members of %s: %v", code, err)
	}
	return members
}

func listMembers(s *servertest.Server, sess api.Session, code string) ([]api.Member, error) {
	var list api.MemberList
	err := s.Do(http.MethodGet, api.PathRoomMembers(code), sess.AccessToken, nil, &list)
	return list.Members, err
}
