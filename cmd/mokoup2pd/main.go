// Command mokoup2pd is the mokoup2p client background service.
//
// Environment:
//
//	MOKOUP2P_STATE_DIR  saved state (default: $XDG_CONFIG_HOME/mokoup2p)
//	MOKOUP2P_SOCKET     IPC socket (default: $XDG_RUNTIME_DIR/mokoup2p/daemon.sock)
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Bank5y/mokoup2p/client/daemon"
	"github.com/Bank5y/mokoup2p/client/engine"
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

	d, err := daemon.New(daemon.Config{StateDir: stateDir, Engine: engine.Noop{}})
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
