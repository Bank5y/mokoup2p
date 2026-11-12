package daemon

import (
	"fmt"
	"net"
	"os/user"
	"strings"

	"github.com/Microsoft/go-winio"
)

// Listen opens the IPC named pipe for the user running the daemon (handy
// when it runs in a console). The Windows service uses ListenPipe with the
// user it was installed for.
func Listen(pipe string) (net.Listener, error) {
	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("current user: %w", err)
	}
	return ListenPipe(pipe, u.Uid) // on Windows Uid is the user's SID
}

// ListenPipe opens the IPC named pipe. Its ACL admits only LocalSystem (the
// service itself) and the given users (SIDs such as S-1-5-21-…): nobody
// else on the machine can talk to the daemon.
func ListenPipe(pipe string, userSIDs ...string) (net.Listener, error) {
	if len(userSIDs) == 0 {
		return nil, fmt.Errorf("listen on %s: no user allowed", pipe)
	}
	var sddl strings.Builder
	sddl.WriteString("D:P(A;;GA;;;SY)")
	for _, sid := range userSIDs {
		if !strings.HasPrefix(sid, "S-") || strings.ContainsAny(sid, "();") {
			return nil, fmt.Errorf("listen on %s: invalid SID %q", pipe, sid)
		}
		fmt.Fprintf(&sddl, "(A;;GRGW;;;%s)", sid)
	}
	return winio.ListenPipe(pipe, &winio.PipeConfig{
		SecurityDescriptor: sddl.String(),
		InputBufferSize:    64 * 1024,
		OutputBufferSize:   64 * 1024,
	})
}

// LookupUserSID resolves a user name (or DOMAIN\user) to its SID; a SID is
// returned as is.
func LookupUserSID(name string) (string, error) {
	if strings.HasPrefix(name, "S-") {
		return name, nil
	}
	u, err := user.Lookup(name)
	if err != nil {
		return "", err
	}
	return u.Uid, nil
}
