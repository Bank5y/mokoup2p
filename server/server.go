// Package server is the mokoup2p server: HTTP JSON API over SQLite.
package server

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/internal/clock"

	_ "modernc.org/sqlite"
)

type Config struct {
	// DBPath is the SQLite database file.
	DBPath string
	// Clock defaults to the wall clock.
	Clock clock.Clock
	// AdminUsername and AdminPassword create the first administrator when
	// none exists. They are ignored once an administrator exists.
	AdminUsername string
	AdminPassword string
	// RelayPeers are the relay node addresses handed to devices in their
	// network configuration.
	RelayPeers []string
}

type Server struct {
	db         *sql.DB
	clock      clock.Clock
	mux        *http.ServeMux
	relayPeers []string
}

func New(cfg Config) (*Server, error) {
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	db, err := openDB(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	s := &Server{db: db, clock: cfg.Clock, relayPeers: cfg.RelayPeers}
	if err := s.ensureAdministrator(cfg.AdminUsername, cfg.AdminPassword); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.endAllMemberships(); err != nil {
		db.Close()
		return nil, err
	}
	s.routes()
	return s, nil
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) Close() error { return s.db.Close() }

func (s *Server) routes() {
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("POST "+api.PathLogin, s.handleLogin)
	s.mux.HandleFunc("POST "+api.PathRefresh, s.handleRefresh)
	s.mux.HandleFunc("GET "+api.PathMe, s.authenticated(s.handleMe))
	s.roomRoutes()
	s.membershipRoutes()
}

func openDB(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, ddl := range []string{schema, settingsSchema, roomSchema, membershipSchema} {
		if _, err := db.Exec(ddl); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}
	return db, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY,
	username      TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	is_admin      INTEGER NOT NULL DEFAULT 0,
	disabled      INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS devices (
	id        INTEGER PRIMARY KEY,
	user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	device_id TEXT NOT NULL,
	name      TEXT NOT NULL,
	UNIQUE (user_id, device_id)
);
CREATE TABLE IF NOT EXISTS tokens (
	token_hash TEXT PRIMARY KEY,
	kind       TEXT NOT NULL,
	device_pk  INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
	expires_at INTEGER NOT NULL
);
`

var errNoAdminCredentials = errors.New("no administrator exists: ADMIN_USERNAME and ADMIN_PASSWORD must be set")
