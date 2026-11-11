//go:build !linux

package daemon

import (
	"errors"
	"net"
)

// Listen is only implemented on Linux for now; Windows will use a named pipe.
func Listen(string) (net.Listener, error) {
	return nil, errors.New("IPC listener not implemented on this platform")
}
