package main

import (
	"context"
	"time"

	"github.com/Bank5y/mokoup2p/client/ipc"
)

func (a *App) Join(code, password string) (ipc.Status, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	st, err := a.ipc.Join(ctx, code, password)
	return st, codeError(err)
}

func (a *App) Leave() (ipc.Status, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	st, err := a.ipc.Leave(ctx)
	return st, codeError(err)
}

// Members lists the devices in the current room.
func (a *App) Members() ([]ipc.Member, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	members, err := a.ipc.Members(ctx)
	return members, codeError(err)
}

// WaitStatus returns the status once its revision differs from revision,
// or after the daemon's long-poll period. The frontend calls it in a loop
// to follow changes such as members joining.
func (a *App) WaitStatus(revision uint64) (ipc.Status, error) {
	ctx, cancel := context.WithTimeout(a.ctx, time.Minute)
	defer cancel()
	st, err := a.ipc.WaitStatus(ctx, revision)
	return st, codeError(err)
}
