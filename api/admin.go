package api

// Administrator room management and system settings.
const (
	// PathAdminRooms lists every room (GET).
	PathAdminRooms = "/api/v1/admin/rooms"
	// PathAdminSettings reads (GET) and replaces (PUT) the system settings.
	PathAdminSettings = "/api/v1/admin/settings"
)

// PathAdminRoom is one room on the administrator page; DELETE removes it.
// Like PathRoom it accepts the code in any case, with or without separator.
func PathAdminRoom(code string) string { return PathAdminRooms + "/" + code }

// AdminRoom is a room as seen on the administrator page.
type AdminRoom struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Owner       string `json:"owner"`
	HasPassword bool   `json:"hasPassword"`
	Subnet      string `json:"subnet"`
	// OnlineMembers is how many devices are in the room now.
	OnlineMembers int `json:"onlineMembers"`
}

// AdminRoomList is returned by GET PathAdminRooms.
type AdminRoomList struct {
	Rooms []AdminRoom `json:"rooms"`
}

// Settings are the system-wide limits. They apply to rooms created and
// devices joining after they change.
type Settings struct {
	// RoomsPerUser is how many rooms one user may own (1–1000).
	RoomsPerUser int `json:"roomsPerUser"`
	// RoomCapacity is how many devices one room may hold (1–254, the
	// usable addresses of a /24).
	RoomCapacity int `json:"roomCapacity"`
}
