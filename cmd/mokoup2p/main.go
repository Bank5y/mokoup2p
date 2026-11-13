// Command mokoup2p is the command-line front end of the client daemon.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

const usage = `用法：mokoup2p <命令> [参数]

命令：
  server [地址]          查看或设置服务端地址，例如 1.2.3.4:8080 或 https://p2p.example.com
                         （不写协议时按 http，不写端口时按 8080）
  login [-u 用户名]      登录（密码从终端读取）
  status                 查看登录状态
  passwd                 修改密码（旧密码和新密码从终端读取）
  device-name [名称]     查看或修改设备名
  logout                 退出登录
  room create [-p] 名称  创建房间（-p：设置房间密码，从终端读取）
  room list              列出我的房间
  join [-p] 房间号       加入房间（需要时从终端读取房间密码；-p：直接输入）
  leave                  退出当前房间
  members [-w]           列出当前房间的成员（-w：持续显示并实时刷新）
  kick 虚拟IP            把成员移出当前房间（仅房主）
  update --check         检查是否有新版本
  version                查看客户端版本
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	client := ipc.NewClient(ipc.DefaultSocketPath())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	switch os.Args[1] {
	case "server":
		err = cmdServer(ctx, client, os.Args[2:])
	case "login":
		err = cmdLogin(ctx, client, os.Args[2:])
	case "status":
		err = cmdStatus(ctx, client)
	case "passwd":
		err = cmdPasswd(ctx, client)
	case "device-name":
		err = cmdDeviceName(ctx, client, os.Args[2:])
	case "logout":
		err = cmdLogout(ctx, client)
	case "room":
		err = cmdRoom(ctx, client, os.Args[2:])
	case "join":
		err = cmdJoin(ctx, client, os.Args[2:])
	case "leave":
		err = cmdLeave(ctx, client)
	case "members":
		err = cmdMembers(ctx, client, os.Args[2:])
	case "kick":
		err = cmdKick(ctx, client, os.Args[2:])
	case "update":
		err = cmdUpdate(ctx, client, os.Args[2:])
	case "version":
		err = cmdVersion(ctx, client)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "未知命令：%s\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误："+describe(err))
		os.Exit(1)
	}
}

func cmdServer(ctx context.Context, c *ipc.Client, args []string) error {
	var st ipc.Status
	var err error
	if len(args) == 0 {
		st, err = c.Status(ctx)
	} else {
		st, err = c.SetServer(ctx, args[0])
	}
	if err != nil {
		return err
	}
	if st.ServerURL == "" {
		fmt.Println("尚未设置服务端地址")
	} else {
		fmt.Println("服务端地址：" + st.ServerURL)
	}
	return nil
}

func cmdLogin(ctx context.Context, c *ipc.Client, args []string) error {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	username := fs.String("u", "", "用户名")
	_ = fs.Parse(args)

	if st, err := c.Status(ctx); err != nil {
		return err
	} else if st.ServerURL == "" {
		return &api.Error{Code: ipc.ErrServerNotConfigured}
	}
	in := bufio.NewReader(os.Stdin)
	if *username == "" {
		fmt.Print("用户名：")
		line, err := in.ReadString('\n')
		if err != nil {
			return err
		}
		*username = strings.TrimSpace(line)
	}
	password, err := readPassword(in, "密码：")
	if err != nil {
		return err
	}
	st, err := c.Login(ctx, *username, password)
	if err != nil {
		return err
	}
	fmt.Printf("登录成功：%s（设备：%s）\n", st.Username, st.DeviceName)
	return nil
}

func readPassword(in *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		return string(b), err
	}
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		return "", inputError("未能读取输入")
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func cmdPasswd(ctx context.Context, c *ipc.Client) error {
	if st, err := c.Status(ctx); err != nil {
		return err
	} else if st.State == ipc.StateLoggedOut {
		return &api.Error{Code: ipc.ErrNotLoggedIn}
	}
	in := bufio.NewReader(os.Stdin)
	oldPassword, err := readPassword(in, "旧密码：")
	if err != nil {
		return err
	}
	newPassword, err := readPassword(in, "新密码：")
	if err != nil {
		return err
	}
	again, err := readPassword(in, "再次输入新密码：")
	if err != nil {
		return err
	}
	if newPassword != again {
		return inputError("两次输入的新密码不一致")
	}
	if newPassword == "" {
		return inputError("新密码不能为空")
	}
	if err := c.ChangePassword(ctx, oldPassword, newPassword); err != nil {
		if api.CodeOf(err) == api.ErrInvalidCredentials {
			return inputError("旧密码错误")
		}
		return err
	}
	fmt.Println("密码已修改")
	return nil
}

func cmdDeviceName(ctx context.Context, c *ipc.Client, args []string) error {
	var st ipc.Status
	var err error
	if len(args) == 0 {
		st, err = c.Status(ctx)
	} else {
		st, err = c.SetDeviceName(ctx, strings.Join(args, " "))
	}
	if err != nil {
		return err
	}
	fmt.Println("设备名：" + st.DeviceName)
	return nil
}

func cmdLogout(ctx context.Context, c *ipc.Client) error {
	if _, err := c.Logout(ctx); err != nil {
		return err
	}
	fmt.Println("已退出登录")
	return nil
}

func cmdStatus(ctx context.Context, c *ipc.Client) error {
	st, err := c.Status(ctx)
	if err != nil {
		return err
	}
	server := st.ServerURL
	if server == "" {
		server = "（未设置）"
	}
	fmt.Println("服务端：" + server)
	fmt.Println("版本：" + st.Version)
	fmt.Printf("设备：%s（%s）\n", st.DeviceName, st.DeviceID)
	if text, ok := notices[st.Notice]; ok {
		fmt.Println("提示：" + text)
	}
	switch st.State {
	case ipc.StateIdle:
		role := ""
		if st.IsAdmin {
			role = "，管理员"
		}
		fmt.Printf("状态：已登录（%s%s），未在房间中\n", st.Username, role)
	case ipc.StateJoining:
		fmt.Printf("状态：已登录（%s），正在加入房间…\n", st.Username)
	case ipc.StateInRoom:
		fmt.Printf("状态：已登录（%s），在房间中\n", st.Username)
		fmt.Printf("房间：%s（%s）\n", st.Room.Name, st.Room.Code)
		fmt.Printf("本机虚拟 IP：%s（网段 %s）\n", st.Room.VirtualIP, st.Room.Subnet)
	default:
		fmt.Println("状态：未登录")
		return nil
	}
	if st.Connected {
		fmt.Println("实时连接：已连接")
	} else {
		fmt.Println("实时连接：已断开，正在重连…")
	}
	return nil
}

var messages = map[api.ErrorCode]string{
	api.ErrBadRequest:              "请求无效",
	api.ErrInvalidCredentials:      "用户名或密码错误",
	api.ErrAccountDisabled:         "账号已被禁用",
	api.ErrTokenInvalid:            "登录状态已失效，请重新登录",
	ipc.ErrServerNotConfigured:     "尚未设置服务端地址，请先运行 mokoup2p server <地址>",
	ipc.ErrServerUnreachable:       "无法连接服务端",
	ipc.ErrServerBadResponse:       "服务端响应异常，请检查服务端地址",
	ipc.ErrNotLoggedIn:             "尚未登录，请先运行 mokoup2p login",
	api.ErrRoomNotFound:            "房间不存在",
	api.ErrRoomLimitReached:        "房间数已达上限",
	api.ErrNotRoomOwner:            "只有房主可以这样做",
	api.ErrRoomPasswordWrong:       "房间密码错误",
	api.ErrRoomFull:                "房间已满",
	api.ErrInOtherRoom:             "本设备已在其他房间中，请先退出",
	ipc.ErrInRoom:                  "本设备已在房间中，请先运行 mokoup2p leave 退出",
	ipc.ErrEngineFailed:            "网络组件启动失败，已退出房间",
	ipc.ErrEngineDependencyMissing: "缺少网络组件依赖，已退出房间（Windows 请先安装 Npcap：https://npcap.com）",
	ipc.ErrNotInRoom:               "当前未在房间中",
	ipc.ErrUpdateCheckFailed:       "检查更新失败，请检查网络能否访问 GitHub",
	api.ErrNotRoomMember:           "只有房间成员、房主和管理员可以查看成员",
	api.ErrMemberNotFound:          "该设备不在这个房间中",
}

// inputError is a mistake in what the user typed, reported as is.
type inputError string

func (e inputError) Error() string { return string(e) }

func describe(err error) string {
	var inErr inputError
	if errors.As(err, &inErr) {
		return string(inErr)
	}
	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		if msg, ok := messages[apiErr.Code]; ok {
			if apiErr.Code == api.ErrBadRequest {
				return msg + "：" + apiErr.Message
			}
			return msg
		}
		return apiErr.Message
	}
	return "无法连接后台服务（mokoup2pd 是否在运行？）：" + err.Error()
}

func cmdVersion(ctx context.Context, c *ipc.Client) error {
	st, err := c.Status(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("mokoup2p %s（%s）\n", st.Version, st.Edition)
	return nil
}

// cmdUpdate only checks: installing an update is done from the GUI (Windows)
// or with the package manager / a new download (Linux).
func cmdUpdate(ctx context.Context, c *ipc.Client, args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	_ = fs.Bool("check", true, "check for a newer release (the only action)")
	_ = fs.Parse(args)
	info, err := c.CheckUpdate(ctx)
	if err != nil {
		return err
	}
	fmt.Println("当前版本：" + info.CurrentVersion)
	switch {
	case info.LatestVersion == "":
		fmt.Println("还没有发布任何正式版本")
	case !info.Available:
		fmt.Printf("已是最新（最新正式版 %s）\n", info.LatestVersion)
	default:
		fmt.Printf("发现新版本：%s\n", info.LatestVersion)
		fmt.Println("发布说明：" + info.ReleaseURL)
		if info.AssetURL != "" {
			fmt.Println("下载：" + info.AssetURL)
		}
		if info.Ignored {
			fmt.Println("（这个版本已在 GUI 中设为忽略）")
		}
	}
	return nil
}
