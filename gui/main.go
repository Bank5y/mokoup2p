// Command mokoup2p-gui is the desktop front end. It talks to the client
// daemon over IPC only.
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	if !startPortable(app.ipc) {
		return
	}
	err := wails.Run(&options.App{
		Title:            "mokoup2p",
		Width:            960,
		Height:           640,
		MinWidth:         720,
		MinHeight:        480,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 255},
		// With a tray, × hides the window to the tray and the device stays in
		// its room. Without one (e.g. stock GNOME) × closes the GUI so the
		// window can't get lost; the daemon still keeps the room.
		HideWindowOnClose: app.tray,
		// Launching the GUI again brings the hidden window back.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "mokoup2p-gui",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) { app.showWindow() },
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind:       []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
