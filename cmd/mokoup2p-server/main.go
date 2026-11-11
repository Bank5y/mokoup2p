// Command mokoup2p-server runs the mokoup2p server.
//
// Environment:
//
//	MOKOUP2P_LISTEN   listen address (default ":8080")
//	MOKOUP2P_DB       SQLite database file (default "/data/mokoup2p.db")
//	ADMIN_USERNAME    first administrator, created only if none exists
//	ADMIN_PASSWORD
//	MOKOUP2P_RELAY_PEERS  relay node addresses handed to devices, comma-separated,
//	                      e.g. "tcp://p2p.example.com:11010,udp://p2p.example.com:11010"
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Bank5y/mokoup2p/server"
)

func main() {
	listen := envOr("MOKOUP2P_LISTEN", ":8080")
	srv, err := server.New(server.Config{
		DBPath:        envOr("MOKOUP2P_DB", "/data/mokoup2p.db"),
		AdminUsername: os.Getenv("ADMIN_USERNAME"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		RelayPeers:    splitList(os.Getenv("MOKOUP2P_RELAY_PEERS")),
	})
	if err != nil {
		log.Fatalf("start: %v", err)
	}
	defer srv.Close()

	httpSrv := &http.Server{Addr: listen, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	log.Printf("mokoup2p-server listening on %s", listen)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// splitList splits a comma-separated list, dropping blanks.
func splitList(v string) []string {
	var out []string
	for _, item := range strings.Split(v, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
