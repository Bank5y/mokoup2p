package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"
)

// afterUpdateFlag runs the GUI executable as the post-update helper:
//
//	mokoup2p-update-helper.exe -after-update <installer handle> <GUI path>
const afterUpdateFlag = "-after-update"

// startInstaller runs the installer elevated and silent, then starts the
// helper: a copy of this executable (the installer replaces the original)
// running as this user, unelevated, that holds an inherited handle to the
// installer process. Relaunching from the elevated installer could not do
// that when a standard user typed an administrator's password into UAC.
func startInstaller(installer string) error {
	h, err := shellRunAsProcess(installer, "/S", windows.SW_SHOWNORMAL, true)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	gui, err := os.Executable()
	if err != nil {
		log.Printf("update: helper: %v", err)
		return nil // the installer runs anyway; only the reopening is lost
	}
	helper := filepath.Join(updateDir(), "mokoup2p-update-helper.exe")
	if err := copyFile(gui, helper); err != nil {
		log.Printf("update: helper: %v", err)
		return nil
	}
	if err := windows.SetHandleInformation(h, windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT); err != nil {
		log.Printf("update: helper: %v", err)
		return nil
	}
	cmd := exec.Command(helper, afterUpdateFlag, strconv.FormatUint(uint64(h), 10), gui)
	cmd.SysProcAttr = &syscall.SysProcAttr{AdditionalInheritedHandles: []syscall.Handle{syscall.Handle(h)}}
	if err := cmd.Start(); err != nil {
		log.Printf("update: helper: %v", err)
	}
	return nil
}

// afterUpdate is the helper's whole job; it reports whether os.Args asked
// for it. It waits for the installer, then reopens the GUI, first saying so
// if the install failed (the old version is then still in place).
func afterUpdate() bool {
	if len(os.Args) != 4 || os.Args[1] != afterUpdateFlag {
		return false
	}
	n, err := strconv.ParseUint(os.Args[2], 10, 64)
	if err != nil {
		return true
	}
	h := windows.Handle(n)
	gui := os.Args[3]
	_, _ = windows.WaitForSingleObject(h, windows.INFINITE)
	var code uint32
	_ = windows.GetExitCodeProcess(h, &code)
	windows.CloseHandle(h)
	if code != 0 {
		fail(textf("update.installFailed", map[string]string{"code": fmt.Sprint(code)}))
	}
	if err := exec.Command(gui).Start(); err != nil {
		fail(text("update.reopenFailed"))
	}
	return true
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
