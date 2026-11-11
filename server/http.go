package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/Bank5y/mokoup2p/api"
)

func decode(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return &api.Error{Code: api.ErrBadRequest, Message: "invalid JSON body"}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

var statusByCode = map[api.ErrorCode]int{
	api.ErrBadRequest:         http.StatusBadRequest,
	api.ErrInvalidCredentials: http.StatusUnauthorized,
	api.ErrTokenInvalid:       http.StatusUnauthorized,
	api.ErrAccountDisabled:    http.StatusForbidden,
}

// writeError writes err as an api.Error. Errors that are not api.Errors are
// logged and reported as internal errors.
func writeError(w http.ResponseWriter, err error) {
	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		log.Printf("internal error: %v", err)
		apiErr = &api.Error{Code: api.ErrInternal, Message: "internal server error"}
	}
	status, ok := statusByCode[apiErr.Code]
	if !ok {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, apiErr)
}
