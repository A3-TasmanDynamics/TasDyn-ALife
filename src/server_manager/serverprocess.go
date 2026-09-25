package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ServerStatus is what the frontend polls/displays for the launch tab.
type ServerStatus struct {
	Running   bool   `json:"running"`
	PID       int    `json:"pid"`
	StartedAt string `json:"startedAt"` // RFC3339, empty if not running
}

// serverProcessManager owns the running arma3server_x64.exe child process
// (if any) and the goroutine tailing its RPT log. One dedicated server at
// a time -- this app manages a single local install, not a fleet.
type serverProcessManager struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	startedAt  time.Time
	profileDir string
	stopTail   chan struct{}
}

var procMgr = &serverProcessManager{}

// LaunchServer writes server.cfg into the configured Arma 3 Server install
// and starts arma3server_x64.exe. Persists the current ServerConfig first,
// so what actually launches always matches what's saved.
func (a *App) LaunchServer() error {
	procMgr.mu.Lock()
	defer procMgr.mu.Unlock()

	if procMgr.cmd != nil {
		return fmt.Errorf("server already running (PID %d)", procMgr.cmd.Process.Pid)
	}

	settings, err := loadSettings()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}
	if settings.ArmaServerPath == "" {
		return fmt.Errorf("Arma 3 Server path not set -- configure it in Settings first")
	}

	exePath := filepath.Join(settings.ArmaServerPath, "arma3server_x64.exe")
	if _, err := os.Stat(exePath); err != nil {
		return fmt.Errorf("arma3server_x64.exe not found at %s", exePath)
	}

	cfg, err := loadServerConfig()
	if err != nil {
		return fmt.Errorf("load server config: %w", err)
	}

	cfgPath := filepath.Join(settings.ArmaServerPath, "server.cfg")
	if err := os.WriteFile(cfgPath, []byte(renderServerCfg(cfg)), 0o644); err != nil {
		return fmt.Errorf("write server.cfg: %w", err)
	}

	profileDir := filepath.Join(os.TempDir(), fmt.Sprintf("alife_server_profile_%d", time.Now().Unix()))
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return fmt.Errorf("create profile dir: %w", err)
	}

	cmd := exec.Command(exePath,
		"-config=server.cfg",
		fmt.Sprintf("-profiles=%s", profileDir),
		"-name=alife_server",
		"-port=2302",
	)
	cmd.Dir = settings.ArmaServerPath

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	procMgr.cmd = cmd
	procMgr.startedAt = time.Now()
	procMgr.profileDir = profileDir
	procMgr.stopTail = make(chan struct{})

	go procMgr.tailLog(a.ctx, profileDir, procMgr.stopTail)

	go func() {
		_ = cmd.Wait() // reap the process; also means Running flips to false if the server exits/crashes on its own
		procMgr.mu.Lock()
		procMgr.cmd = nil
		select {
		case <-procMgr.stopTail:
			// already closed by StopServer
		default:
			close(procMgr.stopTail)
		}
		procMgr.mu.Unlock()
		runtime.EventsEmit(a.ctx, "server:stopped")
	}()

	return nil
}

// StopServer terminates the running server, if any. Config files this app
// wrote (server.cfg) are left in place -- StopServer isn't a cleanup tool
// like tools/test_local_server.ps1, it's for a real, intentionally-running
// server, and touching config on every stop would be surprising.
func (a *App) StopServer() error {
	procMgr.mu.Lock()
	defer procMgr.mu.Unlock()

	if procMgr.cmd == nil {
		return fmt.Errorf("server is not running")
	}

	if err := procMgr.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("stop server: %w", err)
	}

	return nil
}

func (a *App) GetServerStatus() ServerStatus {
	procMgr.mu.Lock()
	defer procMgr.mu.Unlock()

	if procMgr.cmd == nil {
		return ServerStatus{Running: false}
	}

	return ServerStatus{
		Running:   true,
		PID:       procMgr.cmd.Process.Pid,
		StartedAt: procMgr.startedAt.Format(time.RFC3339),
	}
}

// tailLog streams new RPT log lines to the frontend as "server:log" events
// as they're written -- the live console. Waits for the RPT file to
// actually appear first (Arma creates it a few seconds after launch, not
// instantly).
func (m *serverProcessManager) tailLog(ctx context.Context, profileDir string, stop chan struct{}) {
	var rptPath string
	for i := 0; i < 60; i++ {
		select {
		case <-stop:
			return
		default:
		}
		path := findLatestRpt(profileDir)
		if path != "" {
			rptPath = path
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if rptPath == "" {
		return
	}

	f, err := os.Open(rptPath)
	if err != nil {
		return
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	for {
		select {
		case <-stop:
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if line != "" {
			runtime.EventsEmit(ctx, "server:log", line)
		}
		if err != nil {
			time.Sleep(300 * time.Millisecond)
		}
	}
}

func findLatestRpt(profileDir string) string {
	entries, err := os.ReadDir(profileDir)
	if err != nil {
		return ""
	}

	var rpts []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".rpt" {
			rpts = append(rpts, filepath.Join(profileDir, e.Name()))
		}
	}
	if len(rpts) == 0 {
		return ""
	}

	sort.Strings(rpts)
	return rpts[len(rpts)-1]
}
