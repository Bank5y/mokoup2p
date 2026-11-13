package main

import (
	"github.com/Bank5y/mokoup2p/client/ipc"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

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
