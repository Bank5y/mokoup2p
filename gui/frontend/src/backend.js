// Calls into the Go App bound by Wails (window.go.main.App). Rejections carry
// the error code as the message.
const app = () => window.go.main.App

export const status = () => app().Status()
export const setServer = (url) => app().SetServer(url)
export const login = (username, password) => app().Login(username, password)
