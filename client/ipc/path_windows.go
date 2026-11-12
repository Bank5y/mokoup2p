package ipc

import (
	"context"
	"net"
	"os"

	"github.com/Microsoft/go-winio"
)

// DefaultPipeName is the named pipe of the mokoup2pd Windows service. Its
// ACL admits only SYSTEM and the user the service was installed for.
const DefaultPipeName = `\\.\pipe\mokoup2p`

// DefaultSocketPath is where the daemon listens: the service's named pipe,
// unless MOKOUP2P_SOCKET names another one.
func DefaultSocketPath() string {
	if p := os.Getenv("MOKOUP2P_SOCKET"); p != "" {
		return p
	}
	return DefaultPipeName
}

func dial(ctx context.Context, pipe string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, pipe)
}
