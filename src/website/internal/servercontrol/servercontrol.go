// Package servercontrol is the website's client for server_manager's
// control API (src/server_manager/controlapi.go), behind Admin → Server
// Control. The website never starts or kills the game process itself:
// every action is a request to server_manager on the game host.
package servercontrol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Client talks to one server_manager. A zero URL means not configured.
type Client struct {
	URL   string // e.g. http://127.0.0.1:8095
	Token string
	HTTP  *http.Client
}

// ErrNotConfigured is returned when SERVER_MANAGER_URL/TOKEN aren't set.
var ErrNotConfigured = errors.New("server_manager isn't connected")

// Status is server_manager's view of the game server.
type Status struct {
	Running       bool          `json:"running"`
	PID           int           `json:"pid"`
	StartedAt     string        `json:"startedAt"`
	UptimeSeconds int64         `json:"uptimeSeconds"`
	LastRestart   string        `json:"lastRestart"`
	RestartAt     string        `json:"restartAt"`
	CooldownUntil string        `json:"cooldownUntil"`
	ServerFPS     float64       `json:"serverFps"`
	Latency       time.Duration `json:"-"`
}

func (c *Client) Configured() bool { return c != nil && c.URL != "" && c.Token != "" }

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.URL, "/")+path, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("can't reach server_manager: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New("server_manager refused the token; check SERVER_MANAGER_TOKEN")
	}
	var generic struct {
		Error string `json:"error"`
	}
	if resp.StatusCode >= 300 {
		_ = json.NewDecoder(resp.Body).Decode(&generic)
		if generic.Error != "" {
			return errors.New(generic.Error)
		}
		return fmt.Errorf("server_manager answered HTTP %d", resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	var s Status
	start := time.Now()
	err := c.do(ctx, http.MethodGet, "/api/status", nil, &s)
	s.Latency = time.Since(start)
	return s, err
}

func (c *Client) Start(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/api/start", nil, nil)
}
func (c *Client) Stop(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/api/stop", nil, nil)
}

// Restart schedules a restart after delay and returns when it will happen.
func (c *Client) Restart(ctx context.Context, delay time.Duration) (time.Time, error) {
	var out struct {
		RestartAt string `json:"restartAt"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/restart", map[string]int{"delaySeconds": int(delay.Seconds())}, &out); err != nil {
		return time.Time{}, err
	}
	t, _ := time.Parse(time.RFC3339, out.RestartAt)
	return t, nil
}

func (c *Client) CancelRestart(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/api/restart/cancel", nil, nil)
}

// Logs returns the last n lines of the server's RPT log.
func (c *Client) Logs(ctx context.Context, n int) ([]string, error) {
	var out struct {
		Lines []string `json:"lines"`
	}
	err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/logs?lines=%d", n), nil, &out)
	return out.Lines, err
}

var ipv4 = regexp.MustCompile(`\b(\d{1,3})\.(\d{1,3})\.\d{1,3}\.\d{1,3}\b`)

// MaskIPs hides the last two parts of IPv4 addresses (for staff below Head
// Admin, per the layout plan).
func MaskIPs(line string) string { return ipv4.ReplaceAllString(line, "$1.$2.x.x") }
