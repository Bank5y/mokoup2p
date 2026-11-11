package api

// PathRelayNetworks lists the EasyTier network names the relay node may
// serve. It is called by the relay wrapper (cmd/mokoup2p-relay), not by
// devices, and is authorized with the shared relay token as bearer token.
// See docs/adr/0002-relay-whitelist-sync.md.
const PathRelayNetworks = "/internal/v1/relay/networks"

// RelayNetworks is the relay whitelist: the network names of every existing
// room, sorted.
type RelayNetworks struct {
	Networks []string `json:"networks"`
}
