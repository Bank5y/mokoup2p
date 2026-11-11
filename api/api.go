// Package api holds the HTTP JSON types shared by the server and the client
// daemon. It contains no behaviour.
package api

import (
	"errors"
	"time"
)

// Paths of the server HTTP API.
const (
	PathLogin   = "/api/v1/auth/login"
	PathRefresh = "/api/v1/auth/refresh"
	PathMe      = "/api/v1/me"
)

// ErrorCode distinguishes failures so clients can react to them.
type ErrorCode string

const (
	ErrBadRequest         ErrorCode = "bad_request"
	ErrInvalidCredentials ErrorCode = "invalid_credentials"
	ErrAccountDisabled    ErrorCode = "account_disabled"
	ErrTokenInvalid       ErrorCode = "token_invalid"
	ErrInternal           ErrorCode = "internal"
)

// Error is the body of every non-2xx response.
type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }

// CodeOf returns the code of an *Error in err's chain, or "" if there is none.
func CodeOf(err error) ErrorCode {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return ""
}

type LoginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	DeviceID   string `json:"deviceId"`
	DeviceName string `json:"deviceName"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"isAdmin"`
}

type Device struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Session is returned by login and refresh.
type Session struct {
	AccessToken          string    `json:"accessToken"`
	AccessTokenExpiresAt time.Time `json:"accessTokenExpiresAt"`
	RefreshToken         string    `json:"refreshToken"`
	User                 User      `json:"user"`
	Device               Device    `json:"device"`
}

// Me is returned by GET PathMe.
type Me struct {
	User   User   `json:"user"`
	Device Device `json:"device"`
}
