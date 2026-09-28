package handlers

import (
	"log/slog"
	"net/http"
	"time"
	_ "time/tzdata" // Australia/Sydney must resolve on Windows hosts too

	"website/internal/status"
)

// statusLoc is the region the server (and its players) live in -- the
// status page's daily bars are bucketed on its calendar.
var statusLoc = func() *time.Location {
	loc, err := time.LoadLocation("Australia/Sydney")
	if err != nil {
		return time.UTC
	}
	return loc
}()

type statusData struct {
	Base
	status.Snapshot
	Days int
}

// Status is the public status page -- no login required. Everything on it
// comes from checks actually recorded by internal/status; see that
// package for what each component's check really tests.
func (d *Deps) Status(w http.ResponseWriter, r *http.Request) {
	data := statusData{Base: baseFrom(r, "Status"), Days: status.Days}
	if d.StatusMonitor != nil {
		snap, err := d.StatusMonitor.Snapshot(r.Context(), statusLoc)
		if err != nil {
			slog.Error("status page: snapshot failed", "error", err)
			// If we can't read our own check history the database is the
			// likely culprit -- say so honestly instead of a bare 500.
			snap = status.Snapshot{State: "down", Headline: "Status data unavailable"}
		}
		data.Snapshot = snap
	}
	d.Render.Render(w, "status.html", data)
}
