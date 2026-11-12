// Package easytier is the production network engine: it runs the pinned,
// unmodified easytier-core as a child process (ADR-0001) and reads peer
// state with easytier-cli over the core's local RPC portal.
package easytier

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Bank5y/mokoup2p/client/engine"
	"github.com/Bank5y/mokoup2p/internal/etproc"
)

// Options locate the EasyTier binaries and fix the local resources the
// engine uses. Zero values take the defaults below. Only one engine per
// machine is supported: the TUN name, listener and RPC ports are fixed.
type Options struct {
	// CorePath and CLIPath are the easytier-core and easytier-cli binaries
	// (default: looked up in PATH).
	CorePath string
	CLIPath  string
	// DevName is the TUN device name (default "mokoup2p0", max 15 chars).
	DevName string
	// ListenPort is where the core accepts direct connections from other
	// members, on TCP and UDP (default 11010).
	ListenPort int
	// RPCPortal is the core's local RPC address (default 127.0.0.1:15888).
	// It exposes the network secret, so it must stay on loopback.
	RPCPortal string
	// PIDFile records the running core so Cleanup can kill it after the
	// daemon died abnormally (optional).
	PIDFile string
	// StartTimeout bounds how long Start waits for the core to answer on
	// its RPC portal (default 20s).
	StartTimeout time.Duration
	// UDPBroadcastRelay turns on EasyTier's Windows broadcast fix
	// (--enable-udp-broadcast-relay): local UDP broadcasts to
	// 255.255.255.255 also enter the room, so LAN games find each other.
	// Windows only; see docs/adr/0003-windows-broadcast.md.
	UDPBroadcastRelay bool
}

func (o Options) withDefaults() Options {
	if o.CorePath == "" {
		o.CorePath = "easytier-core"
	}
	if o.CLIPath == "" {
		o.CLIPath = "easytier-cli"
	}
	if o.DevName == "" {
		o.DevName = "mokoup2p0"
	}
	if o.ListenPort == 0 {
		o.ListenPort = 11010
	}
	if o.RPCPortal == "" {
		o.RPCPortal = "127.0.0.1:15888"
	}
	if o.StartTimeout == 0 {
		o.StartTimeout = 20 * time.Second
	}
	return o
}

// Engine implements engine.Engine.
type Engine struct {
	opts Options

	mu   sync.Mutex
	proc *etproc.Process
}

var _ engine.Engine = (*Engine)(nil)

func New(opts Options) *Engine { return &Engine{opts: opts.withDefaults()} }

// args is the easytier-core command line for cfg, without the network
// secret (passed in the environment so other local users cannot read it).
func (e *Engine) args(cfg engine.Config) []string {
	o := e.opts
	args := []string{
		"--network-name", cfg.NetworkName,
		"--ipv4", cfg.Address.String(),
		"--hostname", cfg.Hostname,
		"--dev-name", o.DevName,
		"--listeners", fmt.Sprintf("tcp://0.0.0.0:%d", o.ListenPort), fmt.Sprintf("udp://0.0.0.0:%d", o.ListenPort),
		"--rpc-portal", o.RPCPortal,
		"--rpc-portal-whitelist", "127.0.0.1/32,::1/128",
		// Refuse strangers (other networks) using this device as a relay.
		"--private-mode", "true",
		"--console-log-level", "warn",
	}
	for _, p := range cfg.RelayPeers {
		args = append(args, "--peers", p)
	}
	if o.UDPBroadcastRelay {
		args = append(args, "--enable-udp-broadcast-relay", "true")
	}
	return args
}

func (e *Engine) Start(ctx context.Context, cfg engine.Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.proc != nil {
		return errors.New("easytier: already running")
	}
	return e.startLocked(ctx, cfg)
}

func (e *Engine) startLocked(ctx context.Context, cfg engine.Config) error {
	proc, err := etproc.Start(etproc.Spec{
		Path:    e.opts.CorePath,
		Args:    e.args(cfg),
		Env:     []string{"ET_NETWORK_SECRET=" + cfg.NetworkSecret},
		Name:    "easytier-core",
		OnStart: e.writePIDFile,
	})
	if err != nil {
		return err
	}
	if err := e.waitReady(ctx, proc); err != nil {
		_ = proc.Stop(context.Background())
		e.removePIDFile()
		return err
	}
	e.proc = proc
	return nil
}

// waitReady waits until the core answers on its RPC portal.
func (e *Engine) waitReady(ctx context.Context, proc *etproc.Process) error {
	ctx, cancel := context.WithTimeout(ctx, e.opts.StartTimeout)
	defer cancel()
	for {
		if proc.Exited() {
			return errors.New("easytier-core exited during start (see the daemon log)")
		}
		if _, err := e.cli(ctx, "node", "info"); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("easytier-core did not become ready: %w", ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (e *Engine) Stop(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stopLocked(ctx)
}

func (e *Engine) stopLocked(ctx context.Context) error {
	if e.proc == nil {
		return nil
	}
	err := e.proc.Stop(ctx)
	e.proc = nil
	e.removePIDFile()
	return err
}

// Restart stops the core and starts it with cfg. Members reconnect within
// a few seconds; the TUN device is recreated with the same address.
func (e *Engine) Restart(ctx context.Context, cfg engine.Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.stopLocked(ctx); err != nil {
		log.Printf("easytier: stop before restart: %v", err)
	}
	return e.startLocked(ctx, cfg)
}

func (e *Engine) Peers(ctx context.Context) ([]engine.Peer, error) {
	e.mu.Lock()
	running := e.proc != nil
	e.mu.Unlock()
	if !running {
		return []engine.Peer{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := e.cli(ctx, "-o", "json", "peer", "list")
	if err != nil {
		return nil, err
	}
	return ParsePeerList(out)
}

// cli runs easytier-cli against the core's RPC portal.
func (e *Engine) cli(ctx context.Context, args ...string) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, e.opts.CLIPath, append([]string{"--rpc-portal", e.opts.RPCPortal}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("easytier-cli %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("easytier-cli %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// Cleanup kills an easytier-core left behind by a previous daemon run and
// removes a leftover TUN device. The kernel removes a non-persistent TUN
// and its routes when the core exits, so the device step is defensive.
func (e *Engine) Cleanup(context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var errs []error
	if e.opts.PIDFile != "" {
		if raw, err := os.ReadFile(e.opts.PIDFile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil && pid > 0 {
				if err := killStaleCore(pid); err != nil {
					errs = append(errs, err)
				}
			}
		}
		e.removePIDFile()
	}
	if err := removeDevice(e.opts.DevName); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (e *Engine) writePIDFile(pid int) {
	if e.opts.PIDFile == "" {
		return
	}
	if err := os.WriteFile(e.opts.PIDFile, []byte(strconv.Itoa(pid)+"\n"), 0o600); err != nil {
		log.Printf("easytier: write PID file: %v", err)
	}
}

func (e *Engine) removePIDFile() {
	if e.opts.PIDFile != "" {
		_ = os.Remove(e.opts.PIDFile)
	}
}
