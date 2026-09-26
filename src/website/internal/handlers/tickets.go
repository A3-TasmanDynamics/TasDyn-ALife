package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"website/internal/auth"
	"website/internal/discord"
)

var validPriorities = map[string]bool{"low": true, "normal": true, "high": true, "urgent": true}

func normalizePriority(p string) string {
	if validPriorities[p] {
		return p
	}
	return "normal"
}

type ticketSummary struct {
	ID               int64
	Subject          string
	CategoryLabel    string
	SubcategoryLabel string
	Status           string
	Priority         string
	AssignedTo       string
	RequesterName    string
	RequesterUID     string
	CreatedAt        string
}

type ticketMessage struct {
	AuthorName string
	Source     string
	Body       string
	Internal   bool
	CreatedAt  string
}

type myTicketsData struct {
	Base
	Tickets       []ticketSummary
	TopCategories []categoryOption
	Subcategories []categoryOption
}

// MyTickets lists the logged-in player's own tickets and handles the "open
// a new ticket" form (member portal side of docs/WEBSITE.md §8 -- a ticket
// is one row viewed from two access levels, not two separate objects).
func (d *Deps) MyTickets(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	data := myTicketsData{Base: baseFrom(r, "My Tickets")}

	topCats, subCats, err := fetchCategoryTree(r.Context(), d.Pool)
	if err != nil {
		slog.Error("my tickets: category tree query failed", "error", err)
		http.Error(w, "Failed to load the ticket form.", http.StatusInternalServerError)
		return
	}
	data.TopCategories = topCats
	data.Subcategories = subCats

	rows, err := d.Pool.Query(r.Context(), `
		SELECT st.id, st.subject, tc1.label, COALESCE(tc2.label, ''), st.status, st.priority
		FROM support_tickets st
		JOIN ticket_categories tc1 ON tc1.id = st.category_id
		LEFT JOIN ticket_categories tc2 ON tc2.id = st.subcategory_id
		WHERE st.player_id = $1 ORDER BY st.created_at DESC
	`, sess.PlayerID)
	if err != nil {
		slog.Error("my tickets: query failed", "error", err)
		http.Error(w, "Failed to load your tickets.", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var t ticketSummary
		if err := rows.Scan(&t.ID, &t.Subject, &t.CategoryLabel, &t.SubcategoryLabel, &t.Status, &t.Priority); err == nil {
			data.Tickets = append(data.Tickets, t)
		}
	}

	d.Render.Render(w, "my_tickets.html", data)
}

// CreateTicket handles the new-ticket form POST. Fires a one-way Discord
// webhook (docs/WEBSITE.md §9) on success -- best-effort, never blocks
// ticket creation on Discord being reachable.
func (d *Deps) CreateTicket(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/tickets?error="+errMsg("Invalid form submission."), http.StatusSeeOther)
		return
	}
	subject := r.FormValue("subject")
	body := r.FormValue("body")
	priority := normalizePriority(r.FormValue("priority"))
	if subject == "" || body == "" {
		http.Redirect(w, r, "/tickets?error="+errMsg("Subject and details are required."), http.StatusSeeOther)
		return
	}

	categoryID, err := strconv.Atoi(r.FormValue("category_id"))
	if err != nil {
		http.Redirect(w, r, "/tickets?error="+errMsg("Please choose a category."), http.StatusSeeOther)
		return
	}
	var subcategoryID int
	if v := r.FormValue("subcategory_id"); v != "" {
		subcategoryID, _ = strconv.Atoi(v) // 0 on parse failure -- validateCategoryPair below treats that as "none"
	}
	if err := validateCategoryPair(r.Context(), d.Pool, categoryID, subcategoryID); err != nil {
		http.Redirect(w, r, "/tickets?error="+errMsg(err.Error()), http.StatusSeeOther)
		return
	}

	var ticketID int64
	var categoryLabel string
	tx, err := d.Pool.Begin(r.Context())
	if err != nil {
		http.Redirect(w, r, "/tickets?error="+errMsg("Something went wrong."), http.StatusSeeOther)
		return
	}
	defer tx.Rollback(r.Context())

	var subcategoryArg any
	if subcategoryID != 0 {
		subcategoryArg = subcategoryID
	}

	err = tx.QueryRow(r.Context(), `
		INSERT INTO support_tickets (player_id, subject, category_id, subcategory_id, priority)
		VALUES ($1, $2, $3, $4, $5) RETURNING id
	`, sess.PlayerID, subject, categoryID, subcategoryArg, priority).Scan(&ticketID)
	if err != nil {
		slog.Error("create ticket: insert failed", "error", err)
		http.Redirect(w, r, "/tickets?error="+errMsg("Something went wrong."), http.StatusSeeOther)
		return
	}

	_, err = tx.Exec(r.Context(), `
		INSERT INTO support_ticket_messages (ticket_id, author_player_id, body, source) VALUES ($1, $2, $3, 'web')
	`, ticketID, sess.PlayerID, body)
	if err != nil {
		slog.Error("create ticket: message insert failed", "error", err)
		http.Redirect(w, r, "/tickets?error="+errMsg("Something went wrong."), http.StatusSeeOther)
		return
	}

	if err := tx.QueryRow(r.Context(), `SELECT label FROM ticket_categories WHERE id = $1`, categoryID).Scan(&categoryLabel); err != nil {
		categoryLabel = "?"
	}

	if err := tx.Commit(r.Context()); err != nil {
		http.Redirect(w, r, "/tickets?error="+errMsg("Something went wrong."), http.StatusSeeOther)
		return
	}

	priorityTag := map[string]string{"low": "", "normal": "", "high": "⚠️ ", "urgent": "🔴 "}[priority]
	discord.SendWebhook(r.Context(), d.Cfg.DiscordTicketLogWebhook,
		"🎫 "+priorityTag+"New support ticket **#"+strconv.FormatInt(ticketID, 10)+"** ("+categoryLabel+", "+priority+"): "+subject)

	http.Redirect(w, r, "/tickets/"+strconv.FormatInt(ticketID, 10), http.StatusSeeOther)
}

type ticketDetail struct {
	ID                       int64
	Subject                  string
	CategoryID               int
	CategoryLabel            string
	SubcategoryID            int // 0 = none
	SubcategoryLabel         string
	Status                   string
	Priority                 string
	AssignedStaffID          int64 // 0 = unassigned
	AssignedTo               string
	RequesterName            string
	RequesterUID             string
	RequesterDiscordID       string
	RequesterDiscordUsername string
	CreatedAt                string
	UpdatedAt                string
}

type ticketThreadData struct {
	Base
	Ticket        ticketDetail
	Messages      []ticketMessage
	IsStaff       bool
	TopCategories []categoryOption // for the staff-only "edit category" form
	Subcategories []categoryOption
	StaffOptions  []staffOption // for the staff-only "assign to" dropdown
}

// TicketThread renders one ticket's conversation plus, for staff, a
// metadata/identity sidebar (docs/WEBSITE.md §8) -- accessible to the
// ticket's owner OR staff with Support Panel access, checked here rather
// than assumed from which route the request came in on, since
// /tickets/{id} is reachable by both audiences. Internal notes
// (support_ticket_messages.internal) are filtered out in the SQL itself
// for a non-staff viewer, not just hidden in the template -- the owner's
// response should never even leave the database, let alone reach the page
// as hidden markup.
func (d *Deps) TicketThread(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	ticketID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var ownerID int64
	var assignedStaffID *int64
	var subcategoryID *int
	var assignedName, discordID, discordUsername *string
	var createdAt, updatedAt time.Time
	t := ticketDetail{ID: ticketID}
	err = d.Pool.QueryRow(r.Context(), `
		SELECT st.player_id, st.subject, st.category_id, tc1.label, st.subcategory_id, COALESCE(tc2.label, ''),
		       st.status, st.priority, st.assigned_staff_id,
		       COALESCE(NULLIF(p.name, ''), 'Player #' || p.id),
		       COALESCE(NULLIF(requester.name, ''), 'Player #' || requester.id),
		       requester.uid, requester.discord_id, requester.discord_username,
		       st.created_at, st.updated_at
		FROM support_tickets st
		JOIN ticket_categories tc1 ON tc1.id = st.category_id
		LEFT JOIN ticket_categories tc2 ON tc2.id = st.subcategory_id
		LEFT JOIN players p ON p.id = st.assigned_staff_id
		JOIN players requester ON requester.id = st.player_id
		WHERE st.id = $1
	`, ticketID).Scan(&ownerID, &t.Subject, &t.CategoryID, &t.CategoryLabel, &subcategoryID, &t.SubcategoryLabel,
		&t.Status, &t.Priority, &assignedStaffID, &assignedName,
		&t.RequesterName, &t.RequesterUID, &discordID, &discordUsername, &createdAt, &updatedAt)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("ticket thread: load failed", "error", err)
		http.Error(w, "Failed to load ticket.", http.StatusInternalServerError)
		return
	}
	if subcategoryID != nil {
		t.SubcategoryID = *subcategoryID
	}
	if assignedStaffID != nil {
		t.AssignedStaffID = *assignedStaffID
	}
	if assignedName != nil {
		t.AssignedTo = *assignedName
	}
	if discordID != nil {
		t.RequesterDiscordID = *discordID
	}
	if discordUsername != nil {
		t.RequesterDiscordUsername = *discordUsername
	}
	t.CreatedAt = createdAt.Format("2006-01-02 15:04")
	t.UpdatedAt = updatedAt.Format("2006-01-02 15:04")

	isStaff := sess.SupportPanelAccess
	if ownerID != sess.PlayerID && !isStaff {
		http.Error(w, "403 Forbidden", http.StatusForbidden)
		return
	}

	data := ticketThreadData{Base: baseFrom(r, t.Subject), Ticket: t, IsStaff: isStaff}

	if isStaff {
		topCats, subCats, err := fetchCategoryTree(r.Context(), d.Pool)
		if err != nil {
			slog.Error("ticket thread: category tree query failed", "error", err)
			http.Error(w, "Failed to load ticket.", http.StatusInternalServerError)
			return
		}
		data.TopCategories = topCats
		data.Subcategories = subCats

		staffOpts, err := fetchSupportStaff(r.Context(), d.Pool)
		if err != nil {
			slog.Error("ticket thread: staff list query failed", "error", err)
			http.Error(w, "Failed to load ticket.", http.StatusInternalServerError)
			return
		}
		data.StaffOptions = staffOpts
	}

	rows, err := d.Pool.Query(r.Context(), `
		SELECT COALESCE(p.name, 'System'), stm.source, stm.body, stm.internal, stm.created_at
		FROM support_ticket_messages stm
		LEFT JOIN players p ON p.id = stm.author_player_id
		WHERE stm.ticket_id = $1 AND (NOT stm.internal OR $2)
		ORDER BY stm.created_at ASC
	`, ticketID, isStaff)
	if err != nil {
		slog.Error("ticket thread: messages query failed", "error", err)
		http.Error(w, "Failed to load ticket.", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var m ticketMessage
		var msgCreatedAt time.Time
		if err := rows.Scan(&m.AuthorName, &m.Source, &m.Body, &m.Internal, &msgCreatedAt); err == nil {
			m.CreatedAt = msgCreatedAt.Format("2006-01-02 15:04")
			data.Messages = append(data.Messages, m)
		}
	}

	d.Render.Render(w, "ticket_thread.html", data)
}

// ReplyToTicket appends a message from whoever's viewing (owner or staff)
// -- same ownership/staff check as TicketThread, re-verified server-side
// rather than trusted from the page that rendered the form. "internal" is
// only ever honored when the submitter actually has Support Panel access
// -- a player's own reply can never become an internal note no matter what
// the submitted form says, since that field only exists in the page's
// staff-only UI branch but nothing stops a raw POST from including it.
func (d *Deps) ReplyToTicket(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	ticketID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil || r.FormValue("body") == "" {
		http.Redirect(w, r, "/tickets/"+chi.URLParam(r, "id")+"?error="+errMsg("Reply cannot be empty."), http.StatusSeeOther)
		return
	}

	var ownerID int64
	var status string
	err = d.Pool.QueryRow(r.Context(), `SELECT player_id, status FROM support_tickets WHERE id = $1`, ticketID).Scan(&ownerID, &status)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load ticket.", http.StatusInternalServerError)
		return
	}
	isStaff := sess.SupportPanelAccess
	if ownerID != sess.PlayerID && !isStaff {
		http.Error(w, "403 Forbidden", http.StatusForbidden)
		return
	}
	if status == "closed" {
		http.Redirect(w, r, "/tickets/"+chi.URLParam(r, "id")+"?error="+errMsg("This ticket is closed."), http.StatusSeeOther)
		return
	}

	internal := isStaff && r.FormValue("internal") == "1"

	_, err = d.Pool.Exec(r.Context(), `
		INSERT INTO support_ticket_messages (ticket_id, author_player_id, body, source, internal) VALUES ($1, $2, $3, 'web', $4)
	`, ticketID, sess.PlayerID, r.FormValue("body"), internal)
	if err != nil {
		slog.Error("ticket reply: insert failed", "error", err)
		http.Redirect(w, r, "/tickets/"+chi.URLParam(r, "id")+"?error="+errMsg("Something went wrong."), http.StatusSeeOther)
		return
	}
	_, _ = d.Pool.Exec(r.Context(), `UPDATE support_tickets SET updated_at = now() WHERE id = $1`, ticketID)

	http.Redirect(w, r, "/tickets/"+chi.URLParam(r, "id"), http.StatusSeeOther)
}
