package cases

// Player reports: support tickets filed as a report about another player
// (Player Report, Discord / TeamSpeak "Report a Member"). Moderators see
// them on the Reports page and turn one into a case; the ticket and the
// case stay linked (support_tickets.case_id), and closing the case tells
// the reporter on their ticket.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReportCategories are the ticket sub-categories that are reports.
var ReportCategories = []string{"gameplay_player_report", "discord_report_member", "teamspeak_report_member"}

// Report is a report ticket.
type Report struct {
	TicketID    int64
	Subject     string
	Category    string // e.g. "Gameplay · Player Report"
	Reporter    string
	ReporterID  int64
	Status      string // the ticket's: open / pending / closed
	Created     time.Time
	Report      string // the reporter's first message
	Messages    int
	CaseID      int64 // 0 = no case yet
	CaseStatus  string
	CaseOutcome string
}

const reportSelect = `
	SELECT t.id, t.subject, tc1.label || ' · ' || tc2.label, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id), p.id,
	       t.status, t.created_at,
	       COALESCE((SELECT m.body FROM support_ticket_messages m WHERE m.ticket_id = t.id AND m.author_player_id = t.player_id ORDER BY m.id LIMIT 1), ''),
	       (SELECT count(*) FROM support_ticket_messages m WHERE m.ticket_id = t.id AND NOT m.internal)::int,
	       COALESCE(t.case_id, 0), COALESCE(c.status, ''), COALESCE(c.outcome, '')
	FROM support_tickets t
	JOIN ticket_categories tc2 ON tc2.id = t.subcategory_id
	JOIN ticket_categories tc1 ON tc1.id = t.category_id
	JOIN players p ON p.id = t.player_id
	LEFT JOIN staff_cases c ON c.id = t.case_id`

func scanReport(row pgx.Row) (Report, error) {
	var r Report
	err := row.Scan(&r.TicketID, &r.Subject, &r.Category, &r.Reporter, &r.ReporterID, &r.Status, &r.Created, &r.Report, &r.Messages,
		&r.CaseID, &r.CaseStatus, &r.CaseOutcome)
	return r, err
}

// Reports lists report tickets. view: "new" (open, no case yet), "cases"
// (linked to a case), "closed", or "" (all). Oldest new ones first, so the
// longest-waiting is on top; the others newest first.
func Reports(ctx context.Context, pool *pgxpool.Pool, view string, limit int) ([]Report, error) {
	where, order := "", "t.created_at DESC"
	switch view {
	case "new":
		where, order = "AND t.case_id IS NULL AND t.status <> 'closed'", "t.created_at"
	case "cases":
		where = "AND t.case_id IS NOT NULL"
	case "closed":
		where = "AND t.status = 'closed' AND t.case_id IS NULL"
	}
	rows, err := pool.Query(ctx, reportSelect+` WHERE tc2.key = ANY($1) `+where+` ORDER BY `+order+` LIMIT $2`, ReportCategories, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Report
	for rows.Next() {
		r, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetReport is one report ticket (ok false if the ticket isn't a report).
func GetReport(ctx context.Context, pool *pgxpool.Pool, ticketID int64) (Report, bool, error) {
	r, err := scanReport(pool.QueryRow(ctx, reportSelect+` WHERE t.id = $1 AND tc2.key = ANY($2)`, ticketID, ReportCategories))
	if errors.Is(err, pgx.ErrNoRows) {
		return r, false, nil
	}
	return r, err == nil, err
}

// NewReports is how many reports have no case yet (the sidebar badge).
func NewReports(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `
		SELECT count(*) FROM support_tickets t JOIN ticket_categories tc ON tc.id = t.subcategory_id
		WHERE tc.key = ANY($1) AND t.case_id IS NULL AND t.status <> 'closed'`, ReportCategories).Scan(&n)
	return n, err
}

// LinkReport attaches a report ticket to a case just opened from it: the
// reporter joins the case as reporter, the report goes in as the case's
// first evidence, and the ticket gets an internal note (and moves from
// open to pending).
func LinkReport(ctx context.Context, pool *pgxpool.Pool, actor Actor, caseID, ticketID int64) error {
	rep, ok, err := GetReport(ctx, pool, ticketID)
	if err != nil {
		return err
	}
	if !ok {
		return notAllowed("ticket #%d isn't a player report", ticketID)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE support_tickets SET case_id = $2, updated_at = now(), status = CASE WHEN status = 'open' THEN 'pending' ELSE status END
		WHERE id = $1 AND case_id IS NULL`, ticketID, caseID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return notAllowed("ticket #%d is already linked to a case", ticketID)
	}
	if rep.ReporterID != actor.PlayerID {
		if _, err := tx.Exec(ctx, `INSERT INTO staff_case_participants (case_id, player_id, role) VALUES ($1, $2, 'reporter') ON CONFLICT DO NOTHING`, caseID, rep.ReporterID); err != nil {
			return err
		}
	}
	body := fmt.Sprintf("Opened from report ticket #%d (%s) by %s: %q", ticketID, rep.Category, rep.Reporter, rep.Subject)
	if rep.Report != "" {
		report := rep.Report
		if r := []rune(report); len(r) > 3000 {
			report = string(r[:3000]) + "…"
		}
		body += "\n\nThe report:\n" + report
	}
	if _, err := addEntry(ctx, tx, caseID, actor.PlayerID, "note", body, 0, fmt.Sprintf("support_tickets:%d", ticketID)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO support_ticket_messages (ticket_id, author_player_id, body, internal) VALUES ($1, $2, $3, true)`,
		ticketID, actor.PlayerID, fmt.Sprintf("Case #%d opened from this report.", caseID)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// reportClosedMessage is what the reporter sees when the case is closed.
// It says the report was dealt with, never what happened to anyone.
const reportClosedMessage = "Thanks for your report. Staff have looked into it and dealt with it. We don't share what action was taken against other players, but your report helped. This ticket is now closed; open a new one if anything else comes up."

// closeReports tells the reporters on a closed case's report tickets and
// closes those tickets (inside the case's own transaction).
func closeReports(ctx context.Context, tx pgx.Tx, caseID int64) error {
	rows, err := tx.Query(ctx, `SELECT id FROM support_tickets WHERE case_id = $1 AND status <> 'closed'`, caseID)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `INSERT INTO support_ticket_messages (ticket_id, author_player_id, body, internal) VALUES ($1, NULL, $2, false)`, id, reportClosedMessage); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE support_tickets SET status = 'closed', closed_at = now(), updated_at = now() WHERE id = $1`, id); err != nil {
			return err
		}
	}
	return nil
}
