// Jacob Helpa - a desktop helper app.
//
// The whole program is one executable: the Go code below, and the interface in
// frontend/dist which is embedded into the binary. Wails renders that interface
// in a native window using the system WebView, so there is no browser, no local
// server and nothing to install.
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

const (
	appName = "Jacob Helpa"
	version = "3.0.0"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	logFile := setupLogging()
	if logFile != nil {
		defer logFile.Close()
	}
	log.Printf("%s %s starting", appName, version)

	app, err := newApp()
	if err != nil {
		log.Printf("fatal: %v", err)
		return
	}

	err = wails.Run(&options.App{
		Title:            appName,
		Width:            1280,
		Height:           860,
		MinWidth:         900,
		MinHeight:        620,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 7, G: 10, B: 18, A: 1},
		OnStartup:        app.startup,
		Bind:             []any{app},
		Windows: &windows.Options{
			// The welcome animation draws its own background, so letting the
			// window go translucent would show the desktop through it.
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})
	if err != nil {
		log.Printf("could not open the window: %v", err)
	}
}
