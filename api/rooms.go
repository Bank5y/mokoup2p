package api

// PathRooms is the room collection. A room is addressed by its room code
// (PathRoom); the server ignores case and separators in it.
const PathRooms = "/api/v1/rooms"

// PathRoom is the path of one room.
func PathRoom(code string) string { return PathRooms + "/" + code }

// Error codes of the room API.
const (
	ErrRoomNotFound     ErrorCode = "room_not_found"
	ErrRoomLimitReached ErrorCode = "room_limit_reached"
	ErrNotRoomOwner     ErrorCode = "not_room_owner"
)

// Room is a room as its owner sees it. The network key is never included.
type Room struct {
	// Code is the room code, displayed as XXXX-XXXX.
	Code        string `json:"code"`
	Name        string `json:"name"`
	HasPassword bool   `json:"hasPassword"`
	// Subnet is the room's virtual network, e.g. "10.208.3.0/24".
	Subnet string `json:"subnet"`
	// Owner is the room owner's username.
	Owner string `json:"owner"`
}

type CreateRoomRequest struct {
	Name string `json:"name"`
	// Password is optional; empty means no room password.
	Password string `json:"password,omitempty"`
}

// UpdateRoomRequest changes the fields that are present. Password "" removes
// the room password.
type UpdateRoomRequest struct {
	Name     *string `json:"name,omitempty"`
	Password *string `json:"password,omitempty"`
}

type RoomList struct {
	Rooms []Room `json:"rooms"`
}
