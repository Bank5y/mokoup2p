package main

import (
	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

// Account and administrator bindings. Like the rest of App they only relay
// to the daemon.

func (a *App) ChangePassword(oldPassword, newPassword string) error {
	ctx, cancel := a.timeout()
	defer cancel()
	return codeError(a.ipc.ChangePassword(ctx, oldPassword, newPassword))
}

func (a *App) SetDeviceName(name string) (ipc.Status, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	st, err := a.ipc.SetDeviceName(ctx, name)
	return st, codeError(err)
}

func (a *App) Logout() (ipc.Status, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	st, err := a.ipc.Logout(ctx)
	return st, codeError(err)
}

func (a *App) ListUsers() ([]api.AdminUser, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	users, err := a.ipc.ListUsers(ctx)
	return users, codeError(err)
}

func (a *App) CreateUser(username, password string) (api.AdminUser, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	u, err := a.ipc.CreateUser(ctx, username, password)
	return u, codeError(err)
}

func (a *App) ResetUserPassword(id int64, password string) (api.AdminUser, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	u, err := a.ipc.ResetUserPassword(ctx, id, password)
	return u, codeError(err)
}

func (a *App) SetUserDisabled(id int64, disabled bool) (api.AdminUser, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	u, err := a.ipc.SetUserDisabled(ctx, id, disabled)
	return u, codeError(err)
}

func (a *App) SetUserAdmin(id int64, isAdmin bool) (api.AdminUser, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	u, err := a.ipc.SetUserAdmin(ctx, id, isAdmin)
	return u, codeError(err)
}

func (a *App) DeleteUser(id int64) error {
	ctx, cancel := a.timeout()
	defer cancel()
	return codeError(a.ipc.DeleteUser(ctx, id))
}
