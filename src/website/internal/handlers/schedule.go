package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"website/internal/auth"
	"website/internal/schedule"
)

// ---- /admin/schedule (announce.post) ----

type scheduleData struct {
	Base
	AdminShell
	Messages  []schedule.Message
	Now       time.Time
	Default   string // the "when" field's default (in 5 minutes)
	TZ        string
	Countdown []int
}

// Every is a repeat interval for display: "every 2 h", "every 30 min".
func (d scheduleData) Every(mins int) string {
	switch {
	case mins == 0:
		return "once"
	case mins%1440 == 0:
		return "every " + strconv.Itoa(mins/1440) + " day" + map[bool]string{true: "", false: "s"}[mins == 1440]
	case mins%60 == 0:
		return "every " + strconv.Itoa(mins/60) + " h"
	}
	return "every " + strconv.Itoa(mins) + " min"
}

func (d *Deps) Schedule(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	zone, _ := now.Zone()
	data := scheduleData{Base: baseFrom(r, "Scheduled Messages"), AdminShell: d.adminShell(r, "schedule"), Now: now,
		Default: now.Add(5 * time.Minute).Format("2006-01-02T15:04"), TZ: zone, Countdown: schedule.Countdown}
	var err error
	if data.Messages, err = schedule.List(r.Context(), d.Pool); err != nil {
		slog.Error("scheduled messages failed", "error", err)
		http.Error(w, "Failed to load scheduled messages.", http.StatusInternalServerError)
		return
	}
	d.Render.Render(w, "schedule.html", data)
}

// parseLocal reads a datetime-local field in the server's time zone.
func parseLocal(v string) (time.Time, bool) {
	t, err := time.ParseInLocation("2006-01-02T15:04", strings.TrimSpace(v), time.Local)
	return t, err == nil
}

func (d *Deps) ScheduleAction(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	ctx := r.Context()
	var err error
	msg := "Done."
	switch r.FormValue("op") {
	case "add":
		at, ok := parseLocal(r.FormValue("at"))
		if !ok {
			http.Redirect(w, r, "/admin/schedule?error="+errMsg("Pick when to send it."), http.StatusSeeOther)
			return
		}
		repeat, _ := strconv.Atoi(r.FormValue("repeat"))
		var until *time.Time
		if u, ok := parseLocal(r.FormValue("until")); ok {
			until = &u
		}
		_, err = schedule.Add(ctx, d.Pool, sess.PlayerID, r.FormValue("text"), at, repeat, until)
		msg = "Scheduled."
	case "countdown":
		at, ok := parseLocal(r.FormValue("at"))
		if !ok || !at.After(time.Now()) {
			http.Redirect(w, r, "/admin/schedule?error="+errMsg("Pick a restart time that hasn't passed."), http.StatusSeeOther)
			return
		}
		var n int
		if n, err = schedule.QueueCountdown(ctx, d.Pool, sess.PlayerID, at, r.FormValue("reason")); err == nil {
			msg = "Countdown set: " + strconv.Itoa(n) + " warning" + map[bool]string{true: "", false: "s"}[n == 1] + " before " + at.Format("15:04") + ". This only warns players: schedule the restart itself on Server Control."
		}
	case "pause", "resume":
		id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
		err = schedule.SetActive(ctx, d.Pool, id, r.FormValue("op") == "resume")
		msg = map[string]string{"pause": "Paused.", "resume": "Resumed."}[r.FormValue("op")]
	case "delete":
		id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
		err = schedule.Delete(ctx, d.Pool, sess.PlayerID, id)
		msg = "Deleted."
	default:
		http.Redirect(w, r, "/admin/schedule?error="+errMsg("Unknown action."), http.StatusSeeOther)
		return
	}
	switch {
	case err == nil:
		http.Redirect(w, r, "/admin/schedule?notice="+errMsg(msg), http.StatusSeeOther)
	case errors.Is(err, schedule.ErrNotAllowed):
		m := strings.TrimPrefix(err.Error(), schedule.ErrNotAllowed.Error()+": ")
		http.Redirect(w, r, "/admin/schedule?error="+errMsg(strings.ToUpper(m[:1])+m[1:]+"."), http.StatusSeeOther)
	default:
		slog.Error("schedule action failed", "error", err)
		http.Redirect(w, r, "/admin/schedule?error="+errMsg("Something went wrong. Nothing was changed."), http.StatusSeeOther)
	}
}
