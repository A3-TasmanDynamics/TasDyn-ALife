package handlers

import (
	"log/slog"
	"net/http"

	"website/internal/dashboard"
)

type leaderboardData struct {
	Base
	LB dashboard.Leaderboard
}

// Leaderboards is the public /leaderboards page: opted-in players only,
// wealth in bands, plus gangs.
func (d *Deps) Leaderboards(w http.ResponseWriter, r *http.Request) {
	data := leaderboardData{Base: baseFrom(r, "Leaderboards")}
	var you int64
	if data.Session != nil {
		you = data.Session.PlayerID
	}
	lb, err := dashboard.PublicLeaderboard(r.Context(), d.Pool, you, 10)
	if err != nil {
		slog.Error("leaderboards: load failed", "error", err)
		http.Error(w, "Failed to load the leaderboards.", http.StatusInternalServerError)
		return
	}
	data.LB = lb
	d.Render.Render(w, "leaderboards.html", data)
}
