package main

import (
	"context"
	_ "embed"
	"log"
	"runtime"
	"sync"
	"time"

	"fyne.io/systray"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Windows wants .ico bytes for the tray icon; Linux takes PNG.
var (
	//go:embed icons/tray.ico
	trayIconICO []byte
	//go:embed icons/tray.png
	trayIconPNG []byte
)

// quitLeaveTimeout bounds how long the tray "退出" waits for the daemon to
// leave the room before the GUI exits anyway.
const quitLeaveTimeout = 15 * time.Second

// startTray runs the system tray in its own goroutine. systray.Run creates
// its hidden window and pumps messages on the calling OS thread (Windows), so
// that goroutine is locked to its thread. Wails keeps its own main thread.
func (a *App) startTray() {
	go func() {
		runtime.LockOSThread()
		systray.Run(a.onTrayReady, func() {})
	}()
}

func (a *App) stopTray() { systray.Quit() }

func (a *App) onTrayReady() {
	if runtime.GOOS == "windows" {
		systray.SetIcon(trayIconICO)
	} else {
		systray.SetIcon(trayIconPNG)
	}
	systray.SetTitle(text("app.name"))
	systray.SetTooltip(text("app.name"))
	systray.SetOnTapped(a.showWindow) // left click, where the platform supports it

	show := systray.AddMenuItem(text("tray.show"), "")
	systray.AddSeparator()
	quit := systray.AddMenuItem(text("tray.quit"), "")
	go func() {
		for {
			select {
			case <-show.ClickedCh:
				a.showWindow()
			case <-quit.ClickedCh:
				quit.Disable()
				a.quitFromTray()
				return
			}
		}
	}()
}

func (a *App) showWindow() {
	wruntime.WindowUnminimise(a.ctx)
	wruntime.WindowShow(a.ctx)
}

var quitOnce sync.Once

// quitFromTray is the only real exit while the tray is available: the daemon
// leaves the current room (stopping the engine and cleaning up the virtual
// network), then the GUI quits. The leaving itself is the daemon's job; if
// the daemon cannot be reached the GUI still quits.
func (a *App) quitFromTray() {
	quitOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), quitLeaveTimeout)
		defer cancel()
		if _, err := a.ipc.Leave(ctx); err != nil {
			log.Printf("leave room on quit: %v", err)
		}
		wruntime.Quit(a.ctx)
	})
}
