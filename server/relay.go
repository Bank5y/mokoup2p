package server

import (
	"crypto/subtle"
	"net/http"
	"slices"
	"strings"

	"github.com/Bank5y/mokoup2p/api"
)

// relayRoutes serves the relay whitelist to the relay wrapper
// (cmd/mokoup2p-relay). See docs/adr/0002-relay-whitelist-sync.md.
func (s *Server) relayRoutes() {
	if s.relayToken == "" {
		return
	}
	s.mux.HandleFunc("GET "+api.PathRelayNetworks, s.handleRelayNetworks)
}

func (s *Server) handleRelayNetworks(w http.ResponseWriter, r *http.Request) {
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.relayToken)) != 1 {
		writeError(w, &api.Error{Code: api.ErrTokenInvalid, Message: "invalid relay token"})
		return
	}
	names, err := s.relayNetworks()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, api.RelayNetworks{Networks: names})
}

// relayNetworks returns the network names of every existing room, whether
// or not anybody is in it, so the whitelist only changes when rooms are
// created or deleted.
func (s *Server) relayNetworks() ([]string, error) {
	rows, err := s.db.Query(`SELECT id, code FROM rooms`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var r roomRow
		if err := rows.Scan(&r.id, &r.code); err != nil {
			return nil, err
		}
		names = append(names, networkName(r))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	slices.Sort(names)
	return names, nil
}
