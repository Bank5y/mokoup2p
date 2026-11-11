package server_test

import (
	"fmt"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
	"testing"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

// Room codes use upper-case letters and digits without 0, O, 1 and I.
var roomCodeFormat = regexp.MustCompile(`^[A-HJ-NP-Z2-9]{4}-[A-HJ-NP-Z2-9]{4}$`)

func TestCreateRoomReturnsShareableRoomCode(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)

	var room api.Room
	err := s.Do(http.MethodPost, api.PathRooms, sess.AccessToken, api.CreateRoomRequest{Name: "开黑"}, &room)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	if !roomCodeFormat.MatchString(room.Code) {
		t.Fatalf("room code %q is not XXXX-XXXX from the unambiguous alphabet", room.Code)
	}
	if room.Name != "开黑" || room.HasPassword || room.Owner != servertest.AdminUsername {
		t.Fatalf("unexpected room %+v", room)
	}
}

func TestEachRoomGetsItsOwnPrivateSubnetAwayFromHomeNetworks(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)
	homeNetworks := []netip.Prefix{
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("10.0.0.0/16"),
		netip.MustParsePrefix("10.1.0.0/16"),
		netip.MustParsePrefix("172.16.0.0/12"),
	}

	seenSubnets := map[netip.Prefix]bool{}
	seenCodes := map[string]bool{}
	for i := range 5 {
		room := mustCreateRoom(t, s, sess, fmt.Sprintf("room %d", i))

		subnet, err := netip.ParsePrefix(room.Subnet)
		if err != nil || subnet.Bits() != 24 || !subnet.Addr().IsPrivate() || subnet.Masked() != subnet {
			t.Fatalf("subnet %q is not a private /24", room.Subnet)
		}
		for _, home := range homeNetworks {
			if home.Overlaps(subnet) {
				t.Fatalf("subnet %s overlaps common home network %s", subnet, home)
			}
		}
		if seenSubnets[subnet] || seenCodes[room.Code] {
			t.Fatalf("room %+v reuses a subnet or code", room)
		}
		seenSubnets[subnet] = true
		seenCodes[room.Code] = true
	}
}

func TestSixthRoomExceedsDefaultRoomLimit(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)
	for i := range 5 {
		mustCreateRoom(t, s, sess, fmt.Sprintf("room %d", i))
	}

	err := s.Do(http.MethodPost, api.PathRooms, sess.AccessToken, api.CreateRoomRequest{Name: "one too many"}, nil)

	if code := servertest.ErrorCode(err); code != api.ErrRoomLimitReached {
		t.Fatalf("want %s, got %v", api.ErrRoomLimitReached, err)
	}
}

func TestRoomLimitIsPerUser(t *testing.T) {
	s := servertest.Start(t)
	admin := mustLogin(t, s)
	for i := range 5 {
		mustCreateRoom(t, s, admin, fmt.Sprintf("room %d", i))
	}
	alice := mustLoginAs(t, s, "alice")

	mustCreateRoom(t, s, alice, "alice's room")
}

func TestMyRoomsListsOnlyRoomsIOwn(t *testing.T) {
	s := servertest.Start(t)
	admin := mustLogin(t, s)
	alice := mustLoginAs(t, s, "alice")
	first := mustCreateRoom(t, s, alice, "first")
	second := mustCreateRoom(t, s, alice, "second")
	mustCreateRoom(t, s, admin, "not alice's")

	var list api.RoomList
	if err := s.Do(http.MethodGet, api.PathRooms, alice.AccessToken, nil, &list); err != nil {
		t.Fatalf("list rooms: %v", err)
	}

	if len(list.Rooms) != 2 || list.Rooms[0] != first || list.Rooms[1] != second {
		t.Fatalf("want [%+v %+v], got %+v", first, second, list.Rooms)
	}
}

func TestMyRoomsIsEmptyListWhenIOwnNone(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)

	var list api.RoomList
	if err := s.Do(http.MethodGet, api.PathRooms, sess.AccessToken, nil, &list); err != nil {
		t.Fatalf("list rooms: %v", err)
	}

	if list.Rooms == nil || len(list.Rooms) != 0 {
		t.Fatalf("want empty list, got %#v", list.Rooms)
	}
}

func TestRoomCodeInputIgnoresCaseAndSeparator(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)
	room := mustCreateRoom(t, s, sess, "lan party")
	typed := strings.ToLower(strings.ReplaceAll(room.Code, "-", ""))

	var got api.Room
	if err := s.Do(http.MethodGet, api.PathRoom(typed), sess.AccessToken, nil, &got); err != nil {
		t.Fatalf("get room %q: %v", typed, err)
	}

	if got != room {
		t.Fatalf("want %+v, got %+v", room, got)
	}
}

func TestUnknownRoomCodeIsNotFound(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)

	for _, code := range []string{"ABCD-EFGH", "not-a-code"} {
		err := s.Do(http.MethodGet, api.PathRoom(code), sess.AccessToken, nil, nil)
		if c := servertest.ErrorCode(err); c != api.ErrRoomNotFound {
			t.Fatalf("%s: want %s, got %v", code, api.ErrRoomNotFound, err)
		}
	}
}

func TestCreateRoomWithPassword(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)

	var room api.Room
	err := s.Do(http.MethodPost, api.PathRooms, sess.AccessToken, api.CreateRoomRequest{Name: "private", Password: "secret"}, &room)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	if !room.HasPassword {
		t.Fatalf("want room with password, got %+v", room)
	}
}

func TestCreateRoomRequiresName(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)

	err := s.Do(http.MethodPost, api.PathRooms, sess.AccessToken, api.CreateRoomRequest{Name: "  "}, nil)

	if code := servertest.ErrorCode(err); code != api.ErrBadRequest {
		t.Fatalf("want %s, got %v", api.ErrBadRequest, err)
	}
}

func TestRoomResponsesNeverContainNetworkKey(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)
	room := mustCreateRoom(t, s, sess, "lan party")

	var created, got map[string]any
	var list struct{ Rooms []map[string]any }
	if err := s.Do(http.MethodPost, api.PathRooms, sess.AccessToken, api.CreateRoomRequest{Name: "another"}, &created); err != nil {
		t.Fatal(err)
	}
	if err := s.Do(http.MethodGet, api.PathRoom(room.Code), sess.AccessToken, nil, &got); err != nil {
		t.Fatal(err)
	}
	if err := s.Do(http.MethodGet, api.PathRooms, sess.AccessToken, nil, &list); err != nil {
		t.Fatal(err)
	}

	for _, body := range append([]map[string]any{created, got}, list.Rooms...) {
		for field := range body {
			if strings.Contains(strings.ToLower(field), "key") || strings.Contains(strings.ToLower(field), "secret") {
				t.Fatalf("room response exposes field %q: %v", field, body)
			}
		}
	}
}

func TestOwnerRenamesRoom(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)
	room := mustCreateRoom(t, s, sess, "old name")

	updated := mustUpdateRoom(t, s, sess, room.Code, api.UpdateRoomRequest{Name: ptr("new name")})

	if updated.Name != "new name" || updated.Code != room.Code || updated.Subnet != room.Subnet {
		t.Fatalf("want only the name changed from %+v, got %+v", room, updated)
	}
	if got := mustGetRoom(t, s, sess, room.Code); got != updated {
		t.Fatalf("rename not persisted: %+v", got)
	}
}

func TestOwnerSetsChangesAndRemovesRoomPassword(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)
	room := mustCreateRoom(t, s, sess, "lan party")

	if r := mustUpdateRoom(t, s, sess, room.Code, api.UpdateRoomRequest{Password: ptr("first")}); !r.HasPassword || r.Name != "lan party" {
		t.Fatalf("after setting password: %+v", r)
	}
	if r := mustUpdateRoom(t, s, sess, room.Code, api.UpdateRoomRequest{Password: ptr("second")}); !r.HasPassword {
		t.Fatalf("after changing password: %+v", r)
	}
	if r := mustUpdateRoom(t, s, sess, room.Code, api.UpdateRoomRequest{Password: ptr("")}); r.HasPassword {
		t.Fatalf("after removing password: %+v", r)
	}
	if got := mustGetRoom(t, s, sess, room.Code); got.HasPassword {
		t.Fatalf("password removal not persisted: %+v", got)
	}
}

func TestOwnerDeletesRoom(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)
	room := mustCreateRoom(t, s, sess, "lan party")

	if err := s.Do(http.MethodDelete, api.PathRoom(room.Code), sess.AccessToken, nil, nil); err != nil {
		t.Fatalf("delete room: %v", err)
	}

	err := s.Do(http.MethodGet, api.PathRoom(room.Code), sess.AccessToken, nil, nil)
	if code := servertest.ErrorCode(err); code != api.ErrRoomNotFound {
		t.Fatalf("want %s after delete, got %v", api.ErrRoomNotFound, err)
	}
}

func TestDeletingRoomFreesASlotUnderTheLimit(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)
	var rooms []api.Room
	for i := range 5 {
		rooms = append(rooms, mustCreateRoom(t, s, sess, fmt.Sprintf("room %d", i)))
	}

	if err := s.Do(http.MethodDelete, api.PathRoom(rooms[0].Code), sess.AccessToken, nil, nil); err != nil {
		t.Fatalf("delete room: %v", err)
	}

	mustCreateRoom(t, s, sess, "replacement")
}

func TestNonOwnerCannotChangeOrDeleteRoom(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "admin's room")
	alice := mustLoginAs(t, s, "alice")

	errs := map[string]error{
		"rename":   s.Do(http.MethodPatch, api.PathRoom(room.Code), alice.AccessToken, api.UpdateRoomRequest{Name: ptr("mine now")}, nil),
		"password": s.Do(http.MethodPatch, api.PathRoom(room.Code), alice.AccessToken, api.UpdateRoomRequest{Password: ptr("")}, nil),
		"delete":   s.Do(http.MethodDelete, api.PathRoom(room.Code), alice.AccessToken, nil, nil),
	}

	for action, err := range errs {
		if code := servertest.ErrorCode(err); code != api.ErrNotRoomOwner {
			t.Errorf("%s: want %s, got %v", action, api.ErrNotRoomOwner, err)
		}
	}
	if got := mustGetRoom(t, s, owner, room.Code); got != room {
		t.Fatalf("room changed by non-owner: %+v", got)
	}
}

func TestChangingUnknownRoomIsNotFound(t *testing.T) {
	s := servertest.Start(t)
	sess := mustLogin(t, s)

	errs := []error{
		s.Do(http.MethodPatch, api.PathRoom("ABCD-EFGH"), sess.AccessToken, api.UpdateRoomRequest{Name: ptr("x")}, nil),
		s.Do(http.MethodDelete, api.PathRoom("ABCD-EFGH"), sess.AccessToken, nil, nil),
	}

	for _, err := range errs {
		if code := servertest.ErrorCode(err); code != api.ErrRoomNotFound {
			t.Errorf("want %s, got %v", api.ErrRoomNotFound, err)
		}
	}
}

func mustUpdateRoom(t *testing.T, s *servertest.Server, sess api.Session, code string, req api.UpdateRoomRequest) api.Room {
	t.Helper()
	var room api.Room
	if err := s.Do(http.MethodPatch, api.PathRoom(code), sess.AccessToken, req, &room); err != nil {
		t.Fatalf("update room: %v", err)
	}
	return room
}

func mustGetRoom(t *testing.T, s *servertest.Server, sess api.Session, code string) api.Room {
	t.Helper()
	var room api.Room
	if err := s.Do(http.MethodGet, api.PathRoom(code), sess.AccessToken, nil, &room); err != nil {
		t.Fatalf("get room: %v", err)
	}
	return room
}

func ptr[T any](v T) *T { return &v }

func mustLoginAs(t *testing.T, s *servertest.Server, username string) api.Session {
	t.Helper()
	s.AddUser(t, username, username+"-password")
	sess, err := s.Login(username, username+"-password", username+"-dev")
	if err != nil {
		t.Fatalf("login %s: %v", username, err)
	}
	return sess
}

func mustCreateRoom(t *testing.T, s *servertest.Server, sess api.Session, name string) api.Room {
	t.Helper()
	var room api.Room
	if err := s.Do(http.MethodPost, api.PathRooms, sess.AccessToken, api.CreateRoomRequest{Name: name}, &room); err != nil {
		t.Fatalf("create room %q: %v", name, err)
	}
	return room
}
