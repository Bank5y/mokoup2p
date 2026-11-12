// Command mokoup2pd is the mokoup2p client background service.
//
// Environment:
//
//	MOKOUP2P_STATE_DIR  saved state (default: $XDG_CONFIG_HOME/mokoup2p)
//	MOKOUP2P_SOCKET     IPC socket (default: see ipc.DefaultSocketPath)
//	MOKOUP2P_EASYTIER_DIR  directory holding easytier-core and easytier-cli
//	                    (default: next to this binary if present there, else PATH)
//
// Joining a room creates a TUN device, so the daemon needs root or
// CAP_NET_ADMIN (see deploy/systemd/mokoup2pd@.service).
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Bank5y/mokoup2p/client/daemon"
	"github.com/Bank5y/mokoup2p/client/engine/easytier"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

func main() {
	stateDir := os.Getenv("MOKOUP2P_STATE_DIR")
	if stateDir == "" {
		cfgDir, err := os.UserConfigDir()
		if err != nil {
			log.Fatalf("locate config directory: %v", err)
		}
		stateDir = filepath.Join(cfgDir, "mokoup2p")
	}

	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		log.Fatalf("create state directory: %v", err)
	}
	etDir := easytierDir()
	eng := easytier.New(easytier.Options{
		CorePath: filepath.Join(etDir, "easytier-core"),
		CLIPath:  filepath.Join(etDir, "easytier-cli"),
		PIDFile:  filepath.Join(stateDir, "easytier-core.pid"),
	})

	d, err := daemon.New(daemon.Config{StateDir: stateDir, Engine: eng})
	if err != nil {
		log.Fatalf("start: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	d.Start(ctx)

	socketPath := ipc.DefaultSocketPath()
	l, err := daemon.Listen(socketPath)
	if err != nil {
		log.Fatalf("listen on %s: %v", socketPath, err)
	}
	go func() {
		<-ctx.Done()
		d.Close()
	}()
	log.Printf("mokoup2pd listening on %s", socketPath)
	if err := d.Serve(l); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

// easytierDir finds the pinned EasyTier binaries. An empty result makes
// the engine look them up in PATH.
func easytierDir() string {
	if dir := os.Getenv("MOKOUP2P_EASYTIER_DIR"); dir != "" {
		return dir
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "easytier-core")); err == nil {
			return dir
		}
	}
	return ""
}
