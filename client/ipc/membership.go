package ipc

import (
	"context"
	"net/http"

	"github.com/Bank5y/mokoup2p/api"
)

const (
	PathJoin    = "/v1/join"
	PathLeave   = "/v1/leave"
	PathMembers = "/v1/members"
	PathKick    = "/v1/kick"
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
	// ErrNotInRoom: the request needs the device to be in a room.
	ErrNotInRoom api.ErrorCode = "not_in_room"
)

// Notice tells the user why the device left its room without asking to.
// It is shown until the next join or login.
type Notice string

const (
	NoticeKicked          Notice = "kicked"           // the room owner kicked this device
	NoticeRoomDeleted     Notice = "room_deleted"     // the room was deleted
	NoticeAccountDisabled Notice = "account_disabled" // an administrator disabled the account
)

// KickRequest asks the room owner's daemon to kick a member (Member.ID).
type KickRequest struct {
	MemberID int64 `json:"memberId"`
}

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

// Kick removes a member from the current room; only its owner may do so.
func (c *Client) Kick(ctx context.Context, memberID int64) error {
	return c.do(ctx, http.MethodPost, PathKick, KickRequest{MemberID: memberID}, nil)
}

// Leave leaves the current room; it succeeds when not in a room as well.
func (c *Client) Leave(ctx context.Context) (Status, error) {
	var st Status
	err := c.do(ctx, http.MethodPost, PathLeave, nil, &st)
	return st, err
}

// Member is a device in the current room.
type Member struct {
	// ID identifies the device on the server.
	ID         int64  `json:"id"`
	Username   string `json:"username"`
	DeviceName string `json:"deviceName"`
	VirtualIP  string `json:"virtualIp"`
	// Self marks this device.
	Self bool `json:"self,omitempty"`
}

type MemberList struct {
	Members []Member `json:"members"`
}

// Members lists the devices in the current room, ordered by virtual IP.
func (c *Client) Members(ctx context.Context) ([]Member, error) {
	var list MemberList
	err := c.do(ctx, http.MethodGet, PathMembers, nil, &list)
	return list.Members, err
}
