package handlers

import (
	"log/slog"
	"net/http"
	"time"
)

type landingData struct {
	Base
	OnlineCount     int
	RegisteredCount int
	LatestPosts     []devlogSummary
	// ServerState is the game server's status-page state: "up", "down", or
	// "" when it isn't monitored (GAME_QUERY_ADDR unset) or unknown.
	ServerState string
}

// Landing renders the public landing page -- no login required. OnlineCount
// comes straight from player_sessions, the same table
// src/server_manager/logs.go already reads, so the public page and the
// operator's desktop dashboard never disagree from two separate code paths.
// Deliberately doesn't claim a "server online" status here -- a successful
// DB query says nothing about whether arma3server_x64.exe is actually up and
// accepting connections (a separate system entirely), so showing that
// without a real signal for it would just be lying to visitors.
func (d *Deps) Landing(w http.ResponseWriter, r *http.Request) {
	data := landingData{Base: baseFrom(r, "")}

	err := d.Pool.QueryRow(r.Context(), `
		SELECT count(*) FROM player_sessions WHERE disconnected_at IS NULL
	`).Scan(&data.OnlineCount)
	if err != nil {
		// A DB hiccup on a non-essential stat shouldn't take the whole
		// landing page down -- show 0 rather than a 500.
		data.OnlineCount = 0
	}

	if err := d.Pool.QueryRow(r.Context(), `SELECT count(*) FROM players`).Scan(&data.RegisteredCount); err != nil {
		data.RegisteredCount = 0
	}

	// Server status comes from the status page's real A2S query of the game
	// server, never from the database being reachable.
	if d.StatusMonitor != nil {
		if snap, err := d.StatusMonitor.Snapshot(r.Context(), time.UTC); err == nil {
			for _, c := range snap.Components {
				if c.Key == "game" && (c.State == "up" || c.State == "down") {
					data.ServerState = c.State
				}
			}
		}
	}

	rows, err := d.Pool.Query(r.Context(), `
		SELECT slug, title, body, published_at FROM devlog_posts
		WHERE published_at IS NOT NULL ORDER BY published_at DESC LIMIT 3
	`)
	if err != nil {
		// Same reasoning as the stat queries above -- a devlog teaser is
		// non-essential to the landing page; log it and show none rather
		// than failing the whole page.
		slog.Error("landing: devlog teaser query failed", "error", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var s devlogSummary
			var body string
			var publishedAt time.Time
			if err := rows.Scan(&s.Slug, &s.Title, &body, &publishedAt); err == nil {
				s.Snippet = snippetOf(body)
				s.PublishedAt = publishedAt.Format("2 January 2006")
				data.LatestPosts = append(data.LatestPosts, s)
			}
		}
	}

	d.Render.Render(w, "landing.html", data)
}
