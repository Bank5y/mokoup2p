package main

import (
	"errors"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// shellRunAs starts exe with args through the UAC prompt ("runas") and
// returns once it has started.
func shellRunAs(exe, args string, show int32) error {
	h, err := shellRunAsProcess(exe, args, show, false)
	if h != 0 {
		windows.CloseHandle(h)
	}
	return err
}

// shellExecuteInfo is SHELLEXECUTEINFOW, which x/sys/windows lacks.
type shellExecuteInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         windows.HWND
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     windows.Handle
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    windows.Handle
	dwHotKey     uint32
	hIcon        windows.Handle
	hProcess     windows.Handle
}

const (
	seeMaskNoCloseProcess = 0x00000040
	seeMaskNoAsync        = 0x00000100
)

var procShellExecuteExW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

// shellRunAsProcess is shellRunAs that, with wantHandle, also returns a
// handle to the elevated process (the caller closes it).
func shellRunAsProcess(exe, args string, show int32, wantHandle bool) (windows.Handle, error) {
	verb, _ := windows.UTF16PtrFromString("runas")
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return 0, err
	}
	params, _ := windows.UTF16PtrFromString(args)
	dir, _ := windows.UTF16PtrFromString(filepath.Dir(exe))
	info := shellExecuteInfo{
		fMask:        seeMaskNoAsync,
		lpVerb:       verb,
		lpFile:       file,
		lpParameters: params,
		lpDirectory:  dir,
		nShow:        show,
	}
	if wantHandle {
		info.fMask |= seeMaskNoCloseProcess
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	ok, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		if errors.Is(callErr, windows.ERROR_CANCELLED) {
			return 0, errElevationCancelled
		}
		return 0, callErr
	}
	return info.hProcess, nil
}
