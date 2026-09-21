package main

import (
	"embed"

	"nte-optimizer/internal/buildinfo"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	projectDir, err := resolveProjectDir()
	if err != nil {
		panic(err)
	}
	userDataDir, err := resolveUserDataDir()
	if err != nil {
		panic(err)
	}
	if err := prepareUserDataDir(projectDir, userDataDir); err != nil {
		panic(err)
	}
	app := NewDesktopAppWithStateDir(projectDir, userDataDir)
	if err := wails.Run(&options.App{
		Title:            buildinfo.WindowTitle("NTE Optimizer"),
		Width:            1180,
		Height:           760,
		MinWidth:         900,
		MinHeight:        600,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 12, G: 15, B: 22, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
	}); err != nil {
		panic(err)
	}
}
