package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"website/internal/auth"
	"website/internal/cases"
)

// ---- /admin/reports (cases.view) ----

type reportsData struct {
	Base
	AdminShell
	View       string // new / cases / closed / all
	Reports    []cases.Report
	CanLead    bool
	CanTickets bool // support panel access: the ticket itself can be opened
	Now        time.Time
}

// Waiting is how long a report has been waiting, roughly.
func (d reportsData) Waiting(t time.Time) string {
	since := d.Now.Sub(t)
	switch {
	case since < time.Hour:
		return "under an hour"
	case since < 48*time.Hour:
		return hoursMinutes(since.Truncate(time.Hour))
	}
	return strconv.Itoa(int(since.Hours()/24)) + " days"
}

func (d *Deps) Reports(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	data := reportsData{Base: baseFrom(r, "Reports"), AdminShell: d.adminShell(r, "reports"), View: r.URL.Query().Get("view"),
		CanLead: d.can(r, "cases.lead"), CanTickets: sess != nil && sess.SupportPanelAccess, Now: time.Now()}
	switch data.View {
	case "cases", "closed", "all":
	default:
		data.View = "new"
	}
	view := data.View
	if view == "all" {
		view = ""
	}
	var err error
	if data.Reports, err = cases.Reports(r.Context(), d.Pool, view, 200); err != nil {
		slog.Error("reports failed", "error", err)
		http.Error(w, "Failed to load reports.", http.StatusInternalServerError)
		return
	}
	d.Render.Render(w, "reports.html", data)
}
