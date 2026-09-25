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

type ticketSummary struct {
	ID         int64
	Subject    string
	Category   string
	Status     string
	AssignedTo string
}

type ticketMessage struct {
	AuthorName string
	Source     string
	Body       string
	CreatedAt  string
}

type myTicketsData struct {
	Base
	Tickets []ticketSummary
}

// MyTickets lists the logged-in player's own tickets and handles the "open
// a new ticket" form (member portal side of docs/WEBSITE.md §8 -- a ticket
// is one row viewed from two access levels, not two separate objects).
func (d *Deps) MyTickets(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	data := myTicketsData{Base: baseFrom(r, "My Tickets")}

	rows, err := d.Pool.Query(r.Context(), `
		SELECT id, subject, category, status FROM support_tickets
		WHERE player_id = $1 ORDER BY created_at DESC
	`, sess.PlayerID)
	if err != nil {
		slog.Error("my tickets: query failed", "error", err)
		http.Error(w, "Failed to load your tickets.", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var t ticketSummary
		if err := rows.Scan(&t.ID, &t.Subject, &t.Category, &t.Status); err == nil {
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
	category := r.FormValue("category")
	body := r.FormValue("body")
	if subject == "" || body == "" {
		http.Redirect(w, r, "/tickets?error="+errMsg("Subject and details are required."), http.StatusSeeOther)
		return
	}

	var ticketID int64
	tx, err := d.Pool.Begin(r.Context())
	if err != nil {
		http.Redirect(w, r, "/tickets?error="+errMsg("Something went wrong."), http.StatusSeeOther)
		return
	}
	defer tx.Rollback(r.Context())

	err = tx.QueryRow(r.Context(), `
		INSERT INTO support_tickets (player_id, subject, category) VALUES ($1, $2, $3) RETURNING id
	`, sess.PlayerID, subject, category).Scan(&ticketID)
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

	if err := tx.Commit(r.Context()); err != nil {
		http.Redirect(w, r, "/tickets?error="+errMsg("Something went wrong."), http.StatusSeeOther)
		return
	}

	discord.SendWebhook(r.Context(), d.Cfg.DiscordTicketLogWebhook,
		"🎫 New support ticket **#"+strconv.FormatInt(ticketID, 10)+"** ("+category+"): "+subject)

	http.Redirect(w, r, "/tickets/"+strconv.FormatInt(ticketID, 10), http.StatusSeeOther)
}

type ticketDetail struct {
	ID         int64
	Subject    string
	Category   string
	Status     string
	AssignedTo string
}

type ticketThreadData struct {
	Base
	Ticket   ticketDetail
	Messages []ticketMessage
	IsStaff  bool
}

// TicketThread renders one ticket's conversation. Accessible to the
// ticket's owner OR staff with Support Panel access -- checked here, not
// assumed from which route the request came in on, since /tickets/{id} is
// reachable by both audiences.
func (d *Deps) TicketThread(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	ticketID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var ownerID int64
	var assignedName *string
	t := ticketDetail{ID: ticketID}
	err = d.Pool.QueryRow(r.Context(), `
		SELECT st.player_id, st.subject, st.category, st.status, p.name
		FROM support_tickets st
		LEFT JOIN players p ON p.id = st.assigned_staff_id
		WHERE st.id = $1
	`, ticketID).Scan(&ownerID, &t.Subject, &t.Category, &t.Status, &assignedName)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("ticket thread: load failed", "error", err)
		http.Error(w, "Failed to load ticket.", http.StatusInternalServerError)
		return
	}
	if assignedName != nil {
		t.AssignedTo = *assignedName
	}

	isStaff := sess.SupportPanelAccess
	if ownerID != sess.PlayerID && !isStaff {
		http.Error(w, "403 Forbidden", http.StatusForbidden)
		return
	}

	data := ticketThreadData{Base: baseFrom(r, t.Subject), Ticket: t, IsStaff: isStaff}

	rows, err := d.Pool.Query(r.Context(), `
		SELECT COALESCE(p.name, 'System'), stm.source, stm.body, stm.created_at
		FROM support_ticket_messages stm
		LEFT JOIN players p ON p.id = stm.author_player_id
		WHERE stm.ticket_id = $1 ORDER BY stm.created_at ASC
	`, ticketID)
	if err != nil {
		slog.Error("ticket thread: messages query failed", "error", err)
		http.Error(w, "Failed to load ticket.", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var m ticketMessage
		var createdAt time.Time
		if err := rows.Scan(&m.AuthorName, &m.Source, &m.Body, &createdAt); err == nil {
			m.CreatedAt = createdAt.Format("2006-01-02 15:04")
			data.Messages = append(data.Messages, m)
		}
	}

	d.Render.Render(w, "ticket_thread.html", data)
}

// ReplyToTicket appends a message from whoever's viewing (owner or staff)
// -- same ownership/staff check as TicketThread, re-verified server-side
// rather than trusted from the page that rendered the form.
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
	if ownerID != sess.PlayerID && !sess.SupportPanelAccess {
		http.Error(w, "403 Forbidden", http.StatusForbidden)
		return
	}
	if status == "closed" {
		http.Redirect(w, r, "/tickets/"+chi.URLParam(r, "id")+"?error="+errMsg("This ticket is closed."), http.StatusSeeOther)
		return
	}

	_, err = d.Pool.Exec(r.Context(), `
		INSERT INTO support_ticket_messages (ticket_id, author_player_id, body, source) VALUES ($1, $2, $3, 'web')
	`, ticketID, sess.PlayerID, r.FormValue("body"))
	if err != nil {
		slog.Error("ticket reply: insert failed", "error", err)
		http.Redirect(w, r, "/tickets/"+chi.URLParam(r, "id")+"?error="+errMsg("Something went wrong."), http.StatusSeeOther)
		return
	}
	_, _ = d.Pool.Exec(r.Context(), `UPDATE support_tickets SET updated_at = now() WHERE id = $1`, ticketID)

	http.Redirect(w, r, "/tickets/"+chi.URLParam(r, "id"), http.StatusSeeOther)
}
