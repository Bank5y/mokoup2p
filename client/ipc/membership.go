package ipc

import (
	"context"
	"net/http"

	"github.com/Bank5y/mokoup2p/api"
)

const (
	PathJoin  = "/v1/join"
	PathLeave = "/v1/leave"
)

// States of the room lifecycle, after StateIdle.
const (
	StateJoining State = "joining"
	StateInRoom  State = "in_room"
)

// Daemon error codes of joining and leaving. Server rejections keep the
// server's codes (api.ErrRoomNotFound, api.ErrRoomPasswordWrong, ...).
const (
	// ErrInRoom: the device is joining or in a room and must leave it first.
	ErrInRoom api.ErrorCode = "in_room"
	// ErrEngineFailed: the server admitted the device but the network
	// engine could not start; the device has left the room again.
	ErrEngineFailed api.ErrorCode = "engine_failed"
)

type JoinRequest struct {
	Code     string `json:"code"`
	Password string `json:"password,omitempty"`
}

// CurrentRoom is the room the device is in.
type CurrentRoom struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Owner string `json:"owner"`
	// VirtualIP is this device's address in the room.
	VirtualIP string `json:"virtualIp"`
	Subnet    string `json:"subnet"`
}

// Join enters a room and returns the status, which is StateInRoom on success.
func (c *Client) Join(ctx context.Context, code, password string) (Status, error) {
	var st Status
	err := c.do(ctx, http.MethodPost, PathJoin, JoinRequest{Code: code, Password: password}, &st)
	return st, err
}

// Leave leaves the current room; it succeeds when not in a room as well.
func (c *Client) Leave(ctx context.Context) (Status, error) {
	var st Status
	err := c.do(ctx, http.MethodPost, PathLeave, nil, &st)
	return st, err
}
