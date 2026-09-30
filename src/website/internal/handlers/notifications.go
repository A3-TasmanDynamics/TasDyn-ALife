package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"website/internal/auth"
	"website/internal/notify"
)

// Notifications (docs/GAMEPANEL_PARITY.md §7.3).

type notificationsData struct {
	Base
	Items  []notify.Item
	Unread int
}

func (d *Deps) Notifications(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	items, unread, err := notify.List(r.Context(), d.Pool, sess.PlayerID, 100)
	if err != nil {
		slog.Error("notifications: list failed", "error", err)
		http.Error(w, "Failed to load notifications.", http.StatusInternalServerError)
		return
	}
	d.Render.Render(w, "notifications.html", notificationsData{Base: baseFrom(r, "Notifications"), Items: items, Unread: unread})
}

// NotificationOpen marks one read and follows its link.
func (d *Deps) NotificationOpen(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	link, err := notify.Open(r.Context(), d.Pool, sess.PlayerID, id)
	if err != nil {
		slog.Error("notifications: open failed", "error", err)
	}
	if link == "" {
		link = "/notifications"
	}
	http.Redirect(w, r, link, http.StatusSeeOther)
}

// backTo returns a same-site path to return to after a POST.
func backTo(r *http.Request, fallback string) string {
	if b := notify.SafeLink(r.FormValue("back")); b != "" {
		return b
	}
	return fallback
}

func (d *Deps) NotificationsReadAll(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	if err := notify.MarkAllRead(r.Context(), d.Pool, sess.PlayerID); err != nil {
		slog.Error("notifications: mark read failed", "error", err)
	}
	http.Redirect(w, r, backTo(r, "/notifications"), http.StatusSeeOther)
}

func (d *Deps) NotificationAck(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := notify.Acknowledge(r.Context(), d.Pool, sess.PlayerID, id); err != nil {
		slog.Error("notifications: acknowledge failed", "error", err)
	}
	http.Redirect(w, r, backTo(r, "/notifications"), http.StatusSeeOther)
}

// ---- Admin: essential notices ----

type noticesData struct {
	Base
	AdminShell
	Notices []notify.Notice
	Form    map[string]string
}

func (d *Deps) AdminNotices(w http.ResponseWriter, r *http.Request) {
	data := noticesData{Base: baseFrom(r, "Essential notices"), AdminShell: d.adminShell(r, "notices"), Form: map[string]string{"audience": "staff"}}
	var err error
	if data.Notices, err = notify.Notices(r.Context(), d.Pool, 30); err != nil {
		slog.Error("notices: list failed", "error", err)
	}
	d.Render.Render(w, "notices.html", data)
}

func (d *Deps) AdminNoticePost(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	tx, err := d.Pool.Begin(r.Context())
	if err != nil {
		http.Error(w, "Database unavailable.", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())
	_, n, err := notify.Broadcast(r.Context(), tx, sess.PlayerID, r.FormValue("title"), r.FormValue("body"), r.FormValue("link"), r.FormValue("audience"))
	if err == nil {
		err = tx.Commit(r.Context())
	}
	switch {
	case err == nil:
		who := fmt.Sprintf("%d people", n)
		if n == 1 {
			who = "1 person"
		}
		http.Redirect(w, r, "/admin/notices?notice="+errMsg("Essential notice sent to "+who+". It shows as a banner until each of them acknowledges it."), http.StatusSeeOther)
	case errors.Is(err, notify.ErrNotAllowed):
		data := noticesData{Base: baseFrom(r, "Essential notices"), AdminShell: d.adminShell(r, "notices"),
			Form: map[string]string{"title": r.FormValue("title"), "body": r.FormValue("body"), "link": r.FormValue("link"), "audience": r.FormValue("audience")}}
		data.Notices, _ = notify.Notices(r.Context(), d.Pool, 30)
		msg := strings.TrimPrefix(err.Error(), notify.ErrNotAllowed.Error()+": ")
		data.Error = strings.ToUpper(msg[:1]) + msg[1:] + "."
		d.Render.RenderStatus(w, http.StatusUnprocessableEntity, "notices.html", data)
	default:
		slog.Error("notices: post failed", "error", err)
		http.Redirect(w, r, "/admin/notices?error="+errMsg("Something went wrong. Nothing was sent."), http.StatusSeeOther)
	}
}
