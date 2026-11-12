package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/Bank5y/mokoup2p/client/ipc"
)

// cmdMembers lists the members of the current room. With -w it keeps
// running and prints the list again whenever it changes.
func cmdMembers(ctx context.Context, c *ipc.Client, args []string) error {
	fs := flag.NewFlagSet("members", flag.ExitOnError)
	watch := fs.Bool("w", false, "持续显示，成员变化时自动刷新（Ctrl+C 结束）")
	_ = fs.Parse(args)

	if !*watch {
		members, err := c.Members(ctx)
		if err != nil {
			return err
		}
		printMembers(members)
		return nil
	}

	// Watching runs until interrupted, so it does not use ctx's timeout.
	ctx = context.Background()
	st, err := c.Status(ctx)
	if err != nil {
		return err
	}
	for {
		if st.Room == nil {
			fmt.Println("已不在房间中")
			return nil
		}
		members, err := c.Members(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("房间：%s（%s）\n", st.Room.Name, st.Room.Code)
		if !st.Connected {
			fmt.Println("与服务端的连接已断开，正在重连…")
		}
		printMembers(members)
		fmt.Println()
		before := st
		for st.Revision == before.Revision {
			if st, err = c.WaitStatus(ctx, before.Revision); err != nil {
				return err
			}
		}
	}
}

func printMembers(members []ipc.Member) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "用户名 / 设备名\t虚拟 IP\t")
	for _, m := range members {
		self := ""
		if m.Self {
			self = "（本机）"
		}
		fmt.Fprintf(w, "%s / %s%s\t%s\t\n", m.Username, m.DeviceName, self, m.VirtualIP)
	}
	w.Flush()
	fmt.Printf("共 %d 台设备\n", len(members))
}
