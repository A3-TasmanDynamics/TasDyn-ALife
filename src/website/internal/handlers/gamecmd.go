package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"website/internal/auth"
	"website/internal/gamecmd"
)

// gameCaps is which in-game actions the viewer may send from the panel.
func (d *Deps) gameCaps(r *http.Request) map[string]bool {
	caps := map[string]bool{}
	for action, perm := range gamecmd.Perms {
		if d.can(r, perm) {
			caps[action] = true
		}
	}
	return caps
}

// GameCommand queues a message / warning / kick / announcement for the
// game server (POST /admin/game/command). The permission is the same as
// the in-game admin menu's for that action.
func (d *Deps) GameCommand(w http.ResponseWriter, r *http.Request) {
	back := safeBack(r, "/admin/online")
	action := r.FormValue("action")
	perm, ok := gamecmd.Perms[action]
	if !ok || !d.can(r, perm) {
		http.Redirect(w, r, withParam(back, "error", "You don't have permission to do that."), http.StatusSeeOther)
		return
	}
	sess, _ := auth.FromContext(r.Context())
	target, _ := strconv.ParseInt(r.FormValue("player"), 10, 64)
	_, err := gamecmd.Issue(r.Context(), d.Pool, sess.PlayerID, action, target, r.FormValue("text"))
	switch {
	case err == nil:
		msg := map[string]string{"message": "Message sent", "warn": "Warning sent", "kick": "Kick sent", "announce": "Announcement sent"}[action]
		http.Redirect(w, r, withParam(back, "notice", msg+". It reaches the game within a few seconds; see In-game tools on Online Now."), http.StatusSeeOther)
	case errors.Is(err, gamecmd.ErrNotAllowed):
		m := strings.TrimPrefix(err.Error(), gamecmd.ErrNotAllowed.Error()+": ")
		http.Redirect(w, r, withParam(back, "error", strings.ToUpper(m[:1])+m[1:]+"."), http.StatusSeeOther)
	default:
		slog.Error("game command failed", "action", action, "error", err)
		http.Redirect(w, r, withParam(back, "error", "Something went wrong. Nothing was sent."), http.StatusSeeOther)
	}
}
