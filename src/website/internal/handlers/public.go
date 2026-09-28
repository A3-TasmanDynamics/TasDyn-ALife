package handlers

import (
	"net/http"
)

type landingData struct {
	Base
	OnlineCount     int
	RegisteredCount int
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

	d.Render.Render(w, "landing.html", data)
}
