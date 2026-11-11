package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

// cmdJoin joins a room. Without -p it first tries without a password and
// asks for one only if the room turns out to need it.
func cmdJoin(ctx context.Context, c *ipc.Client, args []string) error {
	fs := flag.NewFlagSet("join", flag.ExitOnError)
	askPassword := fs.Bool("p", false, "输入房间密码")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		return usageError("请提供房间号，例如 mokoup2p join K7QH-3MXP")
	}
	code := fs.Arg(0)
	in := bufio.NewReader(os.Stdin)

	password := ""
	if *askPassword {
		var err error
		if password, err = readPassword(in, "房间密码："); err != nil {
			return err
		}
	}
	st, err := c.Join(ctx, code, password)
	if api.CodeOf(err) == api.ErrRoomPasswordWrong && !*askPassword {
		fmt.Println("该房间需要密码")
		if password, err = readPassword(in, "房间密码："); err != nil {
			return err
		}
		st, err = c.Join(ctx, code, password)
	}
	if err != nil {
		return err
	}
	fmt.Printf("已加入房间：%s（%s）\n", st.Room.Name, st.Room.Code)
	fmt.Printf("本机虚拟 IP：%s（网段 %s）\n", st.Room.VirtualIP, st.Room.Subnet)
	return nil
}

func cmdLeave(ctx context.Context, c *ipc.Client) error {
	before, err := c.Status(ctx)
	if err != nil {
		return err
	}
	if _, err := c.Leave(ctx); err != nil {
		return err
	}
	if before.Room == nil {
		fmt.Println("当前未在房间中")
		return nil
	}
	fmt.Printf("已退出房间：%s（%s）\n", before.Room.Name, before.Room.Code)
	return nil
}
