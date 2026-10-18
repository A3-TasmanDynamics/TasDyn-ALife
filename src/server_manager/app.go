package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct -- bound to the frontend by main.go. Every exported method
// here is callable from TypeScript via the generated wailsjs bindings.
type App struct {
	ctx context.Context
}

// theApp is the running App, for code outside its methods (events).
var theApp *App

func NewApp() *App {
	theApp = &App{}
	return theApp
}

// The four services (services.go), in start order: the website first (the
// bot and the game both lean on the database and the site), the game last.
var (
	svcWebsite = &service{Key: "website", Title: "Website", start: websiteStart("web"),
		cfg: func(c ServicesConfig) ServiceConfig { return c.Website }}
	svcBot = &service{Key: "bot", Title: "Discord Bot", start: websiteStart("bot"),
		cfg: func(c ServicesConfig) ServiceConfig { return c.Bot }}
	svcTeamSpeak = &service{Key: "teamspeak", Title: "TeamSpeak 3", start: ts3Start,
		cfg: func(c ServicesConfig) ServiceConfig { return c.TeamSpeak }}
	svcArma = &service{Key: "arma", Title: "Arma 3 Server", start: armaStart,
		cfg: func(c ServicesConfig) ServiceConfig { return c.Arma }}

	allServices = []*service{svcWebsite, svcBot, svcTeamSpeak, svcArma}
)

// Hooks are set here, not above: they refer back to the services.
func init() {
	svcArma.onStarted, svcArma.onExited = armaStarted, armaExited
	svcArma.exePath = func() string {
		if st, err := loadSettings(); err == nil && st.ArmaServerPath != "" {
			return filepath.Join(st.ArmaServerPath, "arma3server_x64.exe")
		}
		return ""
	}
	svcArma.extraFor = latestArmaProfile
	svcWebsite.exePath = func() string { return websiteExe("web") }
	svcBot.exePath = func() string { return websiteExe("bot") }
	svcTeamSpeak.exePath = func() string {
		if c, err := loadServicesConfig(); err == nil && c.TS3Path != "" {
			return filepath.Join(c.TS3Path, "ts3server.exe")
		}
		return ""
	}
	svcTeamSpeak.onStarted = func(int, string, bool) {
		if p := svcTeamSpeak.logPath(); p != "" {
			go ts3WatchForKey(p)
		}
	}
}

func serviceByKey(key string) (*service, error) {
	for _, s := range allServices {
		if s.Key == key {
			return s, nil
		}
	}
	return nil, fmt.Errorf("%w %q", errUnknownService, key)
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go a.startControlAPI(ctx)
	go startServices()
}

// startServices re-attaches to anything a previous manager session left
// running, then auto-starts the rest that are set to.
func startServices() {
	state := loadRunState()
	for _, s := range allServices {
		if rec, ok := state[s.Key]; ok && !s.adopt(rec) {
			setRunState(s.Key, nil) // stale: that process is gone
		}
		if !s.Running() {
			s.findRunning()
		}
	}
	cfg, _ := loadServicesConfig()
	for _, s := range allServices {
		if s.cfg(cfg).AutoStart && !s.Running() {
			if _, err := s.Start(); err != nil {
				log.Printf("services: auto-start %s: %v", s.Title, err)
			}
		}
	}
}

// beforeClose: if anything is running, ask whether to stop it all or leave
// it running (the next manager session re-attaches). Returning true keeps
// the window open.
func (a *App) beforeClose(ctx context.Context) bool {
	var running []string
	for _, s := range allServices {
		if s.Running() {
			running = append(running, s.Title)
		}
	}
	if len(running) == 0 {
		return false
	}
	choice, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "Close the Server Manager",
		Message:       fmt.Sprintf("Still running: %s.\n\nStop them, or leave them running? (The manager picks them back up next time it opens.)", joinTitles(running)),
		Buttons:       []string{"Leave running", "Stop everything", "Cancel"},
		DefaultButton: "Leave running",
		CancelButton:  "Cancel",
	})
	if err != nil || choice == "Cancel" || choice == "" {
		return true
	}
	if choice == "Stop everything" {
		a.StopAllServices()
	}
	return false
}

func joinTitles(t []string) string {
	switch len(t) {
	case 1:
		return t[0]
	case 2:
		return t[0] + " and " + t[1]
	}
	out := ""
	for i, s := range t {
		if i == len(t)-1 {
			out += "and " + s
		} else {
			out += s + ", "
		}
	}
	return out
}

// ---- Services tab ----

func (a *App) GetServices() []ServiceStatus {
	out := make([]ServiceStatus, len(allServices))
	for i, s := range allServices {
		out[i] = s.Status()
	}
	return out
}

// GetServiceHealth is a live check of a running service (does the site
// answer, is the bot connected, how many are on TeamSpeak, server FPS).
func (a *App) GetServiceHealth(key string) string {
	s, err := serviceByKey(key)
	if err != nil || !s.Running() {
		return ""
	}
	switch key {
	case "website":
		return websiteHealth()
	case "bot":
		return botHealth()
	case "teamspeak":
		return ts3Health()
	case "arma":
		procMgr.fpsMu.Lock()
		fps := procMgr.lastFPS
		procMgr.fpsMu.Unlock()
		if fps < 0 {
			return "Loading the mission..."
		}
		return fmt.Sprintf("Server FPS %.0f", fps)
	}
	return ""
}

func (a *App) StartService(key string) ([]string, error) {
	s, err := serviceByKey(key)
	if err != nil {
		return nil, err
	}
	return s.Start()
}

func (a *App) StopService(key string) error {
	s, err := serviceByKey(key)
	if err != nil {
		return err
	}
	if key == "arma" {
		a.cancelPendingRestart()
	}
	return s.Stop()
}

func (a *App) RestartService(key string) ([]string, error) {
	s, err := serviceByKey(key)
	if err != nil {
		return nil, err
	}
	return s.Restart()
}

func (a *App) StartAllServices() []string {
	var errs []string
	for _, s := range allServices {
		if !s.Running() {
			if _, err := s.Start(); err != nil {
				errs = append(errs, s.Title+": "+err.Error())
			}
		}
	}
	return errs
}

func (a *App) StopAllServices() {
	a.cancelPendingRestart()
	var wg sync.WaitGroup
	for _, s := range allServices {
		if s.Running() {
			wg.Add(1)
			go func(s *service) {
				defer wg.Done()
				if err := s.Stop(); err != nil {
					log.Printf("services: stop %s: %v", s.Title, err)
				}
			}(s)
		}
	}
	wg.Wait()
}

func (a *App) SetServiceOptions(key string, autoStart, restartOnCrash bool) error {
	if _, err := serviceByKey(key); err != nil {
		return err
	}
	err := updateServicesConfig(func(c *ServicesConfig) {
		sc := ServiceConfig{AutoStart: autoStart, RestartOnCrash: restartOnCrash}
		switch key {
		case "arma":
			c.Arma = sc
		case "website":
			c.Website = sc
		case "bot":
			c.Bot = sc
		case "teamspeak":
			c.TeamSpeak = sc
		}
	})
	emit("services:changed")
	return err
}

// GetServiceLog returns the last lines of a service's log (Arma: its RPT).
func (a *App) GetServiceLog(key string, lines int) []string {
	if lines <= 0 || lines > 2000 {
		lines = 300
	}
	if key == "arma" {
		out, _ := tailRpt(lines)
		return out
	}
	s, err := serviceByKey(key)
	if err != nil {
		return nil
	}
	return tailFile(s.logPath(), lines)
}

// OpenServiceLogs opens the folder the service logs are written to.
func (a *App) OpenServiceLogs() error {
	dir, err := appDataDir()
	if err != nil {
		return err
	}
	return exec.Command("explorer", filepath.Join(dir, "logs")).Start()
}

// ---- service settings (website folder, TeamSpeak) ----

// ServiceSettings is what the Services tab's setup panels edit. The
// ServerQuery password is never sent to the UI.
type ServiceSettings struct {
	WebsiteDir         string `json:"websiteDir"`
	WebsiteDirDefault  string `json:"websiteDirDefault"`
	TS3Path            string `json:"ts3Path"`
	TS3LicenseAccepted bool   `json:"ts3LicenseAccepted"`
	TS3QueryPort       int    `json:"ts3QueryPort"`
	TS3VoicePort       int    `json:"ts3VoicePort"`
	TS3PrivilegeKey    string `json:"ts3PrivilegeKey"`
}

func (a *App) GetServiceSettings() (ServiceSettings, error) {
	c, err := loadServicesConfig()
	out := ServiceSettings{
		WebsiteDir: c.WebsiteDir, TS3Path: c.TS3Path, TS3LicenseAccepted: c.TS3LicenseAccepted,
		TS3QueryPort: c.TS3QueryPort, TS3VoicePort: c.TS3VoicePort, TS3PrivilegeKey: c.TS3PrivilegeKey,
	}
	if root, err := findRepoRoot(); err == nil {
		out.WebsiteDirDefault = filepath.Join(root, "src", "website")
	}
	return out, err
}

func (a *App) SaveServiceSettings(in ServiceSettings) error {
	if in.TS3QueryPort <= 0 || in.TS3QueryPort > 65535 || in.TS3VoicePort <= 0 || in.TS3VoicePort > 65535 {
		return fmt.Errorf("ports must be between 1 and 65535")
	}
	err := updateServicesConfig(func(c *ServicesConfig) {
		c.WebsiteDir, c.TS3Path, c.TS3LicenseAccepted = in.WebsiteDir, in.TS3Path, in.TS3LicenseAccepted
		c.TS3QueryPort, c.TS3VoicePort = in.TS3QueryPort, in.TS3VoicePort
	})
	emit("services:changed")
	return err
}

// CreateTS3AdminKey makes a new Server Admin privilege key (TeamSpeak must
// be running).
func (a *App) CreateTS3AdminKey() (string, error) {
	if !svcTeamSpeak.Running() {
		return "", fmt.Errorf("start TeamSpeak first")
	}
	return ts3NewAdminKey()
}

func (a *App) BrowseForFolder(title string) (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: title})
}

// ---- settings (unchanged) ----

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
