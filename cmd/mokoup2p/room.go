package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

func cmdRoom(ctx context.Context, c *ipc.Client, args []string) error {
	if len(args) == 0 {
		return usageError("缺少子命令：room create 或 room list")
	}
	switch args[0] {
	case "create":
		return cmdRoomCreate(ctx, c, args[1:])
	case "list":
		return cmdRoomList(ctx, c)
	default:
		return usageError("未知子命令：room " + args[0])
	}
}

func usageError(msg string) error {
	return &api.Error{Code: api.ErrBadRequest, Message: msg}
}

func cmdRoomCreate(ctx context.Context, c *ipc.Client, args []string) error {
	fs := flag.NewFlagSet("room create", flag.ExitOnError)
	withPassword := fs.Bool("p", false, "设置房间密码")
	_ = fs.Parse(args)
	name := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if name == "" {
		return usageError("请提供房间名称，例如 mokoup2p room create 开黑")
	}
	req := api.CreateRoomRequest{Name: name}
	if *withPassword {
		password, err := readPassword(bufio.NewReader(os.Stdin), "房间密码：")
		if err != nil {
			return err
		}
		req.Password = password
	}
	room, err := c.CreateRoom(ctx, req)
	if err != nil {
		return err
	}
	fmt.Printf("已创建房间：%s\n房间号：%s\n", room.Name, room.Code)
	if room.HasPassword {
		fmt.Println("已设置房间密码")
	}
	return nil
}

func cmdRoomList(ctx context.Context, c *ipc.Client) error {
	rooms, err := c.MyRooms(ctx)
	if err != nil {
		return err
	}
	if len(rooms) == 0 {
		fmt.Println("你还没有创建房间")
		return nil
	}
	for _, r := range rooms {
		password := "无密码"
		if r.HasPassword {
			password = "有密码"
		}
		fmt.Printf("%s  %s（%s，网段 %s）\n", r.Code, r.Name, password, r.Subnet)
	}
	return nil
}
