package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"website/internal/auth"
	"website/internal/replies"
)

// ---- /admin/replies ----

type repliesData struct {
	Base
	AdminShell
	Scopes       []replies.Scope
	Replies      []replies.Reply
	Placeholders []string
	CanManage    bool
}

// In is the replies for one scope.
func (d repliesData) In(scope string) []replies.Reply {
	var out []replies.Reply
	for _, r := range d.Replies {
		if r.Scope == scope {
			out = append(out, r)
		}
	}
	return out
}

func (d *Deps) Replies(w http.ResponseWriter, r *http.Request) {
	data := repliesData{Base: baseFrom(r, "Saved Replies"), AdminShell: d.adminShell(r, "replies"),
		Scopes: replies.Scopes, Placeholders: replies.Placeholders, CanManage: d.can(r, "replies.manage")}
	var err error
	if data.Replies, err = replies.List(r.Context(), d.Pool, ""); err != nil {
		slog.Error("saved replies failed", "error", err)
		http.Error(w, "Failed to load saved replies.", http.StatusInternalServerError)
		return
	}
	d.Render.Render(w, "replies.html", data)
}

func (d *Deps) RepliesSave(w http.ResponseWriter, r *http.Request) {
	if !d.can(r, "replies.manage") {
		http.Redirect(w, r, "/admin/replies?error="+errMsg("You don't have permission to do that."), http.StatusSeeOther)
		return
	}
	sess, _ := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	_, err := replies.Save(r.Context(), d.Pool, sess.PlayerID, id, r.FormValue("scope"), r.FormValue("title"), r.FormValue("body"))
	finishReplies(w, r, err, "Saved.")
}

func (d *Deps) RepliesDelete(w http.ResponseWriter, r *http.Request) {
	if !d.can(r, "replies.manage") {
		http.Redirect(w, r, "/admin/replies?error="+errMsg("You don't have permission to do that."), http.StatusSeeOther)
		return
	}
	id, ok := profileID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	finishReplies(w, r, replies.Delete(r.Context(), d.Pool, id), "Deleted.")
}

func finishReplies(w http.ResponseWriter, r *http.Request, err error, ok string) {
	switch {
	case err == nil:
		http.Redirect(w, r, "/admin/replies?notice="+errMsg(ok), http.StatusSeeOther)
	case errors.Is(err, replies.ErrNotAllowed):
		m := strings.TrimPrefix(err.Error(), replies.ErrNotAllowed.Error()+": ")
		http.Redirect(w, r, "/admin/replies?error="+errMsg(strings.ToUpper(m[:1])+m[1:]+"."), http.StatusSeeOther)
	default:
		slog.Error("saved replies: save failed", "error", err)
		http.Redirect(w, r, "/admin/replies?error="+errMsg("Something went wrong. Nothing was saved."), http.StatusSeeOther)
	}
}

// savedReplies is one scope's replies for an "Insert a saved reply" menu
// (empty on error: the menu just doesn't show).
func (d *Deps) savedReplies(r *http.Request, scope string) []replies.Reply {
	rs, err := replies.List(r.Context(), d.Pool, scope)
	if err != nil {
		slog.Error("saved replies: list failed", "scope", scope, "error", err)
	}
	return rs
}
