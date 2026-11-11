package ipc

import (
	"context"
	"net/http"
	"net/url"

	"github.com/Bank5y/mokoup2p/api"
)

// PathRooms lists and creates the rooms the logged-in user owns; PathRoom
// addresses one of them by room code.
const PathRooms = "/v1/rooms"

func PathRoom(code string) string { return PathRooms + "/" + code }

// ErrNotLoggedIn is returned by calls that need a logged-in user.
const ErrNotLoggedIn api.ErrorCode = "not_logged_in"

func (c *Client) CreateRoom(ctx context.Context, req api.CreateRoomRequest) (api.Room, error) {
	var room api.Room
	err := c.do(ctx, http.MethodPost, PathRooms, req, &room)
	return room, err
}

// MyRooms lists the rooms the logged-in user owns.
func (c *Client) MyRooms(ctx context.Context) ([]api.Room, error) {
	var list api.RoomList
	err := c.do(ctx, http.MethodGet, PathRooms, nil, &list)
	return list.Rooms, err
}

func (c *Client) UpdateRoom(ctx context.Context, code string, req api.UpdateRoomRequest) (api.Room, error) {
	var room api.Room
	err := c.do(ctx, http.MethodPatch, PathRoom(url.PathEscape(code)), req, &room)
	return room, err
}

func (c *Client) DeleteRoom(ctx context.Context, code string) error {
	return c.do(ctx, http.MethodDelete, PathRoom(url.PathEscape(code)), nil, nil)
}
