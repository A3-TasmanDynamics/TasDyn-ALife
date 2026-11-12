package handlers

// Bulk actions for the busy queues: review several anti-cheat flags at
// once, and assign / re-prioritise several tickets at once.

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"website/internal/auth"
	"website/internal/cases"
)

// maxBulk caps one bulk request.
const maxBulk = 200

// formIDs reads the ids[] checkboxes.
func formIDs(r *http.Request) []int64 {
	_ = r.ParseForm()
	var out []int64
	seen := map[int64]bool{}
	for _, v := range r.Form["ids"] {
		id, err := strconv.ParseInt(v, 10, 64)
		if err == nil && id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
		if len(out) == maxBulk {
			break
		}
	}
	return out
}

// safeBack is the list to return to: a same-site admin path only.
func safeBack(r *http.Request, fallback string) string {
	b := r.FormValue("back")
	if !strings.HasPrefix(b, "/admin/") || strings.HasPrefix(b, "//") || strings.ContainsAny(b, "\r\n") {
		return fallback
	}
	return b
}

func withParam(back, key, value string) string {
	sep := "?"
	if strings.Contains(back, "?") {
		sep = "&"
	}
	return back + sep + key + "=" + url.QueryEscape(value)
}

// FlagBulkReview dismisses or watches several flags with one note
// (anticheat.review). Each goes through cases.ReviewFlag, so the usual
// rules and the Staff Log apply; ones that can't be reviewed (handled in a
// case, already closed) are skipped and counted.
func (d *Deps) FlagBulkReview(w http.ResponseWriter, r *http.Request) {
	back := safeBack(r, "/admin/anticheat")
	ids := formIDs(r)
	res := r.FormValue("resolution")
	if res != "dismiss" && res != "watch" {
		http.Redirect(w, r, withParam(back, "error", "Choose dismiss or watch."), http.StatusSeeOther)
		return
	}
	if len(ids) == 0 {
		http.Redirect(w, r, withParam(back, "error", "Select at least one flag."), http.StatusSeeOther)
		return
	}
	note := r.FormValue("note")
	done, skipped := 0, 0
	var firstSkip string
	for _, id := range ids {
		err := cases.ReviewFlag(r.Context(), d.Pool, caseActor(r), id, res, note)
		switch {
		case err == nil:
			done++
		case errors.Is(err, cases.ErrNotAllowed):
			if skipped == 0 {
				firstSkip = fmt.Sprintf("#%d: %s", id, strings.TrimPrefix(err.Error(), cases.ErrNotAllowed.Error()+": "))
			}
			skipped++
			if strings.Contains(err.Error(), "add a note") {
				http.Redirect(w, r, withParam(back, "error", "Add a note saying why."), http.StatusSeeOther)
				return
			}
		default:
			slog.Error("bulk flag review failed", "flag", id, "error", err)
			skipped++
		}
	}
	verb := map[string]string{"dismiss": "Dismissed", "watch": "Marked as watching"}[res]
	msg := fmt.Sprintf("%s %d flag%s.", verb, done, plural(done))
	if skipped > 0 {
		msg += fmt.Sprintf(" Skipped %d", skipped)
		if firstSkip != "" {
			msg += " (" + firstSkip + ")"
		}
		msg += "."
	}
	key := "notice"
	if done == 0 {
		key = "error"
	}
	http.Redirect(w, r, withParam(back, key, msg), http.StatusSeeOther)
}

// TicketBulk assigns several tickets to the viewer, unassigns them, or sets
// their priority (support panel access).
func (d *Deps) TicketBulk(w http.ResponseWriter, r *http.Request) {
	back := safeBack(r, "/admin/tickets")
	ids := formIDs(r)
	if len(ids) == 0 {
		http.Redirect(w, r, withParam(back, "error", "Select at least one ticket."), http.StatusSeeOther)
		return
	}
	sess, _ := auth.FromContext(r.Context())
	var (
		sql  string
		args []any
		verb string
	)
	switch action := r.FormValue("action"); {
	case action == "assign_me":
		sql, args, verb = `UPDATE support_tickets SET assigned_staff_id = $2, updated_at = now(),
			status = CASE WHEN status = 'open' THEN 'pending' ELSE status END
			WHERE id = ANY($1) AND status <> 'closed'`, []any{ids, sess.PlayerID}, "Assigned"
	case action == "unassign":
		sql, args, verb = `UPDATE support_tickets SET assigned_staff_id = NULL, updated_at = now() WHERE id = ANY($1)`, []any{ids}, "Unassigned"
	case strings.HasPrefix(action, "priority:"):
		p := strings.TrimPrefix(action, "priority:")
		if normalizePriority(p) != p {
			http.Redirect(w, r, withParam(back, "error", "Unknown priority."), http.StatusSeeOther)
			return
		}
		sql, args, verb = `UPDATE support_tickets SET priority = $2, updated_at = now() WHERE id = ANY($1)`, []any{ids, p}, "Set to "+p+" priority:"
	default:
		http.Redirect(w, r, withParam(back, "error", "Unknown action."), http.StatusSeeOther)
		return
	}
	tag, err := d.Pool.Exec(r.Context(), sql, args...)
	if err != nil {
		slog.Error("bulk ticket action failed", "error", err)
		http.Redirect(w, r, withParam(back, "error", "Something went wrong. Nothing was changed."), http.StatusSeeOther)
		return
	}
	n := int(tag.RowsAffected())
	msg := fmt.Sprintf("%s %d ticket%s.", verb, n, plural(n))
	if r.FormValue("action") == "assign_me" {
		msg = fmt.Sprintf("Assigned %d ticket%s to you.", n, plural(n))
	}
	http.Redirect(w, r, withParam(back, "notice", msg), http.StatusSeeOther)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
