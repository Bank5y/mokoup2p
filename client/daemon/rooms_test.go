package daemon_test

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/daemontest"
	"github.com/Bank5y/mokoup2p/client/ipc"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

var roomCodeFormat = regexp.MustCompile(`^[A-HJ-NP-Z2-9]{4}-[A-HJ-NP-Z2-9]{4}$`)

func TestCreateRoomThroughDaemonAppearsInMyRooms(t *testing.T) {
	d := loggedInDaemon(t)

	room, err := d.IPC.CreateRoom(ctx, api.CreateRoomRequest{Name: "开黑", Password: "secret"})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	if !roomCodeFormat.MatchString(room.Code) || room.Name != "开黑" || !room.HasPassword {
		t.Fatalf("unexpected room %+v", room)
	}
	rooms, err := d.IPC.MyRooms(ctx)
	if err != nil {
		t.Fatalf("my rooms: %v", err)
	}
	if len(rooms) != 1 || rooms[0] != room {
		t.Fatalf("want [%+v], got %+v", room, rooms)
	}
}

func TestOwnerManagesRoomThroughDaemon(t *testing.T) {
	d := loggedInDaemon(t)
	room, err := d.IPC.CreateRoom(ctx, api.CreateRoomRequest{Name: "old"})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	name, password := "new", "secret"
	updated, err := d.IPC.UpdateRoom(ctx, room.Code, api.UpdateRoomRequest{Name: &name, Password: &password})
	if err != nil {
		t.Fatalf("update room: %v", err)
	}
	if updated.Name != "new" || !updated.HasPassword || updated.Code != room.Code {
		t.Fatalf("unexpected updated room %+v", updated)
	}
	if err := d.IPC.DeleteRoom(ctx, room.Code); err != nil {
		t.Fatalf("delete room: %v", err)
	}
	if rooms, err := d.IPC.MyRooms(ctx); err != nil || len(rooms) != 0 {
		t.Fatalf("want no rooms after delete, got %+v, %v", rooms, err)
	}
}

func TestRoomLimitErrorIsRelayedByDaemon(t *testing.T) {
	d := loggedInDaemon(t)
	for i := range 5 {
		if _, err := d.IPC.CreateRoom(ctx, api.CreateRoomRequest{Name: fmt.Sprintf("room %d", i)}); err != nil {
			t.Fatalf("create room %d: %v", i, err)
		}
	}

	_, err := d.IPC.CreateRoom(ctx, api.CreateRoomRequest{Name: "one too many"})

	if code := servertest.ErrorCode(err); code != api.ErrRoomLimitReached {
		t.Fatalf("want %s, got %v", api.ErrRoomLimitReached, err)
	}
}

func TestRoomsRequireLogin(t *testing.T) {
	d := daemontest.Start(t)

	_, err := d.IPC.MyRooms(ctx)

	if code := servertest.ErrorCode(err); code != ipc.ErrNotLoggedIn {
		t.Fatalf("want %s, got %v", ipc.ErrNotLoggedIn, err)
	}
}

func TestDaemonRenewsExpiredAccessTokenTransparently(t *testing.T) {
	s := servertest.Start(t)
	d := daemontest.Start(t)
	mustSetServer(t, d, s)
	mustLogin(t, d)

	s.Clock.Advance(16 * time.Minute)

	if _, err := d.IPC.MyRooms(ctx); err != nil {
		t.Fatalf("my rooms after access token expiry: %v", err)
	}
	if _, err := d.IPC.CreateRoom(ctx, api.CreateRoomRequest{Name: "later"}); err != nil {
		t.Fatalf("create room after access token expiry: %v", err)
	}
}

func TestDaemonCanUseRoomsAfterAutomaticLogin(t *testing.T) {
	s := servertest.Start(t)
	d := daemontest.Start(t)
	mustSetServer(t, d, s)
	mustLogin(t, d)

	d.Restart(t)

	if _, err := d.IPC.CreateRoom(ctx, api.CreateRoomRequest{Name: "after restart"}); err != nil {
		t.Fatalf("create room after automatic login: %v", err)
	}
}

func loggedInDaemon(t *testing.T) *daemontest.Daemon {
	t.Helper()
	s := servertest.Start(t)
	d := daemontest.Start(t)
	mustSetServer(t, d, s)
	mustLogin(t, d)
	return d
}
