package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// The service supervisor: the manager runs four services -- the Arma 3
// server, the website, the Discord bot (the website binary in `-mode bot`)
// and a TeamSpeak 3 server -- and treats them all the same way:
//
//   - Start / Stop / Restart from the Services tab (or the control API).
//   - Auto-start when the manager opens (per service, ServicesConfig).
//   - Restart on crash, with a crash-loop guard: maxCrashes crashes within
//     crashWindow and it stops trying until you start it yourself.
//   - Re-attach: the manager can close and leave services running. Each
//     running service's PID is saved to state.json; next time the manager
//     opens it picks them back up (checking the PID is still that program,
//     not a reused number) instead of starting duplicates.
//
// Website, bot and TeamSpeak run with no console window and their output
// going to logs/<service>.log under the manager's AppData folder, so the
// log survives a re-attach. Arma keeps its own RPT log (serverprocess.go).

const (
	maxCrashes  = 3
	crashWindow = 5 * time.Minute
)

// Service states, as shown on the Services tab.
const (
	StateStopped   = "stopped"
	StateStarting  = "starting"
	StateRunning   = "running"
	StateStopping  = "stopping"
	StateCrashed   = "crashed"    // exited on its own; restarting shortly
	StateCrashLoop = "crash-loop" // crashed too often; waiting for you
)

// ServiceConfig is one service's toggles.
type ServiceConfig struct {
	AutoStart      bool `json:"autoStart"`
	RestartOnCrash bool `json:"restartOnCrash"`
}

// ServicesConfig is services.json: per-service toggles plus the settings
// the website, bot and TeamSpeak services need. Kept apart from
// settings.json so the Settings form (which doesn't show these) can't wipe
// them.
type ServicesConfig struct {
	Arma      ServiceConfig `json:"arma"`
	Website   ServiceConfig `json:"website"`
	Bot       ServiceConfig `json:"bot"`
	TeamSpeak ServiceConfig `json:"teamspeak"`

	// WebsiteDir is src/website (its .env and web/ folder); "" = found from
	// the repo this manager runs from.
	WebsiteDir string `json:"websiteDir"`

	// TeamSpeak 3 server: the folder holding ts3server.exe (downloaded from
	// teamspeak.com -- the manager doesn't download it), whether you've
	// accepted its licence, and the ServerQuery login the manager uses.
	TS3Path            string `json:"ts3Path"`
	TS3LicenseAccepted bool   `json:"ts3LicenseAccepted"`
	TS3QueryPort       int    `json:"ts3QueryPort"`
	TS3VoicePort       int    `json:"ts3VoicePort"`
	TS3QueryPassword   string `json:"ts3QueryPassword"` // serveradmin; generated, set on every start
	TS3PrivilegeKey    string `json:"ts3PrivilegeKey"`  // first-run admin key, read from the log
}

func defaultServicesConfig() ServicesConfig {
	return ServicesConfig{
		Arma:         ServiceConfig{AutoStart: false, RestartOnCrash: true},
		Website:      ServiceConfig{AutoStart: true, RestartOnCrash: true},
		Bot:          ServiceConfig{AutoStart: true, RestartOnCrash: true},
		TeamSpeak:    ServiceConfig{AutoStart: true, RestartOnCrash: true},
		TS3QueryPort: 10011,
		TS3VoicePort: 9987,
	}
}

// appDataDir is %AppData%\TasDyn-ALife-ServerManager (services.json,
// state.json, logs, website builds). ALIFE_MANAGER_DIR overrides it, for
// tests.
func appDataDir() (string, error) {
	if d := os.Getenv("ALIFE_MANAGER_DIR"); d != "" {
		return d, os.MkdirAll(d, 0o755)
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "TasDyn-ALife-ServerManager")
	return appDir, os.MkdirAll(appDir, 0o755)
}

var servicesFileMu sync.Mutex

func loadServicesConfig() (ServicesConfig, error) {
	servicesFileMu.Lock()
	defer servicesFileMu.Unlock()
	c := defaultServicesConfig()
	dir, err := appDataDir()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "services.json"))
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(data, &c)
	if c.TS3QueryPort == 0 {
		c.TS3QueryPort = 10011
	}
	if c.TS3VoicePort == 0 {
		c.TS3VoicePort = 9987
	}
	return c, err
}

func saveServicesConfig(c ServicesConfig) error {
	servicesFileMu.Lock()
	defer servicesFileMu.Unlock()
	dir, err := appDataDir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "services.json"), data, 0o600)
}

// updateServicesConfig loads, changes and saves services.json in one go.
func updateServicesConfig(change func(*ServicesConfig)) error {
	c, err := loadServicesConfig()
	if err != nil {
		return err
	}
	change(&c)
	return saveServicesConfig(c)
}

// ---- running-service state (state.json), for re-attaching ----

type runRecord struct {
	PID       int       `json:"pid"`
	Exe       string    `json:"exe"` // full path, checked on re-attach
	StartedAt time.Time `json:"startedAt"`
	Extra     string    `json:"extra"` // per service (Arma: its profile folder)
}

var stateMu sync.Mutex

func statePath() (string, error) {
	dir, err := appDataDir()
	return filepath.Join(dir, "state.json"), err
}

func loadRunState() map[string]runRecord {
	stateMu.Lock()
	defer stateMu.Unlock()
	out := map[string]runRecord{}
	if p, err := statePath(); err == nil {
		if data, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(data, &out)
		}
	}
	return out
}

func setRunState(name string, rec *runRecord) {
	stateMu.Lock()
	defer stateMu.Unlock()
	p, err := statePath()
	if err != nil {
		return
	}
	all := map[string]runRecord{}
	if data, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(data, &all)
	}
	if rec == nil {
		delete(all, name)
	} else {
		all[name] = *rec
	}
	if data, err := json.MarshalIndent(all, "", "  "); err == nil {
		_ = os.WriteFile(p, data, 0o644)
	}
}

// ---- the supervisor ----

// launchSpec is what a service's start function hands the supervisor.
type launchSpec struct {
	Cmd      *exec.Cmd
	Warnings []string
	Extra    string // saved with the PID (runRecord.Extra)
	// LogToFile: send the process's output to the service's log file.
	// False for Arma, which writes its own RPT.
	LogToFile bool
	Hidden    bool // no console window
}

type service struct {
	Key   string // "arma", "website", "bot", "teamspeak"
	Title string

	// start builds the command (deploying / building first as needed).
	start func() (launchSpec, error)
	// cfg returns this service's toggles.
	cfg func(ServicesConfig) ServiceConfig
	// onStarted / onExited: per-service extras (Arma's log tail and
	// performance sampling). Optional.
	onStarted func(pid int, extra string, adopted bool)
	onExited  func()
	// exePath is where this service's program lives, so one started some
	// other way (or by a manager too old to record its PID) is still found
	// and re-attached instead of started twice. extraFor fills in
	// runRecord.Extra for such a find. Both optional.
	exePath  func() string
	extraFor func() string

	mu        sync.Mutex
	state     string
	proc      *os.Process
	pid       int
	exe       string
	startedAt time.Time
	stopping  bool
	crashes   []time.Time
	lastError string
	warnings  []string
	logFile   *os.File
	restartAt time.Time // pending crash restart
}

// ServiceStatus is one row of the Services tab.
type ServiceStatus struct {
	Key            string   `json:"key"`
	Title          string   `json:"title"`
	State          string   `json:"state"`
	PID            int      `json:"pid"`
	StartedAt      string   `json:"startedAt"`
	UptimeSeconds  int64    `json:"uptimeSeconds"`
	LastError      string   `json:"lastError"`
	Warnings       []string `json:"warnings"`
	AutoStart      bool     `json:"autoStart"`
	RestartOnCrash bool     `json:"restartOnCrash"`
	RecentCrashes  int      `json:"recentCrashes"`
	RestartAt      string   `json:"restartAt"`
}

// emit sends a frontend event, once the UI is up.
func emit(event string, data ...any) {
	if a := theApp; a != nil && a.ctx != nil {
		runtime.EventsEmit(a.ctx, event, data...)
	}
}

func (s *service) logPath() string {
	dir, err := appDataDir()
	if err != nil {
		return ""
	}
	_ = os.MkdirAll(filepath.Join(dir, "logs"), 0o755)
	return filepath.Join(dir, "logs", s.Key+".log")
}

func (s *service) Status() ServiceStatus {
	cfg, _ := loadServicesConfig()
	sc := s.cfg(cfg)
	s.mu.Lock()
	defer s.mu.Unlock()
	st := ServiceStatus{
		Key: s.Key, Title: s.Title, State: s.state, PID: s.pid, LastError: s.lastError,
		Warnings: s.warnings, AutoStart: sc.AutoStart, RestartOnCrash: sc.RestartOnCrash,
		RecentCrashes: s.recentCrashesLocked(),
	}
	if st.State == "" {
		st.State = StateStopped
	}
	if !s.startedAt.IsZero() && (s.state == StateRunning || s.state == StateStarting) {
		st.StartedAt = s.startedAt.Format(time.RFC3339)
		st.UptimeSeconds = int64(time.Since(s.startedAt).Seconds())
	}
	if !s.restartAt.IsZero() {
		st.RestartAt = s.restartAt.Format(time.RFC3339)
	}
	return st
}

func (s *service) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.proc != nil
}

func (s *service) recentCrashesLocked() int {
	n := 0
	for _, t := range s.crashes {
		if time.Since(t) < crashWindow {
			n++
		}
	}
	return n
}

func (s *service) setState(state string) {
	s.state = state
	emit("services:changed")
}

// Start launches the service (you asked it to: this also resets the
// crash-loop guard). Returns the start function's warnings.
func (s *service) Start() ([]string, error) { return s.startService(true) }

func (s *service) startService(manual bool) ([]string, error) {
	s.mu.Lock()
	if s.proc != nil || s.state == StateStarting {
		s.mu.Unlock()
		return nil, fmt.Errorf("%s is already running", s.Title)
	}
	if manual {
		s.crashes = nil
	}
	s.restartAt = time.Time{}
	s.lastError, s.warnings = "", nil
	s.setState(StateStarting)
	s.mu.Unlock()

	spec, err := s.start()
	if err == nil {
		err = s.launch(spec)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.lastError = err.Error()
		s.setState(StateStopped)
		return spec.Warnings, err
	}
	s.warnings = spec.Warnings
	return spec.Warnings, nil
}

func (s *service) launch(spec launchSpec) error {
	cmd := spec.Cmd
	if spec.Hidden {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	}
	var logFile *os.File
	if spec.LogToFile {
		path := s.logPath()
		_ = os.Remove(path + ".1")
		_ = os.Rename(path, path+".1") // keep the previous run's log
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("open log: %w", err)
		}
		fmt.Fprintf(f, "==== %s started by the Server Manager at %s ====\n", s.Title, time.Now().Format(time.RFC1123))
		cmd.Stdout, cmd.Stderr = f, f
		logFile = f
	}
	if err := cmd.Start(); err != nil {
		if logFile != nil {
			logFile.Close()
		}
		return fmt.Errorf("start %s: %w", s.Title, err)
	}
	exe, _ := filepath.Abs(cmd.Path)
	now := time.Now()
	s.mu.Lock()
	s.proc, s.pid, s.exe, s.startedAt, s.stopping, s.logFile = cmd.Process, cmd.Process.Pid, exe, now, false, logFile
	s.setState(StateRunning)
	s.mu.Unlock()
	setRunState(s.Key, &runRecord{PID: cmd.Process.Pid, Exe: exe, StartedAt: now, Extra: spec.Extra})
	if s.onStarted != nil {
		s.onStarted(cmd.Process.Pid, spec.Extra, false)
	}
	go s.wait(func() error { return cmd.Wait() })
	return nil
}

// adopt picks up a process left running by a previous manager session.
func (s *service) adopt(rec runRecord) bool {
	p, err := process.NewProcess(int32(rec.PID))
	if err != nil {
		return false
	}
	exe, err := p.Exe()
	if err != nil || !strings.EqualFold(filepath.Clean(exe), filepath.Clean(rec.Exe)) {
		return false // gone, or the PID now belongs to something else
	}
	proc, err := os.FindProcess(rec.PID)
	if err != nil {
		return false
	}
	s.mu.Lock()
	s.proc, s.pid, s.exe, s.startedAt, s.stopping = proc, rec.PID, rec.Exe, rec.StartedAt, false
	s.setState(StateRunning)
	s.mu.Unlock()
	log.Printf("services: re-attached to %s (PID %d)", s.Title, rec.PID)
	if s.onStarted != nil {
		s.onStarted(rec.PID, rec.Extra, true)
	}
	go s.wait(func() error { _, err := proc.Wait(); return err })
	return true
}

// findRunning re-attaches to this service's program if it's running
// without a state.json record (matched by full path).
func (s *service) findRunning() bool {
	if s.exePath == nil {
		return false
	}
	want := s.exePath()
	if want == "" {
		return false
	}
	procs, err := process.Processes()
	if err != nil {
		return false
	}
	for _, p := range procs {
		exe, err := p.Exe()
		if err != nil || !strings.EqualFold(filepath.Clean(exe), filepath.Clean(want)) {
			continue
		}
		rec := runRecord{PID: int(p.Pid), Exe: exe, StartedAt: time.Now()}
		if ms, err := p.CreateTime(); err == nil {
			rec.StartedAt = time.UnixMilli(ms)
		}
		if s.extraFor != nil {
			rec.Extra = s.extraFor()
		}
		if s.adopt(rec) {
			setRunState(s.Key, &rec)
			return true
		}
	}
	return false
}

// wait runs until the process exits, then decides: stopped on purpose, or
// crashed (restart, or give up after too many).
func (s *service) wait(waitFn func() error) {
	err := waitFn()
	if s.onExited != nil {
		s.onExited()
	}
	setRunState(s.Key, nil)

	s.mu.Lock()
	if s.logFile != nil {
		s.logFile.Close()
		s.logFile = nil
	}
	intentional := s.stopping
	s.proc, s.pid, s.stopping = nil, 0, false
	if intentional {
		s.setState(StateStopped)
		s.mu.Unlock()
		return
	}
	s.crashes = append(s.crashes, time.Now())
	msg := "exited unexpectedly"
	if err != nil {
		msg = "exited unexpectedly: " + err.Error()
	}
	s.lastError = msg
	cfg, _ := loadServicesConfig()
	if !s.cfg(cfg).RestartOnCrash {
		s.setState(StateStopped)
		s.mu.Unlock()
		log.Printf("services: %s %s", s.Title, msg)
		return
	}
	n := s.recentCrashesLocked()
	if n >= maxCrashes {
		s.lastError = fmt.Sprintf("%s -- crashed %d times in %d minutes, not restarting until you start it", msg, n, int(crashWindow.Minutes()))
		s.setState(StateCrashLoop)
		s.mu.Unlock()
		log.Printf("services: %s crash loop", s.Title)
		return
	}
	delay := time.Duration(5<<uint(n-1)) * time.Second // 5s, 10s, ...
	s.restartAt = time.Now().Add(delay)
	s.setState(StateCrashed)
	s.mu.Unlock()
	log.Printf("services: %s %s; restarting in %s", s.Title, msg, delay)

	time.Sleep(delay)
	s.mu.Lock()
	still := s.state == StateCrashed && s.proc == nil
	s.restartAt = time.Time{}
	s.mu.Unlock()
	if still {
		if _, err := s.startService(false); err != nil {
			log.Printf("services: restarting %s failed: %v", s.Title, err)
		}
	}
}

// Stop ends the service and waits (up to 30s) for it to exit.
func (s *service) Stop() error {
	s.mu.Lock()
	if s.state == StateCrashed || s.state == StateCrashLoop {
		s.restartAt = time.Time{}
		s.setState(StateStopped) // cancels a pending crash restart
	}
	if s.proc == nil {
		s.mu.Unlock()
		return fmt.Errorf("%s is not running", s.Title)
	}
	s.stopping = true
	proc := s.proc
	s.setState(StateStopping)
	s.mu.Unlock()

	if err := proc.Kill(); err != nil && s.Running() {
		return fmt.Errorf("stop %s: %w", s.Title, err)
	}
	for i := 0; i < 120 && s.Running(); i++ {
		time.Sleep(250 * time.Millisecond)
	}
	if s.Running() {
		return fmt.Errorf("%s didn't stop within 30 seconds", s.Title)
	}
	return nil
}

func (s *service) Restart() ([]string, error) {
	if s.Running() {
		if err := s.Stop(); err != nil {
			return nil, err
		}
	}
	return s.Start()
}

// tailFile returns the last n lines of a text file.
func tailFile(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	const window = 256 << 10
	if fi, err := f.Stat(); err == nil && fi.Size() > window {
		_, _ = f.Seek(-window, io.SeekEnd)
	}
	data, _ := io.ReadAll(f)
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	var out []string
	for _, l := range lines {
		if l = strings.TrimRight(l, " \r"); l != "" {
			out = append(out, l)
		}
	}
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return out
}

var errUnknownService = errors.New("unknown service")
