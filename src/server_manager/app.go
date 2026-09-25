package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct -- bound to the frontend by main.go. Every exported method
// here is callable from TypeScript via the generated wailsjs bindings.
type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetSettings() (Settings, error) {
	return loadSettings()
}

func (a *App) SaveSettings(s Settings) error {
	return saveSettingsToDisk(s)
}

func (a *App) GetServerConfig() (ServerConfig, error) {
	return loadServerConfig()
}

func (a *App) SaveServerConfig(c ServerConfig) error {
	return saveServerConfigToDisk(c)
}

// BrowseForArmaServerPath opens a native folder picker -- faster and less
// error-prone than asking the user to type/paste an absolute path.
func (a *App) BrowseForArmaServerPath() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select your Arma 3 Server install folder",
	})
}
