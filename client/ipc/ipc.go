// Package ipc is the local API between the client daemon and its front ends
// (CLI, GUI): JSON over HTTP on a Unix socket (Linux).
package ipc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Bank5y/mokoup2p/api"
)

const (
	PathStatus = "/v1/status"
	PathServer = "/v1/server"
	PathLogin  = "/v1/login"
)

// Error codes produced by the daemon itself. Errors relayed from the server
// keep the server's api.ErrorCode.
const (
	ErrServerNotConfigured api.ErrorCode = "server_not_configured"
	ErrServerUnreachable   api.ErrorCode = "server_unreachable"
	ErrServerBadResponse   api.ErrorCode = "server_bad_response"
)

// State is the daemon's login state.
type State string

const (
	StateLoggedOut State = "logged_out"
	StateIdle      State = "idle" // logged in, not in a room
)

type Status struct {
	State      State  `json:"state"`
	ServerURL  string `json:"serverUrl"`
	Username   string `json:"username,omitempty"`
	IsAdmin    bool   `json:"isAdmin,omitempty"`
	DeviceID   string `json:"deviceId"`
	DeviceName string `json:"deviceName"`
	// Room is set in StateInRoom.
	Room *CurrentRoom `json:"room,omitempty"`
}

type SetServerRequest struct {
	URL string `json:"url"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// DefaultSocketPath is where the daemon listens for the current user.
func DefaultSocketPath() string {
	if p := os.Getenv("MOKOUP2P_SOCKET"); p != "" {
		return p
	}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "mokoup2p", "daemon.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("mokoup2p-%d", os.Getuid()), "daemon.sock")
}

// Client talks to the daemon.
type Client struct {
	http *http.Client
}

func NewClient(socketPath string) *Client {
	return &Client{http: &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		},
	}}}
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	var st Status
	err := c.do(ctx, http.MethodGet, PathStatus, nil, &st)
	return st, err
}

func (c *Client) SetServer(ctx context.Context, url string) (Status, error) {
	var st Status
	err := c.do(ctx, http.MethodPut, PathServer, SetServerRequest{URL: url}, &st)
	return st, err
}

func (c *Client) Login(ctx context.Context, username, password string) (Status, error) {
	var st Status
	err := c.do(ctx, http.MethodPost, PathLogin, LoginRequest{Username: username, Password: password}, &st)
	return st, err
}

// do sends a request; non-2xx responses are returned as *api.Error.
func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body bytes.Buffer
	if in != nil {
		if err := json.NewEncoder(&body).Encode(in); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://daemon"+path, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("connect to daemon: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		apiErr := &api.Error{}
		if err := json.NewDecoder(resp.Body).Decode(apiErr); err != nil {
			return fmt.Errorf("daemon returned status %d", resp.StatusCode)
		}
		return apiErr
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
