package ipc

import (
	"context"
	"net/http"

	"github.com/Bank5y/mokoup2p/api"
)

const (
	PathUpdate         = "/v1/update"
	PathUpdateSettings = "/v1/update/settings"
)

// ErrUpdateCheckFailed means the release feed could not be read.
const ErrUpdateCheckFailed api.ErrorCode = "update_check_failed"

// Edition is how this client was packaged; it decides which release asset
// an update offers.
type Edition string

const (
	EditionWindowsInstaller Edition = "windows-installer"
	EditionWindowsPortable  Edition = "windows-portable"
	EditionLinux            Edition = "linux"
)

// UpdateInfo is the result of checking for a newer release.
type UpdateInfo struct {
	CurrentVersion string `json:"currentVersion"`
	// LatestVersion is the newest published (not draft, not prerelease)
	// release, without the leading "v"; empty if there is none.
	LatestVersion string `json:"latestVersion,omitempty"`
	// Available is true when LatestVersion is newer than CurrentVersion.
	// A development build is never offered an update.
	Available bool `json:"available"`
	// Ignored is true when LatestVersion is the version the user chose to
	// ignore.
	Ignored bool `json:"ignored,omitempty"`
	// ReleaseURL is the release's page (notes and every download).
	ReleaseURL string `json:"releaseUrl,omitempty"`
	// AssetName and AssetURL are the package for this edition; empty on
	// editions that do not update themselves (Linux).
	AssetName string `json:"assetName,omitempty"`
	AssetURL  string `json:"assetUrl,omitempty"`
	// ChecksumsURL is the release's SHA256SUMS file.
	ChecksumsURL string `json:"checksumsUrl,omitempty"`
}

type UpdateSettings struct {
	// AutoCheck makes the GUI check for updates when it starts.
	AutoCheck bool `json:"autoCheck"`
	// IgnoredVersion is a release the user does not want to be told about.
	IgnoredVersion string `json:"ignoredVersion,omitempty"`
}

// CheckUpdate asks the daemon to look up the newest release now.
func (c *Client) CheckUpdate(ctx context.Context) (UpdateInfo, error) {
	var info UpdateInfo
	err := c.do(ctx, http.MethodGet, PathUpdate, nil, &info)
	return info, err
}

func (c *Client) UpdateSettings(ctx context.Context) (UpdateSettings, error) {
	var s UpdateSettings
	err := c.do(ctx, http.MethodGet, PathUpdateSettings, nil, &s)
	return s, err
}

func (c *Client) SetUpdateSettings(ctx context.Context, s UpdateSettings) (UpdateSettings, error) {
	var out UpdateSettings
	err := c.do(ctx, http.MethodPut, PathUpdateSettings, s, &out)
	return out, err
}
