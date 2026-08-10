package main

import (
	"embed"
	"flag"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS
// 快速排序

func main() {
	workspaceFlag := flag.String("workspace", "", "Workspace path to open on startup")
	flag.Parse()

	app := NewApp()
	app.startupWorkspace = *workspaceFlag

	err := wails.Run(&options.App{
		Title:     "aimuxterm - 智能终端",
		Width:     1400,
		Height:    900,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:         &options.RGBA{R: 22, G: 22, B: 24, A: 1},
		EnableDefaultContextMenu: true,
		OnStartup:                app.startup,
		OnShutdown:               app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
