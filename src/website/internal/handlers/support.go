package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"website/internal/auth"
)

type supportQueueData struct {
	Base
	Tickets []ticketSummary
}

// SupportQueue lists open/pending tickets for staff with Support Panel
// access (gated by auth.RequireSupportPanel upstream, not re-checked here
// -- this handler trusts its middleware the same way every other
// panel-gated handler does).
func (d *Deps) SupportQueue(w http.ResponseWriter, r *http.Request) {
	data := supportQueueData{Base: baseFrom(r, "Support Panel")}

	rows, err := d.Pool.Query(r.Context(), `
		SELECT st.id, st.subject, st.category, st.status, COALESCE(p.name, '')
		FROM support_tickets st
		LEFT JOIN players p ON p.id = st.assigned_staff_id
		WHERE st.status != 'closed'
		ORDER BY st.created_at ASC
	`)
	if err != nil {
		slog.Error("support queue: query failed", "error", err)
		http.Error(w, "Failed to load the queue.", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var t ticketSummary
		if err := rows.Scan(&t.ID, &t.Subject, &t.Category, &t.Status, &t.AssignedTo); err == nil {
			data.Tickets = append(data.Tickets, t)
		}
	}

	d.Render.Render(w, "support_queue.html", data)
}

func (d *Deps) ClaimTicket(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	ticketID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	_, err = d.Pool.Exec(r.Context(), `
		UPDATE support_tickets SET assigned_staff_id = $1, status = 'pending', updated_at = now()
		WHERE id = $2 AND assigned_staff_id IS NULL
	`, sess.PlayerID, ticketID)
	if err != nil {
		slog.Error("claim ticket failed", "error", err)
	}

	http.Redirect(w, r, "/tickets/"+id, http.StatusSeeOther)
}

func (d *Deps) CloseTicket(w http.ResponseWriter, r *http.Request) {
	d.setTicketStatus(w, r, "closed")
}

func (d *Deps) ReopenTicket(w http.ResponseWriter, r *http.Request) {
	d.setTicketStatus(w, r, "open")
}

func (d *Deps) setTicketStatus(w http.ResponseWriter, r *http.Request, status string) {
	id := chi.URLParam(r, "id")
	ticketID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var query string
	if status == "closed" {
		query = `UPDATE support_tickets SET status = $1, updated_at = now(), closed_at = now() WHERE id = $2`
	} else {
		query = `UPDATE support_tickets SET status = $1, updated_at = now(), closed_at = NULL WHERE id = $2`
	}
	if _, err := d.Pool.Exec(r.Context(), query, status, ticketID); err != nil {
		slog.Error("set ticket status failed", "error", err, "status", status)
	}

	http.Redirect(w, r, "/tickets/"+id, http.StatusSeeOther)
}
