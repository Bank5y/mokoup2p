package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

func (d *Daemon) updateRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+ipc.PathUpdate, d.handleCheckUpdate)
	mux.HandleFunc("GET "+ipc.PathUpdateSettings, d.handleUpdateSettings)
	mux.HandleFunc("PUT "+ipc.PathUpdateSettings, d.handleSetUpdateSettings)
}

// githubRelease is the part of GitHub's release object the daemon reads.
type githubRelease struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func (d *Daemon) handleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	latest, err := d.latestRelease(r.Context())
	if err != nil {
		writeError(w, &api.Error{Code: ipc.ErrUpdateCheckFailed, Message: err.Error()})
		return
	}
	d.mu.Lock()
	ignored := d.saved.IgnoredUpdateVersion
	d.mu.Unlock()

	info := ipc.UpdateInfo{CurrentVersion: d.cfg.Version}
	if latest != nil {
		info.LatestVersion = strings.TrimPrefix(latest.TagName, "v")
		info.ReleaseURL = latest.HTMLURL
		current := "v" + d.cfg.Version
		info.Available = semver.IsValid(current) && semver.Compare(latest.TagName, current) > 0
		info.Ignored = info.LatestVersion == ignored
		if want := assetName(d.cfg.Edition, info.LatestVersion); want != "" {
			for _, a := range latest.Assets {
				switch a.Name {
				case want:
					info.AssetName, info.AssetURL = a.Name, a.URL
				case "SHA256SUMS":
					info.ChecksumsURL = a.URL
				}
			}
		}
		info.Installable = d.cfg.Edition == ipc.EditionWindowsInstaller && info.AssetURL != "" && info.ChecksumsURL != ""
	}
	writeJSON(w, http.StatusOK, info)
}

// assetName is the release file that updates edition; empty for editions
// that are only pointed at the release page.
func assetName(edition ipc.Edition, version string) string {
	switch edition {
	case ipc.EditionWindowsInstaller:
		return "mokoup2p-" + version + "-windows-x64-setup.exe"
	case ipc.EditionWindowsPortable:
		return "mokoup2p-" + version + "-windows-x64-portable.zip"
	}
	return ""
}

// latestRelease returns the newest published release, or nil if there is
// none. Tags that are not semantic versions are skipped.
func (d *Daemon) latestRelease(ctx context.Context) (*githubRelease, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.cfg.ReleasesURL+"?per_page=30", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("read release feed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("read release feed: %s", resp.Status)
	}
	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("read release feed: %w", err)
	}
	var latest *githubRelease
	for i := range releases {
		rel := &releases[i]
		if rel.Draft || rel.Prerelease || !semver.IsValid(rel.TagName) || semver.Prerelease(rel.TagName) != "" {
			continue
		}
		if latest == nil || semver.Compare(rel.TagName, latest.TagName) > 0 {
			latest = rel
		}
	}
	return latest, nil
}

func (d *Daemon) updateSettings() ipc.UpdateSettings {
	return ipc.UpdateSettings{AutoCheck: !d.saved.NoAutoUpdateCheck, IgnoredVersion: d.saved.IgnoredUpdateVersion}
}

func (d *Daemon) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	s := d.updateSettings()
	d.mu.Unlock()
	writeJSON(w, http.StatusOK, s)
}

func (d *Daemon) handleSetUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req ipc.UpdateSettings
	if !decode(w, r, &req) {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.saved.NoAutoUpdateCheck = !req.AutoCheck
	d.saved.IgnoredUpdateVersion = strings.TrimPrefix(strings.TrimSpace(req.IgnoredVersion), "v")
	if err := saveState(d.cfg.StateDir, d.saved); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d.updateSettings())
}
