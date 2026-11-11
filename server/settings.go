package server

import (
	"database/sql"
	"errors"
)

// System settings are adjustable limits stored as integers by name. A setting
// that was never written has its default value. The admin API to read and
// change them comes with the administration ticket (#10).
const settingsSchema = `
CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value INTEGER NOT NULL
);
`

type setting struct {
	key string
	def int
}

var (
	// settingRoomsPerUser is how many rooms one user may own.
	settingRoomsPerUser = setting{key: "rooms_per_user", def: 5}
	// settingRoomCapacity is how many devices one room may hold.
	settingRoomCapacity = setting{key: "room_capacity", def: 32}
)

// queryer is satisfied by *sql.DB and *sql.Tx.
type queryer interface {
	QueryRow(query string, args ...any) *sql.Row
}

func getSetting(q queryer, st setting) (int, error) {
	var v int
	err := q.QueryRow(`SELECT value FROM settings WHERE key = ?`, st.key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return st.def, nil
	}
	return v, err
}

func (s *Server) setSetting(st setting, v int) error {
	_, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, st.key, v)
	return err
}
