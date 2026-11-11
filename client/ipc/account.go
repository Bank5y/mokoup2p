package ipc

import (
	"context"
	"net/http"
	"strconv"

	"github.com/Bank5y/mokoup2p/api"
)

const (
	PathPassword   = "/v1/password"
	PathDeviceName = "/v1/device-name"
	PathLogout     = "/v1/logout"

	// PathAdminUsers and everything below it are relayed to the server's
	// administrator user API (api.PathAdminUsers) unchanged.
	PathAdminUsers = "/v1/admin/users"
)

// PathAdminUser is the IPC path of one user; see api.PathAdminUser.
func PathAdminUser(id int64) string { return PathAdminUsers + "/" + strconv.FormatInt(id, 10) }

type SetDeviceNameRequest struct {
	Name string `json:"name"`
}

// ChangePassword changes the logged-in user's password.
func (c *Client) ChangePassword(ctx context.Context, oldPassword, newPassword string) error {
	return c.do(ctx, http.MethodPut, PathPassword,
		api.ChangePasswordRequest{OldPassword: oldPassword, NewPassword: newPassword}, nil)
}

// SetDeviceName renames this device. When logged out the name is saved and
// used at the next login.
func (c *Client) SetDeviceName(ctx context.Context, name string) (Status, error) {
	var st Status
	err := c.do(ctx, http.MethodPut, PathDeviceName, SetDeviceNameRequest{Name: name}, &st)
	return st, err
}

// Logout revokes this device's session and returns the daemon to logged out.
func (c *Client) Logout(ctx context.Context) (Status, error) {
	var st Status
	err := c.do(ctx, http.MethodPost, PathLogout, nil, &st)
	return st, err
}

func (c *Client) ListUsers(ctx context.Context) ([]api.AdminUser, error) {
	var users []api.AdminUser
	err := c.do(ctx, http.MethodGet, PathAdminUsers, nil, &users)
	return users, err
}

func (c *Client) CreateUser(ctx context.Context, username, password string) (api.AdminUser, error) {
	var u api.AdminUser
	err := c.do(ctx, http.MethodPost, PathAdminUsers, api.CreateUserRequest{Username: username, Password: password}, &u)
	return u, err
}

func (c *Client) ResetUserPassword(ctx context.Context, id int64, password string) (api.AdminUser, error) {
	var u api.AdminUser
	err := c.do(ctx, http.MethodPut, PathAdminUser(id)+api.AdminUserPassword, api.ResetPasswordRequest{Password: password}, &u)
	return u, err
}

func (c *Client) SetUserDisabled(ctx context.Context, id int64, disabled bool) (api.AdminUser, error) {
	var u api.AdminUser
	err := c.do(ctx, http.MethodPut, PathAdminUser(id)+api.AdminUserDisabled, api.SetDisabledRequest{Disabled: disabled}, &u)
	return u, err
}

func (c *Client) SetUserAdmin(ctx context.Context, id int64, isAdmin bool) (api.AdminUser, error) {
	var u api.AdminUser
	err := c.do(ctx, http.MethodPut, PathAdminUser(id)+api.AdminUserAdmin, api.SetAdminRequest{IsAdmin: isAdmin}, &u)
	return u, err
}

func (c *Client) DeleteUser(ctx context.Context, id int64) error {
	return c.do(ctx, http.MethodDelete, PathAdminUser(id), nil, nil)
}
