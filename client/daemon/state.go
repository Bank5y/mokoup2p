package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// persisted is what the daemon remembers across restarts.
type persisted struct {
	ServerURL    string `json:"serverUrl,omitempty"`
	DeviceID     string `json:"deviceId"`
	DeviceName   string `json:"deviceName"`
	RefreshToken string `json:"refreshToken,omitempty"`
	// NoAutoUpdateCheck is stored inverted so that a missing field means
	// the default, checking automatically.
	NoAutoUpdateCheck    bool   `json:"noAutoUpdateCheck,omitempty"`
	IgnoredUpdateVersion string `json:"ignoredUpdateVersion,omitempty"`
}

const stateFile = "state.json"

// loadState reads the saved state, creating a device identity on first run.
func loadState(dir string, hostname func() (string, error)) (persisted, error) {
	var st persisted
	data, err := os.ReadFile(filepath.Join(dir, stateFile))
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return st, err
	default:
		if err := json.Unmarshal(data, &st); err != nil {
			return st, err
		}
	}
	if st.DeviceID != "" {
		return st, nil
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return st, err
	}
	st.DeviceID = hex.EncodeToString(buf)
	if st.DeviceName, err = hostname(); err != nil || st.DeviceName == "" {
		st.DeviceName = "device-" + st.DeviceID[:6]
	}
	return st, saveState(dir, st)
}

// saveState writes the state atomically; the file holds the refresh token so
// it is readable by the owner only.
func saveState(dir string, st persisted) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, stateFile+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, stateFile))
}
