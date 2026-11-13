package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/Bank5y/mokoup2p/client/ipc"
)

// portableMarker, next to the GUI executable, marks the portable edition:
// no service; the GUI starts the daemon itself and keeps state in data\.
const portableMarker = "portable"

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

func isPortable() bool {
	_, err := os.Stat(filepath.Join(exeDir(), portableMarker))
	return err == nil
}

// daemonAnswers reports whether a daemon is listening on the IPC endpoint.
func daemonAnswers(c *ipc.Client, wait time.Duration) bool {
	deadline := time.Now().Add(wait)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := c.Status(ctx)
		cancel()
		if err == nil {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// writableDir creates dir if needed and checks that files can be written
// in it.
func writableDir(dir string) bool {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false
	}
	f, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(f.Name())
	return true
}

// webviewDataDir keeps the portable edition's WebView2 profile in data\
// next to the executables, so the portable edition writes nothing under
// %APPDATA%. Empty (the Wails default) otherwise.
func webviewDataDir() string {
	if !isPortable() {
		return ""
	}
	return filepath.Join(exeDir(), "data", "webview")
}
