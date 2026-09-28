// Package status runs the public status page's health checks. A background
// loop probes each component once a minute and records the result in
// status_checks; the page is then built only from what was actually
// recorded -- days before monitoring began show as "no data", never as
// invented uptime.
package status

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Interval is how often every component is probed.
const Interval = time.Minute

// retention is how long raw checks are kept. The page shows 60 days.
const retention = 90 * 24 * time.Hour

type probe func(ctx context.Context) (ok bool, latency time.Duration, detail string)

// Component is one monitored thing, in page order.
type Component struct {
	Key         string
	Name        string
	Description string
	probe       probe
	// selfCheck marks the website's own check: if the website is down, no
	// row gets written at all, so its uptime counts missing minutes as
	// down instead of ignoring them.
	selfCheck bool
}

type Monitor struct {
	Pool       *pgxpool.Pool
	Components []Component
	// Unmonitored lists components that exist but have no probe
	// configured, so the page can say so rather than silently omit them.
	Unmonitored []Component
}

type Options struct {
	// ListenAddr is the website's own listen address (":8080"), probed over
	// loopback.
	ListenAddr string
	// GameQueryAddr is the Arma server's Steam query endpoint
	// ("host:2303"). Empty = game server not monitored.
	GameQueryAddr string
	// DiscordHealth reports whether the bot's gateway session is up. Nil =
	// bot not running, so not monitored.
	DiscordHealth func() (bool, time.Duration)
}

func New(pool *pgxpool.Pool, o Options) *Monitor {
	m := &Monitor{Pool: pool}

	game := Component{Key: "game", Name: "Game Server", Description: "Arma 3 · Oceania / AU"}
	if o.GameQueryAddr != "" {
		addr := o.GameQueryAddr
		game.probe = func(ctx context.Context) (bool, time.Duration, string) {
			start := time.Now()
			info, err := queryA2SInfo(ctx, addr)
			if err != nil {
				return false, 0, "No response to server query"
			}
			return true, time.Since(start), fmt.Sprintf("%d / %d players", info.Players, info.MaxPlayers)
		}
		m.Components = append(m.Components, game)
	} else {
		m.Unmonitored = append(m.Unmonitored, game)
	}

	selfURL := "http://127.0.0.1" + o.ListenAddr + "/"
	if !strings.HasPrefix(o.ListenAddr, ":") {
		selfURL = "http://" + o.ListenAddr + "/"
	}
	client := &http.Client{Timeout: 10 * time.Second}
	m.Components = append(m.Components, Component{
		Key: "website", Name: "Website", Description: "Landing page, dashboard, support", selfCheck: true,
		probe: func(ctx context.Context) (bool, time.Duration, string) {
			start := time.Now()
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, selfURL, nil)
			resp, err := client.Do(req)
			if err != nil {
				return false, 0, "Not responding"
			}
			resp.Body.Close()
			if resp.StatusCode >= 500 {
				return false, time.Since(start), fmt.Sprintf("HTTP %d", resp.StatusCode)
			}
			return true, time.Since(start), ""
		},
	})

	m.Components = append(m.Components, Component{
		Key: "database", Name: "Database", Description: "Player data, economy, accounts",
		probe: func(ctx context.Context) (bool, time.Duration, string) {
			start := time.Now()
			if err := pool.Ping(ctx); err != nil {
				return false, 0, "Unreachable"
			}
			return true, time.Since(start), ""
		},
	})

	discord := Component{Key: "discord", Name: "Discord Bot", Description: "Account linking (/link)"}
	if o.DiscordHealth != nil {
		health := o.DiscordHealth
		discord.probe = func(ctx context.Context) (bool, time.Duration, string) {
			ok, latency := health()
			if !ok {
				return false, 0, "Disconnected from Discord"
			}
			return true, latency, ""
		}
		m.Components = append(m.Components, discord)
	} else {
		m.Unmonitored = append(m.Unmonitored, discord)
	}

	return m
}

// Run probes every component each Interval until ctx is cancelled. Meant to
// be started once, in its own goroutine.
func (m *Monitor) Run(ctx context.Context) {
	ticker := time.NewTicker(Interval)
	defer ticker.Stop()
	lastPrune := time.Time{}
	for {
		m.checkAll(ctx)
		if time.Since(lastPrune) > time.Hour {
			if _, err := m.Pool.Exec(ctx, `DELETE FROM status_checks WHERE checked_at < now() - $1::interval`, fmt.Sprintf("%d seconds", int(retention.Seconds()))); err != nil {
				slog.Error("status: prune failed", "error", err)
			}
			lastPrune = time.Now()
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *Monitor) checkAll(ctx context.Context) {
	for _, c := range m.Components {
		pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		ok, latency, detail := c.probe(pctx)
		cancel()
		if ctx.Err() != nil {
			return
		}
		_, err := m.Pool.Exec(ctx, `
			INSERT INTO status_checks (component, ok, latency_ms, detail) VALUES ($1, $2, $3, NULLIF($4, ''))
		`, c.Key, ok, int(latency.Milliseconds()), detail)
		if err != nil {
			// The database itself being down lands here too -- nothing to
			// record it in, which the page shows as a gap.
			slog.Error("status: recording check failed", "component", c.Key, "error", err)
		}
	}
}
