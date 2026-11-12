// Package daemon is the client background service. It logs in on behalf of
// the user, will drive the network engine, and serves the IPC API used by the
// CLI and GUI.
package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/engine"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

type Config struct {
	// StateDir holds the saved server address, device identity and refresh token.
	StateDir string
	Engine   engine.Engine
	// Hostname supplies the default device name; defaults to os.Hostname.
	Hostname func() (string, error)
	// AutoLoginRetry is the first delay before retrying automatic login when
	// the server is unreachable; it doubles up to a minute. Default 2s.
	AutoLoginRetry time.Duration
	// HeartbeatInterval is how often the daemon sends heartbeats on the
	// realtime connection. Default api.HeartbeatInterval.
	HeartbeatInterval time.Duration
	// ReconnectDelay is the first delay before reconnecting the realtime
	// connection; it doubles up to a third of api.HeartbeatTimeout. Default 1s.
	ReconnectDelay time.Duration
}

type Daemon struct {
	cfg    Config
	server *serverClient
	http   *http.Server

	stop       context.CancelFunc // ends background work
	background sync.WaitGroup

	mu       sync.Mutex
	saved    persisted
	loggedIn bool
	user     api.User
	// accessToken authenticates calls to the server while logged in.
	accessToken string

	renewMu sync.Mutex // serializes access token renewal

	roomMu  sync.Mutex // serializes joining and leaving rooms
	joining bool
	room    *ipc.CurrentRoom  // the room this device is in, nil when not in one
	network api.NetworkConfig // the network configuration of room
	notice  ipc.Notice        // why the device last left a room on its own

	// serverDeviceName is the device name the server last reported.
	serverDeviceName string

	// The realtime connection (realtime.go).
	connected      bool
	realtimeWake   chan struct{}
	realtimeCancel context.CancelFunc // ends the current connection
	// revision counts changes of the status; changedCh is closed and
	// replaced at each change (see changed).
	revision  uint64
	changedCh chan struct{}
}

func New(cfg Config) (*Daemon, error) {
	if cfg.Hostname == nil {
		cfg.Hostname = os.Hostname
	}
	if cfg.AutoLoginRetry <= 0 {
		cfg.AutoLoginRetry = 2 * time.Second
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = api.HeartbeatInterval
	}
	if cfg.ReconnectDelay <= 0 {
		cfg.ReconnectDelay = time.Second
	}
	saved, err := loadState(cfg.StateDir, cfg.Hostname)
	if err != nil {
		return nil, err
	}
	d := &Daemon{
		cfg:          cfg,
		server:       newServerClient(),
		saved:        saved,
		realtimeWake: make(chan struct{}, 1),
		changedCh:    make(chan struct{}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+ipc.PathStatus, d.handleStatus)
	mux.HandleFunc("PUT "+ipc.PathServer, d.handleSetServer)
	mux.HandleFunc("POST "+ipc.PathLogin, d.handleLogin)
	d.accountRoutes(mux)
	d.roomRoutes(mux)
	d.membershipRoutes(mux)
	mux.HandleFunc("GET "+ipc.PathMembers, d.handleMembers)
	d.http = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	return d, nil
}

// Start cleans up leftovers of a previous run and logs in automatically with
// the saved refresh token. If the server is unreachable, automatic login keeps
// retrying in the background until Close. Failures are logged; the daemon
// stays usable.
func (d *Daemon) Start(ctx context.Context) {
	if err := d.cfg.Engine.Cleanup(ctx); err != nil {
		log.Printf("engine cleanup: %v", err)
	}
	ctx, d.stop = context.WithCancel(context.WithoutCancel(ctx))
	d.background.Add(1)
	go func() {
		defer d.background.Done()
		d.runRealtime(ctx)
	}()
	err := d.autoLogin(ctx)
	if err == nil {
		d.syncDeviceName(ctx)
		return
	}
	log.Printf("auto login: %v", err)
	if api.CodeOf(err) == ipc.ErrServerUnreachable {
		d.background.Add(1)
		go func() {
			defer d.background.Done()
			d.retryAutoLogin(ctx)
		}()
	}
}

func (d *Daemon) retryAutoLogin(ctx context.Context) {
	delay := d.cfg.AutoLoginRetry
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		err := d.autoLogin(ctx)
		if api.CodeOf(err) != ipc.ErrServerUnreachable {
			if err != nil {
				log.Printf("auto login: %v", err)
			} else {
				d.syncDeviceName(ctx)
			}
			return
		}
		delay = min(delay*2, time.Minute)
	}
}

// Serve answers IPC requests on l until Close is called.
func (d *Daemon) Serve(l net.Listener) error {
	err := d.http.Serve(l)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Close leaves the current room (stopping the engine) and stops serving.
func (d *Daemon) Close() error {
	if d.stop != nil {
		d.stop()
	}
	leaveCtx, cancelLeave := context.WithTimeout(context.Background(), 10*time.Second)
	d.leaveRoom(leaveCtx, true)
	cancelLeave()
	d.background.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return d.http.Shutdown(ctx)
}

// autoLogin exchanges the saved refresh token for a session. It returns nil
// when there is nothing to do or the user logged in some other way meanwhile.
func (d *Daemon) autoLogin(ctx context.Context) error {
	d.mu.Lock()
	saved := d.saved
	loggedIn := d.loggedIn
	d.mu.Unlock()
	if loggedIn || saved.ServerURL == "" || saved.RefreshToken == "" {
		return nil
	}

	// The lock is not held during the network call so status stays responsive.
	sess, err := d.server.refresh(ctx, saved.ServerURL, saved.RefreshToken)

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.saved.ServerURL != saved.ServerURL || d.saved.RefreshToken != saved.RefreshToken {
		return nil // superseded by a manual login or a server change
	}
	if err != nil {
		if code := api.CodeOf(err); code == api.ErrTokenInvalid || code == api.ErrAccountDisabled {
			d.saved.RefreshToken = ""
			if saveErr := saveState(d.cfg.StateDir, d.saved); saveErr != nil {
				return saveErr
			}
		}
		return err
	}
	return d.acceptSession(sess)
}

// acceptSession records a fresh session; d.mu must be held.
func (d *Daemon) acceptSession(sess api.Session) error {
	d.saved.RefreshToken = sess.RefreshToken
	if err := saveState(d.cfg.StateDir, d.saved); err != nil {
		return err
	}
	d.loggedIn = true
	d.user = sess.User
	d.accessToken = sess.AccessToken
	d.serverDeviceName = sess.Device.Name
	d.changed()
	d.wakeRealtime()
	return nil
}

// status reports the current state; d.mu must be held.
func (d *Daemon) status() ipc.Status {
	st := ipc.Status{
		State:      ipc.StateLoggedOut,
		ServerURL:  d.saved.ServerURL,
		DeviceID:   d.saved.DeviceID,
		DeviceName: d.saved.DeviceName,
	}
	if d.loggedIn {
		st.State = ipc.StateIdle
		st.Username = d.user.Username
		st.IsAdmin = d.user.IsAdmin
		switch {
		case d.room != nil:
			st.State = ipc.StateInRoom
			room := *d.room
			st.Room = &room
		case d.joining:
			st.State = ipc.StateJoining
		}
		st.Connected = d.connected
	}
	st.Notice = d.notice
	st.Revision = d.revision
	return st
}

func (d *Daemon) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Has(ipc.WaitParam) {
		d.handleWaitStatus(w, r)
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	writeJSON(w, http.StatusOK, d.status())
}

func (d *Daemon) handleSetServer(w http.ResponseWriter, r *http.Request) {
	var req ipc.SetServerRequest
	if !decode(w, r, &req) {
		return
	}
	serverURL, err := normalizeServerURL(req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if serverURL != d.saved.ServerURL {
		if d.joining || d.room != nil {
			writeError(w, errInRoom)
			return
		}
		// A session belongs to one server; switching servers logs out.
		d.saved.ServerURL = serverURL
		d.saved.RefreshToken = ""
		d.loggedIn = false
		d.user = api.User{}
		d.accessToken = ""
		d.changed()
		d.wakeRealtime()
		if err := saveState(d.cfg.StateDir, d.saved); err != nil {
			writeError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, d.status())
}

func normalizeServerURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", &api.Error{Code: api.ErrBadRequest, Message: "server address must be an http:// or https:// URL"}
	}
	return strings.TrimRight(u.String(), "/"), nil
}

func (d *Daemon) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req ipc.LoginRequest
	if !decode(w, r, &req) {
		return
	}
	d.mu.Lock()
	saved := d.saved
	d.mu.Unlock()
	if saved.ServerURL == "" {
		writeError(w, &api.Error{Code: ipc.ErrServerNotConfigured, Message: "server address is not set"})
		return
	}
	// The lock is not held during the network call so status stays responsive.
	sess, err := d.server.login(r.Context(), saved.ServerURL, api.LoginRequest{
		Username:   req.Username,
		Password:   req.Password,
		DeviceID:   saved.DeviceID,
		DeviceName: saved.DeviceName,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	d.mu.Lock()
	if d.saved.ServerURL != saved.ServerURL {
		d.mu.Unlock()
		writeError(w, &api.Error{Code: api.ErrBadRequest, Message: "server address changed during login"})
		return
	}
	err = d.acceptSession(sess)
	if err == nil {
		d.notice = "" // the user is starting afresh
	}
	d.mu.Unlock()
	if err != nil {
		writeError(w, err)
		return
	}
	d.syncDeviceName(r.Context())
	d.mu.Lock()
	defer d.mu.Unlock()
	writeJSON(w, http.StatusOK, d.status())
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, &api.Error{Code: api.ErrBadRequest, Message: "invalid JSON body"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		log.Printf("internal error: %v", err)
		apiErr = &api.Error{Code: api.ErrInternal, Message: err.Error()}
	}
	status := http.StatusBadRequest
	if apiErr.Code == api.ErrInternal {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, apiErr)
}
