package server_test

import (
	"net/http"
	"testing"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

func mustAdminRooms(t *testing.T, s *servertest.Server, sess api.Session) []api.AdminRoom {
	t.Helper()
	var list api.AdminRoomList
	if err := s.Do(http.MethodGet, api.PathAdminRooms, sess.AccessToken, nil, &list); err != nil {
		t.Fatalf("list all rooms: %v", err)
	}
	return list.Rooms
}

func mustSettings(t *testing.T, s *servertest.Server, sess api.Session) api.Settings {
	t.Helper()
	var st api.Settings
	if err := s.Do(http.MethodGet, api.PathAdminSettings, sess.AccessToken, nil, &st); err != nil {
		t.Fatalf("get settings: %v", err)
	}
	return st
}

func putSettings(s *servertest.Server, sess api.Session, st api.Settings) error {
	return s.Do(http.MethodPut, api.PathAdminSettings, sess.AccessToken, st, nil)
}

func TestAdministratorListsAllRoomsWithOwnerAndOnlineMembers(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)
	alice := mustLoginAs(t, s, "alice")
	busy := mustCreateRoom(t, s, alice, "busy")
	mustCreateRoom(t, s, alice, "empty")
	mustJoin(t, s, alice, busy.Code, "")
	mustJoin(t, s, admin, busy.Code, "")

	rooms := mustAdminRooms(t, s, admin)

	got := map[string]api.AdminRoom{}
	for _, r := range rooms {
		got[r.Name] = r
	}
	if len(rooms) != 2 || got["busy"].Owner != "alice" || got["busy"].OnlineMembers != 2 ||
		got["empty"].OnlineMembers != 0 || got["busy"].Code != busy.Code {
		t.Fatalf("unexpected rooms %+v", rooms)
	}
}

func TestOnlyAdministratorsSeeAllRoomsAndSettings(t *testing.T) {
	s := servertest.Start(t)
	alice := mustLoginAs(t, s, "alice")

	for _, path := range []string{api.PathAdminRooms, api.PathAdminSettings} {
		err := s.Do(http.MethodGet, path, alice.AccessToken, nil, nil)
		if code := servertest.ErrorCode(err); code != api.ErrForbidden {
			t.Fatalf("GET %s: want %s, got %v", path, api.ErrForbidden, err)
		}
	}
}

func TestAdministratorDeletesAnyRoom(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)
	alice := mustLoginAs(t, s, "alice")
	room := mustCreateRoom(t, s, alice, "lan party")
	aliceEvents := s.MustConnectEvents(t, alice)
	mustJoin(t, s, alice, room.Code, "")

	if err := s.Do(http.MethodDelete, api.PathAdminRoom(room.Code), admin.AccessToken, nil, nil); err != nil {
		t.Fatalf("delete: %v", err)
	}

	aliceEvents.Expect(t, api.EventRoomDeleted, nil)
	if rooms := mustAdminRooms(t, s, admin); len(rooms) != 0 {
		t.Fatalf("room should be gone, got %+v", rooms)
	}
}

func TestDeletingAnUnknownRoomFails(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)

	err := s.Do(http.MethodDelete, api.PathAdminRoom("AAAA-AAAA"), admin.AccessToken, nil, nil)

	if code := servertest.ErrorCode(err); code != api.ErrRoomNotFound {
		t.Fatalf("want %s, got %v", api.ErrRoomNotFound, err)
	}
}

func TestSettingsHaveDefaults(t *testing.T) {
	s := servertest.Start(t)

	st := mustSettings(t, s, adminSession(t, s))

	if st.RoomsPerUser != 5 || st.RoomCapacity != 32 {
		t.Fatalf("want defaults 5 / 32, got %+v", st)
	}
}

func TestChangedRoomLimitAppliesToLaterRooms(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)
	alice := mustLoginAs(t, s, "alice")
	mustCreateRoom(t, s, alice, "first")

	if err := putSettings(s, admin, api.Settings{RoomsPerUser: 1, RoomCapacity: 32}); err != nil {
		t.Fatalf("put settings: %v", err)
	}

	err := s.Do(http.MethodPost, api.PathRooms, alice.AccessToken, api.CreateRoomRequest{Name: "second"}, nil)
	if code := servertest.ErrorCode(err); code != api.ErrRoomLimitReached {
		t.Fatalf("want %s, got %v", api.ErrRoomLimitReached, err)
	}
	if st := mustSettings(t, s, admin); st.RoomsPerUser != 1 {
		t.Fatalf("setting not stored: %+v", st)
	}
}

func TestChangedRoomCapacityAppliesToLaterJoins(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)
	room := mustCreateRoom(t, s, admin, "lan party")
	mustJoin(t, s, admin, room.Code, "")

	if err := putSettings(s, admin, api.Settings{RoomsPerUser: 5, RoomCapacity: 1}); err != nil {
		t.Fatalf("put settings: %v", err)
	}

	_, err := join(s, mustLoginAs(t, s, "alice"), room.Code, "")
	if code := servertest.ErrorCode(err); code != api.ErrRoomFull {
		t.Fatalf("want %s, got %v", api.ErrRoomFull, err)
	}
}

func TestSettingsOutOfRangeAreRejected(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)

	for _, st := range []api.Settings{{RoomsPerUser: 0, RoomCapacity: 32}, {RoomsPerUser: 5, RoomCapacity: 0}, {RoomsPerUser: 5, RoomCapacity: 255}} {
		if code := servertest.ErrorCode(putSettings(s, admin, st)); code != api.ErrBadRequest {
			t.Fatalf("%+v: want %s, got %s", st, api.ErrBadRequest, code)
		}
	}
}

func TestDisablingAUserTellsItsDevices(t *testing.T) {
	s := servertest.Start(t)
	alice := mustLoginAs(t, s, "alice")
	aliceEvents := s.MustConnectEvents(t, alice)

	s.DisableUser(t, "alice")

	aliceEvents.Expect(t, api.EventAccountDisabled, nil)
}

func TestDeletingAUserDisconnectsItsDevicesAndDeletesItsRooms(t *testing.T) {
	s := servertest.Start(t)
	admin := adminSession(t, s)
	adminEvents := s.MustConnectEvents(t, admin)
	alice := mustLoginAs(t, s, "alice")
	aliceEvents := s.MustConnectEvents(t, alice)
	aliceRoom := mustCreateRoom(t, s, alice, "alice's room")
	mustJoin(t, s, admin, aliceRoom.Code, "")
	mustJoin(t, s, alice, aliceRoom.Code, "")
	var aliceID int64
	var users []api.AdminUser
	if err := s.Do(http.MethodGet, api.PathAdminUsers, admin.AccessToken, nil, &users); err != nil {
		t.Fatal(err)
	}
	for _, u := range users {
		if u.Username == "alice" {
			aliceID = u.ID
		}
	}

	if err := s.Do(http.MethodDelete, api.PathAdminUser(aliceID), admin.AccessToken, nil, nil); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	aliceEvents.Expect(t, api.EventAccountDeleted, nil)
	var ev api.RoomEvent
	adminEvents.Expect(t, api.EventRoomDeleted, &ev)
	if ev.Room != aliceRoom.Code {
		t.Fatalf("room_deleted for %q, want %q", ev.Room, aliceRoom.Code)
	}
	if got := mustMyMembership(t, s, admin); got.Room != nil {
		t.Fatalf("admin should be in no room, got %+v", got.Room)
	}
}
