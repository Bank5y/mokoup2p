package main

import (
	"context"
	"fmt"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

// notices explains why the device left a room without asking to.
var notices = map[ipc.Notice]string{
	ipc.NoticeKicked:          "你已被房主移出房间",
	ipc.NoticeRoomDeleted:     "房间已被删除",
	ipc.NoticeAccountDisabled: "账号已被管理员禁用",
}

// cmdKick kicks the member with the given virtual IP out of the current room.
func cmdKick(ctx context.Context, c *ipc.Client, args []string) error {
	if len(args) != 1 {
		return inputError("用法：mokoup2p kick 虚拟IP（见 mokoup2p members）")
	}
	members, err := c.Members(ctx)
	if err != nil {
		return err
	}
	for _, m := range members {
		if m.VirtualIP != args[0] {
			continue
		}
		if m.Self {
			return inputError("不能移出本设备，请使用 mokoup2p leave")
		}
		if err := c.Kick(ctx, m.ID); err != nil {
			return err
		}
		fmt.Printf("已将 %s / %s（%s）移出房间\n", m.Username, m.DeviceName, m.VirtualIP)
		return nil
	}
	return &api.Error{Code: api.ErrMemberNotFound}
}
