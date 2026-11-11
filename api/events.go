package api

import (
	"encoding/json"
	"time"
)

// PathEvents is the realtime channel: a WebSocket that every logged-in
// device keeps open. The upgrade request is authenticated like any other
// call (Authorization: Bearer <access token>); a rejected token is answered
// with an ordinary JSON error before upgrading.
//
// Every WebSocket message, in either direction, is one JSON Message. The
// device sends MsgHeartbeat at least every HeartbeatInterval and the server
// answers each with MsgHeartbeatAck. The server pushes events (Event*)
// whenever something concerning the device happens. Clients ignore message
// types they do not know, so new events can be added freely.
const PathEvents = "/api/v1/events"

// HeartbeatTimeout is how long the server waits for a device's heartbeat
// before it ends the device's membership. Connecting to PathEvents and
// joining a room count as heartbeats too.
const HeartbeatTimeout = 30 * time.Second

// HeartbeatInterval is how often a device should send MsgHeartbeat.
const HeartbeatInterval = 10 * time.Second

type MessageType string

// Message is the envelope of every realtime message. Data depends on Type.
type Message struct {
	Type MessageType     `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Messages without data.
const (
	MsgHeartbeat    MessageType = "heartbeat"     // device → server
	MsgHeartbeatAck MessageType = "heartbeat_ack" // server → device
)

// Events pushed by the server, with the type of their data.
const (
	// EventMemberJoined (MemberJoined) goes to the room's other members.
	EventMemberJoined MessageType = "member_joined"
	// EventMemberLeft (MemberLeft) goes to the room's remaining members.
	EventMemberLeft MessageType = "member_left"
)

// MemberJoined: a device has become a member of Room.
type MemberJoined struct {
	// Room is the room code, displayed as XXXX-XXXX.
	Room   string `json:"room"`
	Member Member `json:"member"`
}

// LeaveReason says why a membership ended.
type LeaveReason string

const (
	LeftByDevice     LeaveReason = "left"              // the device left
	LeftByTimeout    LeaveReason = "heartbeat_timeout" // no heartbeat for HeartbeatTimeout
	LeftWhenDisabled LeaveReason = "account_disabled"  // its user was disabled
)

// MemberLeft: a device is no longer a member of Room.
type MemberLeft struct {
	Room   string      `json:"room"`
	Member Member      `json:"member"`
	Reason LeaveReason `json:"reason"`
}
