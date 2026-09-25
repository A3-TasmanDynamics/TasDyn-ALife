package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/auth"
)

type queueStats struct {
	Open         int
	Unassigned   int
	AssignedToMe int
	Urgent       int
}

type supportDashboardData struct {
	Base
	ActiveSupportTab string
	Stats            queueStats
	RecentTickets    []ticketSummary
}

// SupportDashboard is the Support Panel's landing page: the stats bar plus
// a snapshot of the 5 most recent tickets -- an overview to land on, not
// the working queue itself (that's SupportQueue/"Tickets", the second
// sidebar page). Same split every real IT ticketing system makes between
// an at-a-glance dashboard and the full filterable list.
func (d *Deps) SupportDashboard(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	data := supportDashboardData{Base: baseFrom(r, "Support Panel"), ActiveSupportTab: "dashboard"}

	stats, err := fetchQueueStats(r.Context(), d.Pool, sess.PlayerID)
	if err != nil {
		slog.Error("support dashboard: stats query failed", "error", err)
		http.Error(w, "Failed to load the dashboard.", http.StatusInternalServerError)
		return
	}
	data.Stats = stats

	rows, err := d.Pool.Query(r.Context(), `
		SELECT st.id, st.subject, st.category, st.status, st.priority, COALESCE(assignee.name, ''),
		       requester.name, st.created_at
		FROM support_tickets st
		JOIN players requester ON requester.id = st.player_id
		LEFT JOIN players assignee ON assignee.id = st.assigned_staff_id
		ORDER BY st.created_at DESC
		LIMIT 5
	`)
	if err != nil {
		slog.Error("support dashboard: recent tickets query failed", "error", err)
		http.Error(w, "Failed to load the dashboard.", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var t ticketSummary
		var createdAt time.Time
		if err := rows.Scan(&t.ID, &t.Subject, &t.Category, &t.Status, &t.Priority, &t.AssignedTo, &t.RequesterName, &createdAt); err == nil {
			t.CreatedAt = createdAt.Format("2006-01-02 15:04")
			data.RecentTickets = append(data.RecentTickets, t)
		}
	}

	d.Render.Render(w, "support_dashboard.html", data)
}

type supportQueueData struct {
	Base
	ActiveSupportTab string
	Tickets          []ticketSummary
	FilterStatus     string
	FilterPriority   string
	FilterAssigned   string
}

// SupportQueue is the Support Panel's "Tickets" page: the full
// filterable/sortable working queue (gated by auth.RequireSupportPanel
// upstream, not re-checked here -- this handler trusts its middleware the
// same way every other panel-gated handler does). Filterable by
// status/priority/assignment via query params -- a plain server-rendered
// GET with query-string state, not client-side JS filtering, matching this
// app's no-SPA-framework approach everywhere else.
func (d *Deps) SupportQueue(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())

	filterStatus := r.URL.Query().Get("status")
	if filterStatus == "" {
		filterStatus = "active" // open + pending, everything not closed -- the default triage view
	}
	filterPriority := r.URL.Query().Get("priority")
	if filterPriority == "" {
		filterPriority = "all"
	}
	filterAssigned := r.URL.Query().Get("assigned")
	if filterAssigned == "" {
		filterAssigned = "all"
	}

	data := supportQueueData{
		Base:             baseFrom(r, "Support Panel · Tickets"),
		ActiveSupportTab: "tickets",
		FilterStatus:     filterStatus,
		FilterPriority:   filterPriority,
		FilterAssigned:   filterAssigned,
	}

	query := `
		SELECT st.id, st.subject, st.category, st.status, st.priority, COALESCE(assignee.name, ''),
		       requester.name, st.created_at
		FROM support_tickets st
		JOIN players requester ON requester.id = st.player_id
		LEFT JOIN players assignee ON assignee.id = st.assigned_staff_id
		WHERE 1=1
	`
	args := []any{}
	argN := 0
	nextArg := func(v any) string {
		argN++
		args = append(args, v)
		return "$" + strconv.Itoa(argN)
	}

	switch filterStatus {
	case "active":
		query += ` AND st.status != 'closed'`
	case "open", "pending", "closed":
		query += ` AND st.status = ` + nextArg(filterStatus)
	} // "all" -> no filter

	if filterPriority != "all" && validPriorities[filterPriority] {
		query += ` AND st.priority = ` + nextArg(filterPriority)
	}

	switch filterAssigned {
	case "me":
		query += ` AND st.assigned_staff_id = ` + nextArg(sess.PlayerID)
	case "unassigned":
		query += ` AND st.assigned_staff_id IS NULL`
	} // "all" -> no filter

	query += ` ORDER BY CASE st.priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'normal' THEN 2 ELSE 3 END, st.created_at ASC`

	rows, err := d.Pool.Query(r.Context(), query, args...)
	if err != nil {
		slog.Error("support queue: query failed", "error", err)
		http.Error(w, "Failed to load the queue.", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var t ticketSummary
		var createdAt time.Time
		if err := rows.Scan(&t.ID, &t.Subject, &t.Category, &t.Status, &t.Priority, &t.AssignedTo, &t.RequesterName, &createdAt); err == nil {
			t.CreatedAt = createdAt.Format("2006-01-02 15:04")
			data.Tickets = append(data.Tickets, t)
		}
	}

	d.Render.Render(w, "support_queue.html", data)
}

func fetchQueueStats(ctx context.Context, pool *pgxpool.Pool, staffPlayerID int64) (queueStats, error) {
	var s queueStats
	err := pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status = 'open'),
			count(*) FILTER (WHERE assigned_staff_id IS NULL AND status != 'closed'),
			count(*) FILTER (WHERE assigned_staff_id = $1 AND status != 'closed'),
			count(*) FILTER (WHERE priority = 'urgent' AND status != 'closed')
		FROM support_tickets
	`, staffPlayerID).Scan(&s.Open, &s.Unassigned, &s.AssignedToMe, &s.Urgent)
	return s, err
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

// UnassignTicket releases a ticket back to the unassigned queue -- the
// symmetric counterpart to ClaimTicket, for when the assigned staff member
// can't actually work it (out of scope, handed to someone else, etc.).
// Deliberately doesn't restrict to "only the assignee can unassign" --
// any staff member with Support Panel access can, same as they could just
// claim someone else's unclaimed ticket; reassignment coordination is a
// team/Discord problem, not one this app enforces.
func (d *Deps) UnassignTicket(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ticketID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	_, err = d.Pool.Exec(r.Context(), `
		UPDATE support_tickets SET assigned_staff_id = NULL, updated_at = now() WHERE id = $1
	`, ticketID)
	if err != nil {
		slog.Error("unassign ticket failed", "error", err)
	}

	http.Redirect(w, r, "/tickets/"+id, http.StatusSeeOther)
}

// SetTicketPriority lets staff re-triage a ticket's priority after actually
// looking at it -- the submitter's own choice at creation (tickets.go) is
// a starting signal, not locked in.
func (d *Deps) SetTicketPriority(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ticketID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/tickets/"+id, http.StatusSeeOther)
		return
	}
	priority := normalizePriority(r.FormValue("priority"))

	if _, err := d.Pool.Exec(r.Context(), `
		UPDATE support_tickets SET priority = $1, updated_at = now() WHERE id = $2
	`, priority, ticketID); err != nil {
		slog.Error("set ticket priority failed", "error", err)
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
