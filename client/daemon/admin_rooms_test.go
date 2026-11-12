package daemon_test

import (
	"testing"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/daemontest"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

func waitForLoggedOutWithNotice(t *testing.T, d *daemontest.Daemon, notice ipc.Notice) {
	t.Helper()
	waitFor(t, func() bool { return mustStatus(t, d).State == ipc.StateLoggedOut })
	if st := mustStatus(t, d); st.Notice != notice {
		t.Fatalf("want notice %q, got %+v", notice, st)
	}
	waitFor(t, func() bool { _, running := d.Engine.Running(); return !running })
}

func aliceID(t *testing.T, admin *daemontest.Daemon) int64 {
	t.Helper()
	users, err := admin.IPC.ListUsers(ctx)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	for _, u := range users {
		if u.Username == "alice" {
			return u.ID
		}
	}
	t.Fatal("alice not listed")
	return 0
}

func TestDisabledUsersDaemonLeavesTheRoomAndLogsOut(t *testing.T) {
	_, owner, alice, _ := roomWithOwnerAndAlice(t, "")

	if _, err := owner.IPC.SetUserDisabled(ctx, aliceID(t, owner), true); err != nil {
		t.Fatalf("disable: %v", err)
	}

	waitForLoggedOutWithNotice(t, alice, ipc.NoticeAccountDisabled)
}

func TestDeletedUsersDaemonLeavesTheRoomAndLogsOut(t *testing.T) {
	_, owner, alice, _ := roomWithOwnerAndAlice(t, "")

	if err := owner.IPC.DeleteUser(ctx, aliceID(t, owner)); err != nil {
		t.Fatalf("delete: %v", err)
	}

	waitForLoggedOutWithNotice(t, alice, ipc.NoticeAccountDeleted)
}

func TestAdministratorManagesRoomsAndSettingsThroughDaemon(t *testing.T) {
	_, owner, alice, room := roomWithOwnerAndAlice(t, "")

	rooms, err := owner.IPC.AllRooms(ctx)
	if err != nil || len(rooms) != 1 || rooms[0].OnlineMembers != 2 || rooms[0].Code != room.Code {
		t.Fatalf("all rooms: %+v, %v", rooms, err)
	}
	st, err := owner.IPC.SetSettings(ctx, api.Settings{RoomsPerUser: 3, RoomCapacity: 10})
	if err != nil || st.RoomCapacity != 10 {
		t.Fatalf("set settings: %+v, %v", st, err)
	}
	if got, err := owner.IPC.Settings(ctx); err != nil || got != st {
		t.Fatalf("settings: %+v, %v", got, err)
	}

	if err := owner.IPC.AdminDeleteRoom(ctx, room.Code); err != nil {
		t.Fatalf("delete room: %v", err)
	}

	waitForIdleWithNotice(t, alice, ipc.NoticeRoomDeleted)
}
