package main

import "github.com/Bank5y/mokoup2p/client/ipc"

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
