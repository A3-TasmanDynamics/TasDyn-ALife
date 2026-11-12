package handlers

import (
	"net/http"
	"time"

	"website/internal/auth"
	"website/internal/live"
)

// Live is /admin/live: a Server-Sent Events stream of panel updates
// (badge counts, new flags / tickets / cases, watched players joining) for
// staff with admin or support panel access. Only what the viewer may see
// is sent. /static/live.js listens.
func (d *Deps) Live(w http.ResponseWriter, r *http.Request) {
	sess, ok := auth.FromContext(r.Context())
	if !ok || sess == nil || !(sess.AdminPanelAccess || sess.SupportPanelAccess) || d.LiveHub == nil {
		http.Error(w, "Not allowed.", http.StatusForbidden)
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming isn't supported.", http.StatusInternalServerError)
		return
	}
	perms, _ := auth.Effective(r.Context(), d.Pool, sess.PlayerID)
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("retry: 5000\n\n"))
	fl.Flush()

	msgs, stop := d.LiveHub.Subscribe()
	defer stop()
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	// Re-check access now and then: a session that loses the panel (or
	// expires) stops getting updates.
	recheck := time.NewTicker(5 * time.Minute)
	defer recheck.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			fl.Flush()
		case <-recheck.C:
			if p, err := auth.Effective(r.Context(), d.Pool, sess.PlayerID); err == nil {
				perms = p
			}
		case m, ok := <-msgs:
			if !ok {
				return
			}
			if m.Topic == "counts" {
				m = countsFor(m, perms, sess.AdminPanelAccess, sess.SupportPanelAccess)
			} else if !live.Allowed(m, perms, sess.AdminPanelAccess, sess.SupportPanelAccess) {
				continue
			}
			if _, err := w.Write(live.Encode(m)); err != nil {
				return
			}
			fl.Flush()
		}
	}
}

// countsFor keeps only the badges the viewer's sidebar shows.
func countsFor(m live.Msg, perms map[string]bool, admin, support bool) live.Msg {
	c, _ := m.Data.(live.Counts)
	out := map[string]int{}
	if admin && perms["cases.view"] {
		out["cases"], out["bans"] = c.OpenCases, c.ActiveBans
	}
	if admin && perms["anticheat.review"] {
		out["anticheat"] = c.OpenFlags
	}
	if admin && perms["applications.view"] {
		out["applications"] = c.OpenApps
	}
	if support {
		out["tickets_open"], out["tickets_unassigned"] = c.OpenTickets, c.UnassignedTickets
	}
	if admin && perms["players.view"] {
		out["online"] = c.Online
	}
	return live.Msg{Topic: "counts", Data: out}
}
