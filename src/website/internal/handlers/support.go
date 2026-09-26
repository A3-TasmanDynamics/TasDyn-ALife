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
		SELECT st.id, st.subject, tc1.label, COALESCE(tc2.label, ''), st.status, st.priority, COALESCE(NULLIF(assignee.name, ''), 'Player #' || assignee.id, ''),
		       COALESCE(NULLIF(requester.name, ''), 'Player #' || requester.id), requester.uid, st.created_at
		FROM support_tickets st
		JOIN ticket_categories tc1 ON tc1.id = st.category_id
		LEFT JOIN ticket_categories tc2 ON tc2.id = st.subcategory_id
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
		if err := rows.Scan(&t.ID, &t.Subject, &t.CategoryLabel, &t.SubcategoryLabel, &t.Status, &t.Priority, &t.AssignedTo, &t.RequesterName, &t.RequesterUID, &createdAt); err == nil {
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
	TopCategories    []categoryOption
	FilterStatus     string
	FilterPriority   string
	FilterAssigned   string
	FilterCategory   string // a category_id as a string, or "all"
	SearchQuery      string
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
	filterCategory := r.URL.Query().Get("category")
	if filterCategory == "" {
		filterCategory = "all"
	}
	searchQuery := r.URL.Query().Get("q")

	topCats, _, err := fetchCategoryTree(r.Context(), d.Pool)
	if err != nil {
		slog.Error("support queue: category tree query failed", "error", err)
		http.Error(w, "Failed to load the queue.", http.StatusInternalServerError)
		return
	}

	data := supportQueueData{
		Base:             baseFrom(r, "Support Panel · Tickets"),
		ActiveSupportTab: "tickets",
		TopCategories:    topCats,
		FilterStatus:     filterStatus,
		FilterPriority:   filterPriority,
		FilterAssigned:   filterAssigned,
		FilterCategory:   filterCategory,
		SearchQuery:      searchQuery,
	}

	query := `
		SELECT st.id, st.subject, tc1.label, COALESCE(tc2.label, ''), st.status, st.priority, COALESCE(NULLIF(assignee.name, ''), 'Player #' || assignee.id, ''),
		       COALESCE(NULLIF(requester.name, ''), 'Player #' || requester.id), requester.uid, st.created_at
		FROM support_tickets st
		JOIN ticket_categories tc1 ON tc1.id = st.category_id
		LEFT JOIN ticket_categories tc2 ON tc2.id = st.subcategory_id
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

	if filterCategory != "all" {
		if categoryID, err := strconv.Atoi(filterCategory); err == nil {
			query += ` AND st.category_id = ` + nextArg(categoryID)
		}
	}

	switch filterAssigned {
	case "me":
		query += ` AND st.assigned_staff_id = ` + nextArg(sess.PlayerID)
	case "unassigned":
		query += ` AND st.assigned_staff_id IS NULL`
	} // "all" -> no filter

	if searchQuery != "" {
		// One placeholder, referenced three times -- pgx/Postgres both
		// allow reusing the same $N multiple times in one query, so this
		// is still a single bound parameter, not three separate ones a
		// caller could desync.
		p := nextArg("%" + searchQuery + "%")
		query += ` AND (st.subject ILIKE ` + p + ` OR requester.name ILIKE ` + p + ` OR requester.uid ILIKE ` + p + `)`
	}

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
		if err := rows.Scan(&t.ID, &t.Subject, &t.CategoryLabel, &t.SubcategoryLabel, &t.Status, &t.Priority, &t.AssignedTo, &t.RequesterName, &t.RequesterUID, &createdAt); err == nil {
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

// staffOption is one row of a "which staff member" dropdown -- who's
// eligible to be assigned a ticket. Resolved the same way session panel
// access is (rank default, overridden per-player if a
// staff_permission_overrides row exists for 'panel.support') rather than a
// simpler "everyone with a staff_rank_id" query, since a support-specific
// override can both grant it to someone whose rank wouldn't otherwise and
// revoke it from someone whose rank would -- see
// internal/auth/session.go's resolvePanelAccess for the same logic applied
// to one player at login instead of the whole staff list.
type staffOption struct {
	ID   int64
	Name string
}

func fetchSupportStaff(ctx context.Context, pool *pgxpool.Pool) ([]staffOption, error) {
	rows, err := pool.Query(ctx, `
		SELECT p.id, COALESCE(NULLIF(p.name, ''), 'Player #' || p.id)
		FROM players p
		LEFT JOIN staff_ranks sr ON sr.id = p.staff_rank_id
		LEFT JOIN staff_permission_overrides spo ON spo.player_id = p.id AND spo.command_key = 'panel.support'
		WHERE COALESCE(spo.allow, sr.default_support_panel, false)
		ORDER BY 2
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var staff []staffOption
	for rows.Next() {
		var s staffOption
		if err := rows.Scan(&s.ID, &s.Name); err == nil {
			staff = append(staff, s)
		}
	}
	return staff, rows.Err()
}

// AssignTicket sets (or clears) a ticket's assignee to any eligible staff
// member, not just the caller -- the NinjaOne-style single assignee
// dropdown, replacing separate "claim for myself"/"unassign" actions with
// one control that covers claiming, reassigning to someone else, and
// releasing back to the queue. staff_id="" means unassign. Re-validates
// the submitted staff_id is actually a current support-panel-eligible
// player server-side -- never trusts that the dropdown's own options were
// the ones a raw POST came from.
func (d *Deps) AssignTicket(w http.ResponseWriter, r *http.Request) {
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

	staffIDStr := r.FormValue("staff_id")
	if staffIDStr == "" {
		if _, err := d.Pool.Exec(r.Context(), `
			UPDATE support_tickets SET assigned_staff_id = NULL, updated_at = now() WHERE id = $1
		`, ticketID); err != nil {
			slog.Error("unassign ticket failed", "error", err)
		}
		http.Redirect(w, r, "/tickets/"+id, http.StatusSeeOther)
		return
	}

	staffID, err := strconv.ParseInt(staffIDStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/tickets/"+id+"?error="+errMsg("Invalid staff member."), http.StatusSeeOther)
		return
	}

	eligible, err := fetchSupportStaff(r.Context(), d.Pool)
	if err != nil {
		slog.Error("assign ticket: staff list query failed", "error", err)
		http.Error(w, "Something went wrong.", http.StatusInternalServerError)
		return
	}
	found := false
	for _, s := range eligible {
		if s.ID == staffID {
			found = true
			break
		}
	}
	if !found {
		http.Redirect(w, r, "/tickets/"+id+"?error="+errMsg("That player doesn't currently have Support Panel access."), http.StatusSeeOther)
		return
	}

	// Bump open -> pending on assignment, same as the old Claim action did
	// -- an assigned ticket nobody's looked at yet reads oddly as "open".
	// Only when currently open, so reassigning an already-pending/closed
	// ticket doesn't fight whatever status it's deliberately in.
	if _, err := d.Pool.Exec(r.Context(), `
		UPDATE support_tickets
		SET assigned_staff_id = $1, updated_at = now(), status = CASE WHEN status = 'open' THEN 'pending' ELSE status END
		WHERE id = $2
	`, staffID, ticketID); err != nil {
		slog.Error("assign ticket failed", "error", err)
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

// SetTicketCategory lets staff re-categorize a misfiled ticket -- the
// submitter's own choice at creation isn't always right, same reasoning
// priority is staff-adjustable. Re-validates the pair server-side via the
// same validateCategoryPair the create-ticket path uses, rather than
// trusting the edit form's own cascading-select JS to have kept them
// consistent.
func (d *Deps) SetTicketCategory(w http.ResponseWriter, r *http.Request) {
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

	categoryID, err := strconv.Atoi(r.FormValue("category_id"))
	if err != nil {
		http.Redirect(w, r, "/tickets/"+id+"?error="+errMsg("Please choose a category."), http.StatusSeeOther)
		return
	}
	var subcategoryID int
	if v := r.FormValue("subcategory_id"); v != "" {
		subcategoryID, _ = strconv.Atoi(v)
	}
	if err := validateCategoryPair(r.Context(), d.Pool, categoryID, subcategoryID); err != nil {
		http.Redirect(w, r, "/tickets/"+id+"?error="+errMsg(err.Error()), http.StatusSeeOther)
		return
	}

	var subcategoryArg any
	if subcategoryID != 0 {
		subcategoryArg = subcategoryID
	}
	if _, err := d.Pool.Exec(r.Context(), `
		UPDATE support_tickets SET category_id = $1, subcategory_id = $2, updated_at = now() WHERE id = $3
	`, categoryID, subcategoryArg, ticketID); err != nil {
		slog.Error("set ticket category failed", "error", err)
	}

	http.Redirect(w, r, "/tickets/"+id, http.StatusSeeOther)
}

// SetTicketSubject lets staff correct/clarify a ticket's title -- players
// don't always write a subject line that's actually useful for triage.
func (d *Deps) SetTicketSubject(w http.ResponseWriter, r *http.Request) {
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

	subject := r.FormValue("subject")
	if subject == "" {
		http.Redirect(w, r, "/tickets/"+id+"?error="+errMsg("Subject cannot be empty."), http.StatusSeeOther)
		return
	}

	if _, err := d.Pool.Exec(r.Context(), `
		UPDATE support_tickets SET subject = $1, updated_at = now() WHERE id = $2
	`, subject, ticketID); err != nil {
		slog.Error("set ticket subject failed", "error", err)
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
