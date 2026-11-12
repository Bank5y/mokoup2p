package api

import "strconv"

// Paths for a user's own account and for administrator user management.
const (
	PathMePassword = "/api/v1/me/password"
	PathMeDevice   = "/api/v1/me/device"
	PathLogout     = "/api/v1/auth/logout"

	// PathAdminUsers lists (GET) and creates (POST) users. Single-user
	// operations live below PathAdminUser(id).
	PathAdminUsers = "/api/v1/admin/users"
)

// PathAdminUser is the path of one user: DELETE removes it, and the
// suffixes AdminUserPassword, AdminUserDisabled and AdminUserAdmin (PUT)
// reset its password, disable/enable it and grant/revoke administrator.
func PathAdminUser(id int64) string { return PathAdminUsers + "/" + strconv.FormatInt(id, 10) }

const (
	AdminUserPassword = "/password"
	AdminUserDisabled = "/disabled"
	AdminUserAdmin    = "/admin"
)

const (
	ErrForbidden     ErrorCode = "forbidden"      // caller is not an administrator
	ErrUserNotFound  ErrorCode = "user_not_found" // no user with that id
	ErrUsernameTaken ErrorCode = "username_taken" // username already exists
	ErrLastAdmin     ErrorCode = "last_admin"     // would leave no enabled administrator
)

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type RenameDeviceRequest struct {
	Name string `json:"name"`
}

// AdminUser is a user as seen on the administrator page.
type AdminUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"isAdmin"`
	Disabled bool   `json:"disabled"`
}

type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ResetPasswordRequest struct {
	Password string `json:"password"`
}

type SetDisabledRequest struct {
	Disabled bool `json:"disabled"`
}

type SetAdminRequest struct {
	IsAdmin bool `json:"isAdmin"`
}
