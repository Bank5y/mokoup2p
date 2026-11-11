package server_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

func TestRelayNetworksListsEveryRoomsNetwork(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	a := mustJoin(t, s, owner, mustCreateRoom(t, s, owner, "a").Code, "")
	mustLeave(t, s, owner)
	b := mustJoin(t, s, owner, mustCreateRoom(t, s, owner, "b").Code, "")

	got := mustRelayNetworks(t, s)

	want := []string{a.Network.NetworkName, b.Network.NetworkName}
	slices.Sort(want)
	if !slices.Equal(got.Networks, want) {
		t.Fatalf("want networks %v, got %v", want, got.Networks)
	}
}

func TestRelayNetworksIncludeRoomsNobodyIsIn(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	mustCreateRoom(t, s, owner, "empty")

	if got := mustRelayNetworks(t, s); len(got.Networks) != 1 {
		t.Fatalf("want the empty room's network listed, got %v", got.Networks)
	}
}

func TestRelayNetworksDropDeletedRooms(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "gone")
	if err := s.Do(http.MethodDelete, api.PathRoom(room.Code), owner.AccessToken, nil, nil); err != nil {
		t.Fatalf("delete room: %v", err)
	}

	got := mustRelayNetworks(t, s)
	if got.Networks == nil || len(got.Networks) != 0 {
		t.Fatalf("want an empty list after deleting the only room, got %#v", got.Networks)
	}
}

func TestRelayNetworksSurviveRoomPasswordChange(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)
	room := mustCreateRoom(t, s, owner, "a")
	before := mustRelayNetworks(t, s)

	mustUpdateRoom(t, s, owner, room.Code, api.UpdateRoomRequest{Password: ptr("new secret")})

	if after := mustRelayNetworks(t, s); !slices.Equal(after.Networks, before.Networks) {
		t.Fatalf("password change must not touch the relay whitelist: before %v, after %v", before.Networks, after.Networks)
	}
}

func TestRelayNetworksNeedTheRelayToken(t *testing.T) {
	s := servertest.Start(t)
	owner := mustLogin(t, s)

	for name, token := range map[string]string{
		"no token":            "",
		"wrong token":         "not-the-relay-token",
		"device access token": owner.AccessToken,
	} {
		err := s.Do(http.MethodGet, api.PathRelayNetworks, token, nil, nil)
		if code := servertest.ErrorCode(err); code != api.ErrTokenInvalid {
			t.Errorf("%s: want %s, got %v", name, api.ErrTokenInvalid, err)
		}
	}
}

func mustRelayNetworks(t *testing.T, s *servertest.Server) api.RelayNetworks {
	t.Helper()
	var got api.RelayNetworks
	if err := s.Do(http.MethodGet, api.PathRelayNetworks, servertest.RelayToken, nil, &got); err != nil {
		t.Fatalf("relay networks: %v", err)
	}
	return got
}
