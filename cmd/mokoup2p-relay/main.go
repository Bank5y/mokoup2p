// Command mokoup2p-relay runs the relay node: an unmodified easytier-core
// that relays only the networks of rooms registered on the mokoup2p server.
//
// EasyTier v2.6.4 reads relay_network_whitelist only at startup, so this
// wrapper polls the server's whitelist and restarts easytier-core with a
// fresh config whenever it changes. See docs/adr/0002-relay-whitelist-sync.md.
//
// Environment:
//
//	MOKOUP2P_SERVER_URL      server base URL (default "http://server:8080")
//	MOKOUP2P_RELAY_TOKEN     shared secret, same value as the server's (required)
//	MOKOUP2P_RELAY_LISTEN_PORT  TCP and UDP port inside the container (default 11010)
//	MOKOUP2P_RELAY_POLL      poll interval (default "3s")
//	MOKOUP2P_EASYTIER_CORE   easytier-core binary (default: from PATH)
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/internal/etproc"
)

// settle is how long a change must stay unchanged before the relay
// restarts, so a burst of room creations costs one restart.
const settle = 2 * time.Second

type relay struct {
	serverURL string
	token     string
	port      int
	core      string
	confPath  string
	secret    string

	proc    *etproc.Process
	current []string // whitelist the running core uses
}

func main() {
	log.SetFlags(log.LstdFlags)
	token := os.Getenv("MOKOUP2P_RELAY_TOKEN")
	if token == "" {
		log.Fatal("MOKOUP2P_RELAY_TOKEN must be set (the same value as the server's)")
	}
	port, err := strconv.Atoi(envOr("MOKOUP2P_RELAY_LISTEN_PORT", "11010"))
	if err != nil {
		log.Fatalf("MOKOUP2P_RELAY_LISTEN_PORT: %v", err)
	}
	poll, err := time.ParseDuration(envOr("MOKOUP2P_RELAY_POLL", "3s"))
	if err != nil {
		log.Fatalf("MOKOUP2P_RELAY_POLL: %v", err)
	}
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		log.Fatal(err)
	}
	confDir, err := os.MkdirTemp("", "mokoup2p-relay")
	if err != nil {
		log.Fatal(err)
	}
	r := &relay{
		serverURL: strings.TrimRight(envOr("MOKOUP2P_SERVER_URL", "http://server:8080"), "/"),
		token:     token,
		port:      port,
		core:      envOr("MOKOUP2P_EASYTIER_CORE", "easytier-core"),
		confPath:  filepath.Join(confDir, "relay.toml"),
		// The relay's own network is never joined by anyone; its secret
		// only has to be unguessable.
		secret: hex.EncodeToString(secret),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	r.run(ctx, poll)
	if r.proc != nil {
		_ = r.proc.Stop(context.Background())
	}
}

func (r *relay) run(ctx context.Context, poll time.Duration) {
	for {
		r.sync(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(poll):
		}
	}
}

// sync fetches the whitelist and restarts the core if it changed. Until the
// first successful fetch the core runs with an empty whitelist (relays
// nothing); after that a failed fetch keeps the last known whitelist.
func (r *relay) sync(ctx context.Context) {
	want, err := r.fetch(ctx)
	if err != nil {
		log.Printf("fetch whitelist: %v", err)
		if r.proc == nil {
			r.apply([]string{})
		}
		return
	}
	if r.proc != nil && slices.Equal(want, r.current) {
		return
	}
	if r.proc != nil {
		// Wait for the burst to settle, then take the latest list.
		select {
		case <-ctx.Done():
			return
		case <-time.After(settle):
		}
		if latest, err := r.fetch(ctx); err == nil {
			want = latest
		}
		if slices.Equal(want, r.current) {
			return
		}
	}
	r.apply(want)
}

func (r *relay) fetch(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.serverURL+api.PathRelayNetworks, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server answered %s", resp.Status)
	}
	var body api.RelayNetworks
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	names := []string{}
	for _, n := range body.Networks {
		// The whitelist is space-separated wildcard patterns: never let a
		// name widen it.
		if n != "" && !strings.ContainsAny(n, " \t\n*?[]") {
			names = append(names, n)
		}
	}
	slices.Sort(names)
	return names, nil
}

// apply writes the config for whitelist and (re)starts the core with it.
func (r *relay) apply(whitelist []string) {
	if err := os.WriteFile(r.confPath, []byte(r.config(whitelist)), 0o600); err != nil {
		log.Printf("write config: %v", err)
		return
	}
	if out, err := exec.Command(r.core, "-c", r.confPath, "--check-config").CombinedOutput(); err != nil {
		log.Printf("config rejected by easytier-core, keeping the running relay: %v: %s", err, out)
		return
	}
	if r.proc != nil {
		if err := r.proc.Stop(context.Background()); err != nil {
			log.Printf("stop easytier-core: %v", err)
		}
		r.proc = nil
	}
	proc, err := etproc.Start(etproc.Spec{Path: r.core, Args: []string{"-c", r.confPath}, Name: "easytier-core"})
	if err != nil {
		log.Printf("start easytier-core: %v", err)
		return
	}
	r.proc = proc
	r.current = whitelist
	log.Printf("relaying %d room network(s)", len(whitelist))
}

func (r *relay) config(whitelist []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "instance_name = \"mokoup2p-relay\"\n")
	fmt.Fprintf(&b, "listeners = [\"tcp://0.0.0.0:%d\", \"udp://0.0.0.0:%d\"]\n\n", r.port, r.port)
	fmt.Fprintf(&b, "[network_identity]\nnetwork_name = \"mokoup2p-relay\"\nnetwork_secret = %q\n\n", r.secret)
	// No ipv4 => no TUN: the relay only forwards. private_mode must stay
	// off because every room has its own secret; the whitelist is the gate.
	fmt.Fprintf(&b, "[flags]\nno_tun = true\nprivate_mode = false\n")
	fmt.Fprintf(&b, "relay_network_whitelist = %q\n", strings.Join(whitelist, " "))
	return b.String()
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
