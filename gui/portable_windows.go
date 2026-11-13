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
// nothing. Problems are shown in a message box (text from the resource
// file), since there is no window yet.
func startPortable(c *ipc.Client) bool {
	if !isPortable() {
		return true
	}
	if serviceInstalled() {
		fail(text("portable.serviceInstalled"))
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
		fail(textf("portable.dataNotWritable", map[string]string{"dir": data}))
		return false
	}
	u, err := user.Current()
	if err != nil {
		fail(textf("portable.noUser", map[string]string{"error": err.Error()}))
		return false
	}
	args := fmt.Sprintf("portable -ipc-user %s -state-dir %s -parent-pid %d",
		u.Uid, syscall.EscapeArg(data), os.Getpid())
	err = shellRunAs(filepath.Join(dir, "mokoup2pd.exe"), args, windows.SW_HIDE)
	switch {
	case errors.Is(err, errElevationCancelled):
		fail(text("portable.elevationCancelled"))
		return false
	case err != nil:
		fail(textf("portable.startFailed", map[string]string{"error": err.Error()}))
		return false
	}
	if !daemonAnswers(c, 20*time.Second) {
		fail(textf("portable.notAnswering", map[string]string{"log": filepath.Join(data, "mokoup2pd.log")}))
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
