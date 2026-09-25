package handlers

import (
	"net/http"
)

type landingData struct {
	Base
	OnlineCount int
}

// Landing renders the public landing page -- no login required. OnlineCount
// comes straight from player_sessions, the same table
// src/server_manager/dashboard.go already reads, so the public page and the
// operator's desktop dashboard never disagree from two separate code paths.
func (d *Deps) Landing(w http.ResponseWriter, r *http.Request) {
	data := landingData{Base: baseFrom(r, "")}

	err := d.Pool.QueryRow(r.Context(), `
		SELECT count(*) FROM player_sessions WHERE disconnected_at IS NULL
	`).Scan(&data.OnlineCount)
	if err != nil {
		// A DB hiccup on the online-count query shouldn't take the whole
		// landing page down -- show 0 rather than a 500 for a
		// non-essential stat.
		data.OnlineCount = 0
	}

	d.Render.Render(w, "landing.html", data)
}
