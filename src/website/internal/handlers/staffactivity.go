package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"website/internal/chart"
	"website/internal/staffreport"
)

// ---- /admin/staff-activity ----

type staffActivityData struct {
	Base
	AdminShell
	Window  string // 7d / 30d / 90d
	Windows []string
	Members []staffreport.Member
	Team    staffreport.Team
	Weeks   []staffreport.Week
	Chart   chart.Chart
	Now     time.Time
	Top     int // the busiest member's total, for the bars
}

// Dur is a handling time, roughly: "35 min", "6 h", "2.5 days".
func (d staffActivityData) Dur(t time.Duration) string {
	switch {
	case t <= 0:
		return "—"
	case t < time.Hour:
		return fmt.Sprintf("%d min", int(t.Minutes()))
	case t < 48*time.Hour:
		return fmt.Sprintf("%.0f h", t.Hours())
	}
	return fmt.Sprintf("%.1f days", t.Hours()/24)
}

// Ago is how long since t.
func (d staffActivityData) Ago(t *time.Time) string {
	if t == nil {
		return "nothing in this window"
	}
	if d.Now.Sub(*t) < time.Minute {
		return "just now"
	}
	return d.Dur(d.Now.Sub(*t)) + " ago"
}

// Bar is a member's share of the busiest member's work, in percent.
func (d staffActivityData) Bar(m staffreport.Member) int {
	if d.Top == 0 {
		return 0
	}
	return m.Total() * 100 / d.Top
}

var staffWindows = map[string]time.Duration{"7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour, "90d": 90 * 24 * time.Hour}

func (d *Deps) StaffActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := staffActivityData{Base: baseFrom(r, "Staff Activity"), AdminShell: d.adminShell(r, "staffactivity"),
		Window: r.URL.Query().Get("window"), Windows: []string{"7d", "30d", "90d"}, Now: time.Now()}
	if _, ok := staffWindows[data.Window]; !ok {
		data.Window = "30d"
	}
	since := data.Now.Add(-staffWindows[data.Window])
	var err error
	if data.Members, err = staffreport.Members(ctx, d.Pool, since); err != nil {
		slog.Error("staff activity failed", "error", err)
		http.Error(w, "Failed to load staff activity.", http.StatusInternalServerError)
		return
	}
	for _, m := range data.Members {
		data.Top = max(data.Top, m.Total())
	}
	if data.Team, err = staffreport.TeamStats(ctx, d.Pool, since); err != nil {
		slog.Error("staff activity: team stats failed", "error", err)
	}
	if data.Weeks, err = staffreport.Weekly(ctx, d.Pool, 12); err != nil {
		slog.Error("staff activity: weekly failed", "error", err)
	}
	if len(data.Weeks) > 1 {
		times := make([]time.Time, len(data.Weeks))
		var flags, replies, entries []float64
		for i, wk := range data.Weeks {
			times[i] = wk.Start.Local()
			flags = append(flags, float64(wk.FlagsReviewed))
			replies = append(replies, float64(wk.TicketReplies))
			entries = append(entries, float64(wk.CaseEntries))
		}
		data.Chart = chart.BuildCounts(times, []chart.Series{
			{Name: "Flag reviews", Class: "lc-s1", Values: flags},
			{Name: "Ticket replies", Class: "lc-s2", Values: replies},
			{Name: "Case entries", Class: "lc-s3", Values: entries},
		})
	}
	d.Render.Render(w, "staff_activity.html", data)
}
