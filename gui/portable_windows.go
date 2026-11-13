package main

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/Bank5y/mokoup2p/client/ipc"
)

const serviceName = "mokoup2p"

// startPortable prepares the portable edition before the window opens and
// reports whether the GUI should go on. In the installer edition it does
// nothing. Problems are shown in a message box, since there is no window
// yet.
func startPortable(c *ipc.Client) bool {
	if !isPortable() {
		return true
	}
	if serviceInstalled() {
		fail("本机已安装 mokoup2p，请使用安装版（开始菜单中的 mokoup2p），或先卸载安装版再使用绿色版。")
		return false
	}
	// Already running: this is a second launch of the portable GUI, whose
	// single-instance lock brings the first window back.
	if daemonAnswers(c, 0) {
		return true
	}
	dir := exeDir()
	data := filepath.Join(dir, "data")
	if !writableDir(data) {
		fail(fmt.Sprintf("无法写入数据目录：\n%s\n\n请把整个 mokoup2p 文件夹移到可写的位置（例如桌面或 D:\\）后再运行。", data))
		return false
	}
	u, err := user.Current()
	if err != nil {
		fail("无法确定当前用户：" + err.Error())
		return false
	}
	args := fmt.Sprintf("portable -ipc-user %s -state-dir %s -parent-pid %d",
		u.Uid, syscall.EscapeArg(data), os.Getpid())
	err = shellRunAs(filepath.Join(dir, "mokoup2pd.exe"), args, windows.SW_HIDE)
	switch {
	case errors.Is(err, errElevationCancelled):
		fail("创建虚拟网卡需要管理员权限。已取消，mokoup2p 将退出。")
		return false
	case err != nil:
		fail("无法启动后台服务：" + err.Error())
		return false
	}
	if !daemonAnswers(c, 20*time.Second) {
		fail("后台服务没有启动，详见：\n" + filepath.Join(data, "mokoup2pd.log"))
		return false
	}
	return true
}

// serviceInstalled reports whether the installer edition's service exists.
// It asks only for rights a standard user has.
func serviceInstalled() bool {
	m, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return false
	}
	defer windows.CloseServiceHandle(m)
	name, _ := windows.UTF16PtrFromString(serviceName)
	s, err := windows.OpenService(m, name, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return false
	}
	windows.CloseServiceHandle(s)
	return true
}

func fail(text string) {
	t, _ := windows.UTF16PtrFromString(text)
	title, _ := windows.UTF16PtrFromString("mokoup2p")
	_, _ = windows.MessageBox(0, t, title, windows.MB_OK|windows.MB_ICONERROR)
}

// dependencyMissing reports whether Npcap, which easytier-core needs
// (ADR-0004), is not installed.
func dependencyMissing() bool {
	_, err := os.Stat(filepath.Join(os.Getenv("SystemRoot"), "System32", "Npcap", "Packet.dll"))
	return err != nil
}
