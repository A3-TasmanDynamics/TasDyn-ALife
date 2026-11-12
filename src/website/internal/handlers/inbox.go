package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"website/internal/auth"
	"website/internal/inbox"
)

// ---- /admin/inbox ----

type inboxData struct {
	Base
	AdminShell
	Items    []inbox.Item
	Kinds    []inbox.Kind
	Counts   map[string]int
	Show     string // all / mine / unassigned
	Kind     string // "" or one kind
	Now      time.Time
	Total    int
	CanClaim bool // support access: tickets can be taken from here
}

// Age is how long it's been waiting.
func (d inboxData) Age(t time.Time) string {
	s := d.Now.Sub(t)
	switch {
	case s < time.Hour:
		return "<1 h"
	case s < 48*time.Hour:
		return strings.TrimSuffix(hoursMinutes(s.Truncate(time.Hour)), " 0m")
	}
	return strconv.Itoa(int(s.Hours()/24)) + " days"
}

// Inbox is everything waiting on staff; each kind is filtered by the
// viewer's permissions (internal/inbox).
func (d *Deps) Inbox(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	q := r.URL.Query()
	data := inboxData{Base: baseFrom(r, "Inbox"), AdminShell: d.adminShell(r, "inbox"), Kinds: inbox.Kinds,
		Show: q.Get("show"), Kind: q.Get("kind"), Now: time.Now(), Counts: map[string]int{}}
	if data.Show != "mine" && data.Show != "unassigned" {
		data.Show = "all"
	}
	v := inbox.Viewer{PlayerID: sess.PlayerID, Perms: data.Perms, Support: sess.SupportPanelAccess}
	data.CanClaim = sess.SupportPanelAccess
	all, err := inbox.Collect(r.Context(), d.Pool, v)
	if err != nil {
		slog.Error("inbox failed", "error", err)
		http.Error(w, "Failed to load the inbox.", http.StatusInternalServerError)
		return
	}
	data.Total = len(all)
	for _, it := range all {
		data.Counts[it.Kind]++
		if data.Kind != "" && it.Kind != data.Kind {
			continue
		}
		switch data.Show {
		case "mine":
			if !it.Mine {
				continue
			}
		case "unassigned":
			if it.Owner != "" {
				continue
			}
		}
		data.Items = append(data.Items, it)
	}
	d.Render.Render(w, "inbox.html", data)
}
