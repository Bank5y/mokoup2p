// Calls into the Go App bound by Wails (window.go.main.App). Rejections carry
// the error code as the message.
const app = () => window.go.main.App

export const status = () => app().Status()
export const setServer = (url) => app().SetServer(url)
export const login = (username, password) => app().Login(username, password)

export const createRoom = (name, password) => app().CreateRoom(name, password)
export const myRooms = () => app().MyRooms()
export const renameRoom = (code, name) => app().RenameRoom(code, name)
export const setRoomPassword = (code, password) => app().SetRoomPassword(code, password)
export const deleteRoom = (code) => app().DeleteRoom(code)

// copyText puts text on the system clipboard through the Wails runtime.
export async function copyText(text) {
  if (window.runtime?.ClipboardSetText && (await window.runtime.ClipboardSetText(text))) return
  await navigator.clipboard.writeText(text)
}
