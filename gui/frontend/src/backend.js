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
