package main

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/ipc"
	"github.com/Bank5y/mokoup2p/client/updater"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Error codes of InstallUpdate, for the frontend's resource file.
const (
	errUpdateUnsupported      api.ErrorCode = "update_unsupported"
	errUpdateDownloadFailed   api.ErrorCode = "update_download_failed"
	errUpdateChecksumMismatch api.ErrorCode = "update_checksum_mismatch"
	errUpdateCancelled        api.ErrorCode = "update_cancelled"
	errUpdateLaunchFailed     api.ErrorCode = "update_launch_failed"
)

// errElevationCancelled is returned by runElevated when the user declines
// the UAC prompt.
var errElevationCancelled = errors.New("elevation cancelled")

// CheckUpdate asks the daemon for the newest release.
func (a *App) CheckUpdate() (ipc.UpdateInfo, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	info, err := a.ipc.CheckUpdate(ctx)
	return info, codeError(err)
}

func (a *App) UpdateSettings() (ipc.UpdateSettings, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	s, err := a.ipc.UpdateSettings(ctx)
	return s, codeError(err)
}

func (a *App) SetUpdateSettings(autoCheck bool, ignoredVersion string) (ipc.UpdateSettings, error) {
	ctx, cancel := a.timeout()
	defer cancel()
	s, err := a.ipc.SetUpdateSettings(ctx, ipc.UpdateSettings{AutoCheck: autoCheck, IgnoredVersion: ignoredVersion})
	return s, codeError(err)
}

// OpenURL opens url in the default browser.
func (a *App) OpenURL(url string) {
	wruntime.BrowserOpenURL(a.ctx, url)
}

// InstallUpdate updates the Windows installer edition in one go: it checks
// again, downloads the installer and verifies it against SHA256SUMS, then
// runs it elevated (UAC) as a silent install that reopens the GUI when done,
// and quits. The service restarts during the install, so the device leaves
// its room; the frontend says so before calling this.
func (a *App) InstallUpdate() error {
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()
	info, err := a.ipc.CheckUpdate(ctx)
	if err != nil {
		return codeError(err)
	}
	st, err := a.ipc.Status(ctx)
	if err != nil {
		return codeError(err)
	}
	if st.Edition != ipc.EditionWindowsInstaller || !info.Available || info.AssetURL == "" || info.ChecksumsURL == "" {
		return errors.New(string(errUpdateUnsupported))
	}
	dir := filepath.Join(os.TempDir(), "mokoup2p-update")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errors.New(string(errUpdateDownloadFailed))
	}
	path, err := updater.Download(ctx, updater.Asset{Name: info.AssetName, URL: info.AssetURL, ChecksumsURL: info.ChecksumsURL}, dir)
	switch {
	case errors.Is(err, updater.ErrChecksumMismatch), errors.Is(err, updater.ErrNotInChecksums):
		log.Printf("update: %v", err)
		return errors.New(string(errUpdateChecksumMismatch))
	case err != nil:
		log.Printf("update: %v", err)
		return errors.New(string(errUpdateDownloadFailed))
	}
	switch err := runElevated(path, "/S /relaunch"); {
	case errors.Is(err, errElevationCancelled):
		return errors.New(string(errUpdateCancelled))
	case err != nil:
		log.Printf("update: run installer: %v", err)
		return errors.New(string(errUpdateLaunchFailed))
	}
	// The installer closes the GUI too; quitting first lets it go cleanly.
	go wruntime.Quit(a.ctx)
	return nil
}

// DependencyMissing reports whether a component the network engine needs
// (Npcap on Windows) is not installed, so the GUI can say so up front.
func (a *App) DependencyMissing() bool { return dependencyMissing() }
