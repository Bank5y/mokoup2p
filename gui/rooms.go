package main

import "github.com/Bank5y/mokoup2p/api"

func (a *App) CreateRoom(name, password string) (api.Room, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	room, err := a.ipc.CreateRoom(ctx, api.CreateRoomRequest{Name: name, Password: password})
	return room, codeError(err)
}

func (a *App) MyRooms() ([]api.Room, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	rooms, err := a.ipc.MyRooms(ctx)
	return rooms, codeError(err)
}

func (a *App) RenameRoom(code, name string) (api.Room, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	room, err := a.ipc.UpdateRoom(ctx, code, api.UpdateRoomRequest{Name: &name})
	return room, codeError(err)
}

// SetRoomPassword sets or changes the room password; "" removes it.
func (a *App) SetRoomPassword(code, password string) (api.Room, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	room, err := a.ipc.UpdateRoom(ctx, code, api.UpdateRoomRequest{Password: &password})
	return room, codeError(err)
}

func (a *App) DeleteRoom(code string) error {
	ctx, cancel := a.timeout()
	defer cancel()
	return codeError(a.ipc.DeleteRoom(ctx, code))
}
