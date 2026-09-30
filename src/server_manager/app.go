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
	go a.startControlAPI(ctx)
}

func (a *App) GetSettings() (Settings, error) {
	return loadSettings()
}

// SaveSettings saves the Settings form. The form doesn't show the control
// API fields, so they're kept from what's already saved rather than wiped.
func (a *App) SaveSettings(s Settings) error {
	if cur, err := loadSettings(); err == nil {
		if s.ControlAPIToken == "" {
			s.ControlAPIToken = cur.ControlAPIToken
		}
		if s.ControlAPIAddr == "" {
			s.ControlAPIAddr = cur.ControlAPIAddr
		}
		if !s.ControlAPIDisabled {
			s.ControlAPIDisabled = cur.ControlAPIDisabled
		}
	}
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
