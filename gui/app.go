package main

import (
	"context"
	"errors"
	"time"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

// errDaemonUnreachable is reported when the background service is not running.
const errDaemonUnreachable api.ErrorCode = "daemon_unreachable"

// App is bound to the frontend. It only relays calls to the daemon over IPC
// and holds no business rules. Errors reach the frontend as their error code
// so the UI can pick the matching text from its resource file.
type App struct {
	ctx context.Context
	ipc *ipc.Client
}

func NewApp() *App {
	return &App{ipc: ipc.NewClient(ipc.DefaultSocketPath())}
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func (a *App) Status() (ipc.Status, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	st, err := a.ipc.Status(ctx)
	return st, codeError(err)
}

func (a *App) SetServer(url string) (ipc.Status, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	st, err := a.ipc.SetServer(ctx, url)
	return st, codeError(err)
}

func (a *App) Login(username, password string) (ipc.Status, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	st, err := a.ipc.Login(ctx, username, password)
	return st, codeError(err)
}

func (a *App) timeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(a.ctx, 30*time.Second)
}

func codeError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		return errors.New(string(apiErr.Code))
	}
	return errors.New(string(errDaemonUnreachable))
}
