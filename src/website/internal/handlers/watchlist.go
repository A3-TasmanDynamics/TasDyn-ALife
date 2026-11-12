package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"website/internal/audit"
	"website/internal/watchlist"
)

// ---- /admin/watchlist ----

type watchlistData struct {
	Base
	AdminShell
	Q       string
	Entries []watchlist.Entry
	Online  int
	Total   int
	CanEdit bool
	Now     time.Time
}

// Ago is how long ago, roughly: "3h 10m", "4 days".
func (d watchlistData) Ago(t *time.Time) string {
	if t == nil {
		return "never in game"
	}
	since := d.Now.Sub(*t)
	if since >= 48*time.Hour {
		return fmt.Sprintf("%d days ago", int(since.Hours()/24))
	}
	return hoursMinutes(since) + " ago"
}

func (d *Deps) Watchlist(w http.ResponseWriter, r *http.Request) {
	data := watchlistData{Base: baseFrom(r, "Watchlist"), AdminShell: d.adminShell(r, "watchlist"),
		Q: strings.TrimSpace(r.URL.Query().Get("q")), CanEdit: d.can(r, "players.notes"), Now: time.Now()}
	var err error
	if data.Entries, err = watchlist.List(r.Context(), d.Pool, data.Q); err != nil {
		slog.Error("watchlist failed", "error", err)
		http.Error(w, "Failed to load the watchlist.", http.StatusInternalServerError)
		return
	}
	for _, e := range data.Entries {
		if e.Online {
			data.Online++
		}
	}
	data.Total = len(data.Entries)
	if data.Q != "" {
		data.Total, _ = watchlist.Count(r.Context(), d.Pool)
	}
	d.Render.Render(w, "watchlist.html", data)
}

// WatchlistAdd adds a player by Steam ID, player number or exact name
// (players.notes).
func (d *Deps) WatchlistAdd(w http.ResponseWriter, r *http.Request) {
	back := "/admin/watchlist?"
	if !d.can(r, "players.notes") {
		http.Redirect(w, r, back+"error="+errMsg("You don't have permission to do that."), http.StatusSeeOther)
		return
	}
	id, err := watchlist.FindPlayer(r.Context(), d.Pool, r.FormValue("who"))
	if err == nil {
		err = watchlist.Add(r.Context(), d.Pool, watchlist.Actor{PlayerID: playerActor(r).PlayerID, Source: audit.SourceWebsite}, id, r.FormValue("reason"))
	}
	finishWatchAction(w, r, back, err, "Added to the watchlist.")
}

// WatchlistRemove takes a player off from the Watchlist page (players.notes).
func (d *Deps) WatchlistRemove(w http.ResponseWriter, r *http.Request) {
	back := "/admin/watchlist?"
	id, ok := profileID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !d.can(r, "players.notes") {
		http.Redirect(w, r, back+"error="+errMsg("You don't have permission to do that."), http.StatusSeeOther)
		return
	}
	err := watchlist.Remove(r.Context(), d.Pool, watchlist.Actor{PlayerID: playerActor(r).PlayerID, Source: audit.SourceWebsite}, id, r.FormValue("note"))
	finishWatchAction(w, r, back, err, "Taken off the watchlist.")
}
