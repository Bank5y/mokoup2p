package api

// PathRoomMembers is where a device joins a room (POST). Like PathRoom, the
// caller escapes the code.
func PathRoomMembers(code string) string { return PathRoom(code) + "/members" }

// PathMembership is the calling device's current membership: GET returns
// MyMembership, DELETE leaves the room the device is in.
const PathMembership = "/api/v1/me/membership"

// MyMembership tells a device which room it is in; Room is null when in none.
type MyMembership struct {
	Room *Room `json:"room"`
}

// Error codes of joining a room. A disabled account and an unknown room
// reuse ErrAccountDisabled and ErrRoomNotFound.
const (
	ErrRoomPasswordWrong ErrorCode = "room_password_wrong"
	ErrRoomFull          ErrorCode = "room_full"
	ErrInOtherRoom       ErrorCode = "in_other_room"
)

type JoinRoomRequest struct {
	// Password is the room password; the room owner's devices may omit it.
	Password string `json:"password,omitempty"`
}

// JoinResult is returned when a device has become a member of a room.
type JoinResult struct {
	Room    Room          `json:"room"`
	Network NetworkConfig `json:"network"`
}

// NetworkConfig is everything a device needs to enter a room's virtual
// network. It contains the network key, so it is only ever sent to an
// admitted device.
type NetworkConfig struct {
	// NetworkName is the EasyTier network name, derived from the room's
	// internal identity (not from its room code).
	NetworkName string `json:"networkName"`
	// NetworkKey is the EasyTier network secret.
	NetworkKey string `json:"networkKey"`
	// VirtualIP is this device's address in the room, e.g. "10.208.3.1". It
	// stays the same every time the device joins this room.
	VirtualIP string `json:"virtualIp"`
	// Subnet is the room's network, e.g. "10.208.3.0/24".
	Subnet string `json:"subnet"`
	// RelayPeers are the relay node addresses, e.g. "tcp://relay.example.com:11010".
	RelayPeers []string `json:"relayPeers"`
}

// ErrNotRoomMember: only the room's members, its owner and administrators
// may see who is in a room.
const ErrNotRoomMember ErrorCode = "not_room_member"

// Member is a device in a room, as the other members see it.
type Member struct {
	// ID identifies the device on the server (e.g. to kick it). It is not
	// the device identity the client generated.
	ID         int64  `json:"id"`
	Username   string `json:"username"`
	DeviceName string `json:"deviceName"`
	// VirtualIP is the device's address in the room.
	VirtualIP string `json:"virtualIp"`
}

// MemberList is returned by GET PathRoomMembers, ordered by virtual IP.
type MemberList struct {
	Members []Member `json:"members"`
}
