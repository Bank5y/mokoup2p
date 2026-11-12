// Calls into the Go App bound by Wails (window.go.main.App). Rejections carry
// the error code as the message.
const app = () => window.go.main.App

export const status = () => app().Status()
export const setServer = (url) => app().SetServer(url)
export const login = (username, password) => app().Login(username, password)
export const changePassword = (oldPassword, newPassword) => app().ChangePassword(oldPassword, newPassword)
export const setDeviceName = (name) => app().SetDeviceName(name)
export const logout = () => app().Logout()

export const listUsers = () => app().ListUsers()
export const createUser = (username, password) => app().CreateUser(username, password)
export const resetUserPassword = (id, password) => app().ResetUserPassword(id, password)
export const setUserDisabled = (id, disabled) => app().SetUserDisabled(id, disabled)
export const setUserAdmin = (id, isAdmin) => app().SetUserAdmin(id, isAdmin)
export const deleteUser = (id) => app().DeleteUser(id)

export const createRoom = (name, password) => app().CreateRoom(name, password)
export const myRooms = () => app().MyRooms()
export const renameRoom = (code, name) => app().RenameRoom(code, name)
export const setRoomPassword = (code, password) => app().SetRoomPassword(code, password)
export const deleteRoom = (code) => app().DeleteRoom(code)

export const join = (code, password) => app().Join(code, password)
export const leave = () => app().Leave()
export const members = () => app().Members()
// waitStatus resolves with the status once its revision differs from the
// given one (or after the daemon's long-poll period).
export const waitStatus = (revision) => app().WaitStatus(revision)

// copyText puts text on the system clipboard through the Wails runtime.
export async function copyText(text) {
  if (window.runtime?.ClipboardSetText && (await window.runtime.ClipboardSetText(text))) return
  await navigator.clipboard.writeText(text)
}
export const kick = (memberId) => app().Kick(memberId)
