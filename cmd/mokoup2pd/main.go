// Command mokoup2pd is the mokoup2p client background service.
//
// Environment:
//
//	MOKOUP2P_STATE_DIR  saved state (default: $XDG_CONFIG_HOME/mokoup2p;
//	                    the Windows service uses %ProgramData%\mokoup2p)
//	MOKOUP2P_SOCKET     IPC socket or pipe (default: see ipc.DefaultSocketPath)
//	MOKOUP2P_EASYTIER_DIR  directory holding easytier-core and easytier-cli
//	                    (default: next to this binary if present there, else PATH)
//
// Joining a room creates a virtual network adapter, so the daemon needs
// root or CAP_NET_ADMIN on Linux (see deploy/systemd/mokoup2pd@.service) and
// runs as a Windows service (LocalSystem) on Windows; see main_windows.go.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Bank5y/mokoup2p/client/daemon"
	"github.com/Bank5y/mokoup2p/client/engine/easytier"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

// run runs the daemon with its state in stateDir until ctx is done. edition
// is how this copy was packaged (empty: the platform's default). listen
// opens the IPC listener once the daemon has started; it also returns its
// address for the log.
func run(ctx context.Context, stateDir string, edition ipc.Edition, listen func() (net.Listener, string, error)) error {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	etDir := easytierDir()
	opts := easytier.Options{
		CorePath: filepath.Join(etDir, "easytier-core"+exeSuffix),
		CLIPath:  filepath.Join(etDir, "easytier-cli"+exeSuffix),
		PIDFile:  filepath.Join(stateDir, "easytier-core.pid"),
	}
	platformEngineOptions(&opts)
	eng := easytier.New(opts)

	d, err := daemon.New(daemon.Config{StateDir: stateDir, Engine: eng, Edition: edition})
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	d.Start(ctx)

	l, addr, err := listen()
	if err != nil {
		d.Close()
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	go func() {
		<-ctx.Done()
		d.Close()
	}()
	log.Printf("mokoup2pd listening on %s", addr)
	if err := d.Serve(l); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

var exeSuffix = map[bool]string{true: ".exe", false: ""}[runtime.GOOS == "windows"]

// easytierDir finds the pinned EasyTier binaries. An empty result makes
// the engine look them up in PATH.
func easytierDir() string {
	if dir := os.Getenv("MOKOUP2P_EASYTIER_DIR"); dir != "" {
		return dir
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "easytier-core"+exeSuffix)); err == nil {
			return dir
		}
	}
	return ""
}
