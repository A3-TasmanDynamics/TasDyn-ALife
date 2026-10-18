package discord

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Health across processes. When the bot runs on its own (`-mode bot`), the
// website (`-mode web`) can't call Bot.Health directly, so the bot serves it
// on a loopback address (BOT_HEALTH_ADDR, 127.0.0.1:8096 by default) and the
// website's status page -- and the Server Manager -- read it from there.

type healthReport struct {
	Ready     bool  `json:"ready"`
	LatencyMS int64 `json:"latencyMs"`
}

// ServeHealth answers GET /health until ctx ends. Errors are logged, never
// fatal: the bot works without it.
func (b *Bot) ServeHealth(ctx context.Context, addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ready, latency := b.Health()
		w.Header().Set("Content-Type", "application/json")
		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(healthReport{Ready: ready, LatencyMS: latency.Milliseconds()})
	})
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	slog.Info("discord bot: health endpoint", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("discord bot: health endpoint failed", "addr", addr, "error", err)
	}
}

// RemoteHealth reads a bot process's health endpoint; the same shape as
// Bot.Health, so the status monitor can't tell the difference. Not running
// = not ready.
func RemoteHealth(addr string) func() (bool, time.Duration) {
	client := &http.Client{Timeout: 3 * time.Second}
	return func() (bool, time.Duration) {
		resp, err := client.Get("http://" + addr + "/health")
		if err != nil {
			return false, 0
		}
		defer resp.Body.Close()
		var h healthReport
		if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
			return false, 0
		}
		return h.Ready, time.Duration(h.LatencyMS) * time.Millisecond
	}
}
