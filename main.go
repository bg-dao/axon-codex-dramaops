package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"os"

	"github.com/bg-dao/axon-codex-dramaops/internal/appapi"
	"github.com/bg-dao/axon-codex-dramaops/internal/mcp"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

var assets embed.FS

func main() {
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		flags := flag.NewFlagSet("dramaops mcp", flag.ContinueOnError)
		root := flags.String("project", "", "absolute DramaOps project root")
		if err := flags.Parse(os.Args[2:]); err != nil || *root == "" {
			fmt.Fprintln(os.Stderr, "usage: DramaOps mcp --project /absolute/project/path")
			os.Exit(2)
		}
		if err := mcp.Run(context.Background(), *root, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	backend := appapi.NewBackend()
	projectAPI := appapi.NewProjectAPI(backend)
	agentAPI := appapi.NewAgentAPI(backend)
	runtimeAPI := appapi.NewRuntimeAPI(backend)
	settingsAPI := appapi.NewSettingsAPI(backend)
	assetAPI := appapi.NewAssetAPI(backend)
	storyAPI := appapi.NewStoryAPI(backend)
	renderAPI := appapi.NewRenderAPI(backend)

	err := wails.Run(&options.App{
		Title:            "DramaOps by Axon",
		Width:            1480,
		Height:           940,
		MinWidth:         1180,
		MinHeight:        720,
		Frameless:        false,
		BackgroundColour: &options.RGBA{R: 7, G: 16, B: 15, A: 1},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        backend.Startup,
		OnShutdown:       backend.Shutdown,
		Bind:             []interface{}{projectAPI, storyAPI, assetAPI, renderAPI, agentAPI, runtimeAPI, settingsAPI},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
