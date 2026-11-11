package daemon

import (
	"context"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

var errNotLoggedIn = &api.Error{Code: ipc.ErrNotLoggedIn, Message: "not logged in"}

// serverCall calls an authenticated server API as the logged-in user. When
// the server rejects the access token (it expires after a few minutes), the
// session is renewed with the refresh token and the call is repeated once.
func (d *Daemon) serverCall(ctx context.Context, method, path string, in, out any) error {
	d.mu.Lock()
	loggedIn, baseURL, token := d.loggedIn, d.saved.ServerURL, d.accessToken
	d.mu.Unlock()
	if !loggedIn {
		return errNotLoggedIn
	}
	err := d.server.send(ctx, method, baseURL+path, token, in, out)
	if api.CodeOf(err) != api.ErrTokenInvalid {
		return err
	}
	if token, err = d.renewSession(ctx, token); err != nil {
		return err
	}
	return d.server.send(ctx, method, baseURL+path, token, in, out)
}

// renewSession replaces the rejected access token stale with a fresh one and
// returns it. Concurrent callers share one renewal, since a refresh token can
// only be used once. If the refresh token is no longer accepted the daemon
// logs out.
func (d *Daemon) renewSession(ctx context.Context, stale string) (string, error) {
	d.renewMu.Lock()
	defer d.renewMu.Unlock()

	d.mu.Lock()
	saved, current, loggedIn := d.saved, d.accessToken, d.loggedIn
	d.mu.Unlock()
	if !loggedIn {
		return "", errNotLoggedIn
	}
	if current != stale {
		return current, nil // renewed by a concurrent call
	}

	sess, err := d.server.refresh(ctx, saved.ServerURL, saved.RefreshToken)

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.saved.ServerURL != saved.ServerURL || d.saved.RefreshToken != saved.RefreshToken {
		return "", errNotLoggedIn // logged out or switched server meanwhile
	}
	if err != nil {
		if code := api.CodeOf(err); code == api.ErrTokenInvalid || code == api.ErrAccountDisabled {
			if saveErr := d.logOutLocally(); saveErr != nil {
				return "", saveErr
			}
		}
		return "", err
	}
	if err := d.acceptSession(sess); err != nil {
		return "", err
	}
	return sess.AccessToken, nil
}
