//go:build !windows

package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Bank5y/mokoup2p/client/daemon"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := run(ctx, stateDir, func() (net.Listener, string, error) {
		path := ipc.DefaultSocketPath()
		l, err := daemon.Listen(path)
		return l, path, err
	})
	if err != nil {
		log.Fatal(err)
	}
}
