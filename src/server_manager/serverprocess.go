package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ServerStatus is what the frontend polls/displays for the launch tab.
type ServerStatus struct {
	Running   bool   `json:"running"`
	PID       int    `json:"pid"`
	StartedAt string `json:"startedAt"` // RFC3339, empty if not running
}

// PerformanceSample is one point on the Performance tab's live graphs,
// pushed via the "server:performance" event -- no DB table backs this,
// deliberately: it's ephemeral process telemetry, not durable game state,
// so it doesn't belong in database/schema.sql alongside the things that
// actually need to survive a restart.
type PerformanceSample struct {
	Timestamp        string  `json:"timestamp"` // RFC3339
	CPUPercent       float64 `json:"cpuPercent"`
	MemoryMB         float64 `json:"memoryMB"`
	UptimeSeconds    int64   `json:"uptimeSeconds"`
	NetworkSentKBps  float64 `json:"networkSentKBps"`
	NetworkRecvKBps  float64 `json:"networkRecvKBps"`
	// ServerFPS is -1 until the first "[ALife][FPS]" line has been read
	// from the RPT log (initServer.sqf logs one every 2s, but the first
	// one lags a few seconds behind server startup) -- distinguishes "no
	// data yet" from a genuine 0 FPS, which would otherwise look identical
	// on the graph.
	ServerFPS float64 `json:"serverFPS"`
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

	// fpsMu guards lastFPS specifically -- a separate, lighter-weight lock
	// from mu (which guards process lifecycle) since tailLog updates this
	// on every RPT line and samplePerformance reads it every 2s; neither
	// needs to coordinate with LaunchServer/StopServer to do so.
	fpsMu   sync.Mutex
	lastFPS float64
}

var procMgr = &serverProcessManager{}

// fpsLogPattern matches the "[ALife][FPS] <number>" line initServer.sqf
// logs via diag_log every 2 seconds (see that file) -- Arma exposes
// diag_fps only to script running inside the sim, so the mission has to
// report it; this is where server_manager picks that report back up from
// the RPT log it's already tailing.
var fpsLogPattern = regexp.MustCompile(`\[ALife\]\[FPS\]\s+([0-9]+(?:\.[0-9]+)?)`)

// LaunchResult carries anything Launch succeeded despite -- currently just
// non-fatal deployExtension warnings (e.g. the C++ extension hasn't been
// built yet). Returned directly rather than pushed as "server:log" events:
// those get wiped by the frontend's clearConsole() call immediately after
// a successful launch (fresh console per run), which would silently lose
// warnings emitted before cmd.Start() in exactly that race.
type LaunchResult struct {
	Warnings []string `json:"warnings"`
}

// LaunchServer deploys the mission (and, best-effort, the C++ extension),
// writes server.cfg, and starts arma3server_x64.exe. Persists the current
// ServerConfig first, so what actually launches always matches what's saved.
func (a *App) LaunchServer() (LaunchResult, error) {
	procMgr.mu.Lock()
	defer procMgr.mu.Unlock()

	if procMgr.cmd != nil {
		return LaunchResult{}, fmt.Errorf("server already running (PID %d)", procMgr.cmd.Process.Pid)
	}

	settings, err := loadSettings()
	if err != nil {
		return LaunchResult{}, fmt.Errorf("load settings: %w", err)
	}
	if settings.ArmaServerPath == "" {
		return LaunchResult{}, fmt.Errorf("Arma 3 Server path not set -- configure it in Settings first")
	}

	exePath := filepath.Join(settings.ArmaServerPath, "arma3server_x64.exe")
	if _, err := os.Stat(exePath); err != nil {
		return LaunchResult{}, fmt.Errorf("arma3server_x64.exe not found at %s", exePath)
	}

	cfg, err := loadServerConfig()
	if err != nil {
		return LaunchResult{}, fmt.Errorf("load server config: %w", err)
	}

	// Deploy the mission + (best-effort) the C++ extension before writing
	// server.cfg -- previously server.cfg named a template that nothing
	// had ever copied into mpmissions, so Arma had nothing to load. See
	// deploy.go; this mirrors what tools/test_local_server.ps1 already did
	// for its own smoke test, now wired into the real launch path too.
	repoRoot, err := findRepoRoot()
	if err != nil {
		return LaunchResult{}, fmt.Errorf("locate mission source: %w", err)
	}
	if err := deployMission(repoRoot, settings.ArmaServerPath, cfg.MissionTemplate); err != nil {
		return LaunchResult{}, fmt.Errorf("deploy mission: %w", err)
	}
	result := LaunchResult{Warnings: deployExtension(repoRoot, settings.ArmaServerPath)}

	cfgPath := filepath.Join(settings.ArmaServerPath, "server.cfg")
	if err := os.WriteFile(cfgPath, []byte(renderServerCfg(cfg)), 0o644); err != nil {
		return LaunchResult{}, fmt.Errorf("write server.cfg: %w", err)
	}

	profileDir := filepath.Join(os.TempDir(), fmt.Sprintf("alife_server_profile_%d", time.Now().Unix()))
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return LaunchResult{}, fmt.Errorf("create profile dir: %w", err)
	}

	cmd := exec.Command(exePath,
		"-config=server.cfg",
		fmt.Sprintf("-profiles=%s", profileDir),
		"-name=alife_server",
		"-port=2302",
	)
	cmd.Dir = settings.ArmaServerPath

	if err := cmd.Start(); err != nil {
		return LaunchResult{}, fmt.Errorf("start server: %w", err)
	}

	procMgr.cmd = cmd
	procMgr.startedAt = time.Now()
	procMgr.profileDir = profileDir
	procMgr.stopTail = make(chan struct{})
	procMgr.fpsMu.Lock()
	procMgr.lastFPS = -1 // no "[ALife][FPS]" line read yet this run
	procMgr.fpsMu.Unlock()

	go procMgr.tailLog(a.ctx, profileDir, procMgr.stopTail)
	go procMgr.samplePerformance(a.ctx, cmd.Process.Pid, procMgr.startedAt, procMgr.stopTail)

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

	return result, nil
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
			if match := fpsLogPattern.FindStringSubmatch(line); match != nil {
				if fps, parseErr := strconv.ParseFloat(match[1], 64); parseErr == nil {
					m.fpsMu.Lock()
					m.lastFPS = fps
					m.fpsMu.Unlock()
				}
			}
		}
		if err != nil {
			time.Sleep(300 * time.Millisecond)
		}
	}
}

// samplePerformance polls CPU/memory/network every 2 seconds and pushes a
// PerformanceSample event -- same push-based pattern as tailLog's log
// lines, for the same reason: the Performance tab shouldn't have to poll a
// Go method on a timer just to stay current. FPS isn't sampled here at
// all -- it arrives via tailLog reading initServer.sqf's periodic
// diag_log line (see fpsLogPattern), since gopsutil has no way to ask
// Arma's own simulation for its frame rate; this loop just reads whatever
// tailLog last stored.
//
// CPUPercent() is gopsutil's own convention: percent of ONE core, so a
// process using two full cores reads ~200%, not capped at 100% -- this
// differs from Task Manager's "% of total system capacity" view. Shown
// as-is on the graph, not renormalized, since renormalizing needs the
// machine's core count for a conversion that isn't actually more honest,
// just differently scaled.
//
// Network is SYSTEM-WIDE (net.IOCounters(false), all interfaces summed),
// not specific to the arma3server_x64.exe process -- gopsutil (and Windows
// generally, short of ETW) has no reliable per-process network byte
// counter the way it does for CPU/memory. Labeled as such on the graph;
// on a host dedicated to running this server, system-wide is a reasonable
// proxy, but it's not literally "this process's" traffic and shouldn't be
// presented as such.
func (m *serverProcessManager) samplePerformance(ctx context.Context, pid int, startedAt time.Time, stop chan struct{}) {
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return
	}

	var prevNetSent, prevNetRecv uint64
	var prevNetAt time.Time
	if counters, err := net.IOCounters(false); err == nil && len(counters) > 0 {
		prevNetSent = counters[0].BytesSent
		prevNetRecv = counters[0].BytesRecv
		prevNetAt = time.Now()
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			sample := PerformanceSample{
				Timestamp:     time.Now().Format(time.RFC3339),
				UptimeSeconds: int64(time.Since(startedAt).Seconds()),
			}

			if cpuPct, err := proc.CPUPercent(); err == nil {
				sample.CPUPercent = cpuPct
			}
			if memInfo, err := proc.MemoryInfo(); err == nil && memInfo != nil {
				sample.MemoryMB = float64(memInfo.RSS) / (1024 * 1024)
			}

			if counters, err := net.IOCounters(false); err == nil && len(counters) > 0 && !prevNetAt.IsZero() {
				elapsed := time.Since(prevNetAt).Seconds()
				// >= guards against a wrapped/reset counter (an interface
				// reconnecting mid-sample, say) reading as a huge negative
				// delta -- uint64 subtraction doesn't panic on underflow,
				// it silently wraps to a huge number, which would spike
				// the graph. Treat a decrease as "no data this tick"
				// instead of a nonsensical spike.
				if elapsed > 0 && counters[0].BytesSent >= prevNetSent && counters[0].BytesRecv >= prevNetRecv {
					sample.NetworkSentKBps = float64(counters[0].BytesSent-prevNetSent) / 1024 / elapsed
					sample.NetworkRecvKBps = float64(counters[0].BytesRecv-prevNetRecv) / 1024 / elapsed
				}
				prevNetSent = counters[0].BytesSent
				prevNetRecv = counters[0].BytesRecv
				prevNetAt = time.Now()
			}

			m.fpsMu.Lock()
			sample.ServerFPS = m.lastFPS
			m.fpsMu.Unlock()

			runtime.EventsEmit(ctx, "server:performance", sample)
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
