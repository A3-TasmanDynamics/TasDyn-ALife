package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"website/internal/auth"
	"website/internal/gang"
)

// InviteToGang, RemoveFromGang, and SetGangRank all follow the same shape:
// parse the form, call the gang package (which re-checks leadership itself
// -- never trusted from which page rendered the form), and redirect back
// to the dashboard with a notice or a specific error message for the
// sentinel errors gang.go defines, a generic one for anything else.

func (d *Deps) InviteToGang(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/dashboard?error="+errMsg("Invalid form submission."), http.StatusSeeOther)
		return
	}

	err := gang.Invite(r.Context(), d.Pool, sess.PlayerID, r.FormValue("member_name"))
	redirectFromGangResult(w, r, err, "Member invited.")
}

func (d *Deps) RemoveFromGang(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/dashboard?error="+errMsg("Invalid form submission."), http.StatusSeeOther)
		return
	}
	memberID, err := strconv.ParseInt(r.FormValue("member_id"), 10, 64)
	if err != nil {
		http.Redirect(w, r, "/dashboard?error="+errMsg("Invalid member."), http.StatusSeeOther)
		return
	}

	err = gang.Remove(r.Context(), d.Pool, sess.PlayerID, memberID)
	redirectFromGangResult(w, r, err, "Member removed.")
}

func (d *Deps) SetGangRank(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/dashboard?error="+errMsg("Invalid form submission."), http.StatusSeeOther)
		return
	}
	memberID, err := strconv.ParseInt(r.FormValue("member_id"), 10, 64)
	if err != nil {
		http.Redirect(w, r, "/dashboard?error="+errMsg("Invalid member."), http.StatusSeeOther)
		return
	}

	err = gang.SetRank(r.Context(), d.Pool, sess.PlayerID, memberID, r.FormValue("rank"))
	redirectFromGangResult(w, r, err, "Rank updated.")
}

func redirectFromGangResult(w http.ResponseWriter, r *http.Request, err error, successNotice string) {
	if err != nil {
		if isKnownGangError(err) {
			http.Redirect(w, r, "/dashboard?error="+errMsg(err.Error()), http.StatusSeeOther)
			return
		}
		slog.Error("gang action failed", "error", err)
		http.Redirect(w, r, "/dashboard?error="+errMsg("Something went wrong."), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/dashboard?notice="+errMsg(successNotice), http.StatusSeeOther)
}

func isKnownGangError(err error) bool {
	switch err {
	case gang.ErrNotLeader, gang.ErrMemberNotFound, gang.ErrMemberAmbiguous, gang.ErrAlreadyInAGang,
		gang.ErrCannotManageSelf, gang.ErrMemberNotInGang, gang.ErrInvalidRank:
		return true
	default:
		return false
	}
}
