package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The control API lets the website's Server Control page (Admin → Server
// Control) run this app's own start/stop/restart and read the live log, so
// the website never starts or kills the game process itself. It listens on
// ControlAPIAddr (127.0.0.1:8095 by default, so only the same machine can
// reach it) and every request needs "Authorization: Bearer <ControlAPIToken>".
// The token is generated on first run and stored in settings.json; copy it
// into the website's SERVER_MANAGER_TOKEN.

// RestartCooldown: at most one restart every five minutes.
const RestartCooldown = 5 * time.Minute

type controlState struct {
	mu            sync.Mutex
	lastRestart   time.Time
	pendingAt     time.Time // scheduled restart, zero = none
	pendingCancel chan struct{}
}

var control = &controlState{}

// ControlStatus is what GET /api/status returns.
type ControlStatus struct {
	Running       bool    `json:"running"`
	PID           int     `json:"pid"`
	StartedAt     string  `json:"startedAt"`
	UptimeSeconds int64   `json:"uptimeSeconds"`
	LastRestart   string  `json:"lastRestart"`
	RestartAt     string  `json:"restartAt"`     // scheduled restart, "" = none
	CooldownUntil string  `json:"cooldownUntil"` // "" = can restart now
	ServerFPS     float64 `json:"serverFps"`
}

func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ensureControlSettings fills in the control API defaults (and a token) the
// first time, and saves them.
func ensureControlSettings() (Settings, error) {
	s, err := loadSettings()
	if err != nil {
		return s, err
	}
	changed := false
	if s.ControlAPIAddr == "" {
		s.ControlAPIAddr, changed = "127.0.0.1:8095", true
	}
	if s.ControlAPIToken == "" {
		s.ControlAPIToken, changed = newToken(), true
	}
	if changed {
		err = saveSettingsToDisk(s)
	}
	return s, err
}

// startControlAPI runs the API until ctx ends. Errors are logged, never
// fatal: the desktop app works without it.
func (a *App) startControlAPI(ctx context.Context) {
	s, err := ensureControlSettings()
	if err != nil {
		log.Printf("control api: settings: %v", err)
		return
	}
	if s.ControlAPIDisabled {
		return
	}
	mux := a.controlMux(func() string {
		cur, _ := loadSettings()
		return cur.ControlAPIToken
	})

	srv := &http.Server{Addr: s.ControlAPIAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	log.Printf("control api: listening on %s", s.ControlAPIAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("control api: %v", err)
	}
}

// controlMux builds the API's routes; token returns the current token
// (read per request, so a changed token applies without a restart).
func (a *App) controlMux(token func() string) *http.ServeMux {
	mux := http.NewServeMux()
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			want := token()
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if want == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			h(w, r)
		}
	}
	mux.HandleFunc("GET /api/status", auth(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(a.controlStatus())
	}))
	mux.HandleFunc("POST /api/start", auth(func(w http.ResponseWriter, r *http.Request) {
		res, err := a.LaunchServer()
		reply(w, err, map[string]any{"warnings": res.Warnings})
	}))
	mux.HandleFunc("POST /api/stop", auth(func(w http.ResponseWriter, r *http.Request) {
		a.cancelPendingRestart()
		reply(w, a.StopServer(), nil)
	}))
	mux.HandleFunc("POST /api/restart", auth(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			DelaySeconds int `json:"delaySeconds"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&body)
		at, err := a.scheduleRestart(time.Duration(body.DelaySeconds) * time.Second)
		reply(w, err, map[string]any{"restartAt": at.Format(time.RFC3339)})
	}))
	mux.HandleFunc("POST /api/restart/cancel", auth(func(w http.ResponseWriter, r *http.Request) {
		if !a.cancelPendingRestart() {
			reply(w, errors.New("no restart is scheduled"), nil)
			return
		}
		reply(w, nil, nil)
	}))
	mux.HandleFunc("GET /api/logs", auth(func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.URL.Query().Get("lines"))
		if n <= 0 || n > 1000 {
			n = 200
		}
		lines, file := tailRpt(n)
		_ = json.NewEncoder(w).Encode(map[string]any{"lines": lines, "file": file})
	}))
	return mux
}

func reply(w http.ResponseWriter, err error, extra map[string]any) {
	out := map[string]any{"ok": err == nil}
	if err != nil {
		w.WriteHeader(http.StatusConflict)
		out["error"] = err.Error()
	}
	for k, v := range extra {
		out[k] = v
	}
	_ = json.NewEncoder(w).Encode(out)
}

func (a *App) controlStatus() ControlStatus {
	st := a.GetServerStatus()
	cs := ControlStatus{Running: st.Running, PID: st.PID, StartedAt: st.StartedAt, ServerFPS: -1}
	if t, err := time.Parse(time.RFC3339, st.StartedAt); err == nil {
		cs.UptimeSeconds = int64(time.Since(t).Seconds())
	}
	procMgr.fpsMu.Lock()
	if st.Running {
		cs.ServerFPS = procMgr.lastFPS
	}
	procMgr.fpsMu.Unlock()
	control.mu.Lock()
	defer control.mu.Unlock()
	if !control.lastRestart.IsZero() {
		cs.LastRestart = control.lastRestart.Format(time.RFC3339)
		if until := control.lastRestart.Add(RestartCooldown); time.Now().Before(until) {
			cs.CooldownUntil = until.Format(time.RFC3339)
		}
	}
	if !control.pendingAt.IsZero() {
		cs.RestartAt = control.pendingAt.Format(time.RFC3339)
	}
	return cs
}

// scheduleRestart stops and relaunches the server after delay (0 = now),
// at most once per RestartCooldown.
func (a *App) scheduleRestart(delay time.Duration) (time.Time, error) {
	if delay < 0 || delay > 30*time.Minute {
		return time.Time{}, errors.New("the delay must be between 0 and 30 minutes")
	}
	if !a.GetServerStatus().Running {
		return time.Time{}, errors.New("the server isn't running; start it instead")
	}
	control.mu.Lock()
	defer control.mu.Unlock()
	if !control.pendingAt.IsZero() {
		return control.pendingAt, fmt.Errorf("a restart is already scheduled for %s", control.pendingAt.Format("15:04:05"))
	}
	if until := control.lastRestart.Add(RestartCooldown); time.Now().Before(until) {
		return time.Time{}, fmt.Errorf("restarted recently; next restart allowed at %s", until.Format("15:04:05"))
	}
	at := time.Now().Add(delay)
	control.pendingAt = at
	cancel := make(chan struct{})
	control.pendingCancel = cancel
	go func() {
		select {
		case <-cancel:
			return
		case <-time.After(time.Until(at)):
		}
		control.mu.Lock()
		control.pendingAt, control.pendingCancel = time.Time{}, nil
		control.lastRestart = time.Now()
		control.mu.Unlock()
		if err := a.restartNow(); err != nil {
			log.Printf("control api: restart failed: %v", err)
		}
	}()
	return at, nil
}

func (a *App) cancelPendingRestart() bool {
	control.mu.Lock()
	defer control.mu.Unlock()
	if control.pendingCancel == nil {
		return false
	}
	close(control.pendingCancel)
	control.pendingAt, control.pendingCancel = time.Time{}, nil
	return true
}

// restartNow stops the server, waits for the process to exit, and launches it again.
func (a *App) restartNow() error {
	if err := a.StopServer(); err != nil && a.GetServerStatus().Running {
		return err
	}
	for i := 0; i < 120 && a.GetServerStatus().Running; i++ {
		time.Sleep(250 * time.Millisecond)
	}
	if a.GetServerStatus().Running {
		return errors.New("the server didn't stop within 30 seconds")
	}
	_, err := a.LaunchServer()
	return err
}

// tailRpt returns the last n lines of the current (or most recent) RPT log.
func tailRpt(n int) ([]string, string) {
	procMgr.mu.Lock()
	dir := procMgr.profileDir
	procMgr.mu.Unlock()
	if dir == "" {
		return nil, ""
	}
	path := findLatestRpt(dir)
	if path == "" {
		return nil, ""
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, path
	}
	defer f.Close()
	const window = 512 << 10
	if fi, err := f.Stat(); err == nil && fi.Size() > window {
		_, _ = f.Seek(-window, io.SeekEnd)
	}
	data, _ := io.ReadAll(f)
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	var out []string
	for _, l := range lines {
		if l = strings.TrimRight(l, " "); l != "" && !fpsLogPattern.MatchString(l) {
			out = append(out, l)
		}
	}
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return out, path
}
