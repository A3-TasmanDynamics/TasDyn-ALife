package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"website/internal/chart"
	"website/internal/online"
	"website/internal/watchlist"
)

// ---- /admin/online ----

type onlineData struct {
	Base
	AdminShell
	Now     time.Time
	Players []online.Player
	Left    []online.Left
	Sum     online.Summary
	Chart   chart.Chart
	Watched map[int64]string // player id -> why they're on the watchlist
}

func (d onlineData) Dur(t time.Duration) string { return hoursMinutes(t) }

// Since is how long a player has been on.
func (d onlineData) Since(p online.Player) string { return hoursMinutes(p.For(d.Now)) }

// Ago is how long ago something happened.
func (d onlineData) Ago(t time.Time) string { return hoursMinutes(d.Now.Sub(t)) }

// WatchReason is why the player is on the watchlist ("" if they aren't).
func (d onlineData) WatchReason(id int64) string { return d.Watched[id] }

// FactionName is the faction for display.
func (d onlineData) FactionName(f string) string {
	switch f {
	case "civ":
		return "Civilian"
	case "cop":
		return "Police"
	case "medic":
		return "EMS"
	}
	return "Unknown"
}

func (d *Deps) OnlineNow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := onlineData{Base: baseFrom(r, "Online Now"), AdminShell: d.adminShell(r, "online"), Now: time.Now()}
	var err error
	if data.Players, err = online.Now(ctx, d.Pool); err != nil {
		slog.Error("online now failed", "error", err)
		http.Error(w, "Failed to load who's online.", http.StatusInternalServerError)
		return
	}
	if data.Watched, err = watchlist.Reasons(ctx, d.Pool); err != nil {
		slog.Error("online: watchlist failed", "error", err)
	}
	if data.Left, err = online.RecentlyLeft(ctx, d.Pool, 2*time.Hour, 12); err != nil {
		slog.Error("online: recently left failed", "error", err)
	}
	times, counts, err := online.History(ctx, d.Pool, 24*time.Hour, 15*time.Minute)
	if err != nil {
		slog.Error("online: history failed", "error", err)
	}
	if data.Sum, err = online.Summarize(ctx, d.Pool, data.Players, times, counts); err != nil {
		slog.Error("online: summary failed", "error", err)
	}
	local := make([]time.Time, len(times))
	for i, t := range times {
		local[i] = t.Local()
	}
	data.Chart = chart.BuildCounts(local, []chart.Series{{Name: "Online", Class: "lc-online", Values: counts}})
	d.Render.Render(w, "online.html", data)
}
