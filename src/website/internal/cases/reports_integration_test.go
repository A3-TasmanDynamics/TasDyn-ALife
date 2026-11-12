package cases

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestReports(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var mod, reporter, accused int64
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000821', 'Report Mod') RETURNING id`).Scan(&mod)
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000822', 'Report Reporter') RETURNING id`).Scan(&reporter)
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000823', 'Report Accused') RETURNING id`).Scan(&accused)
	var report, other, kase int64
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM support_tickets WHERE id IN ($1, $2)`, report, other)
		pool.Exec(ctx, `DELETE FROM staff_cases WHERE id = $1`, kase)
		pool.Exec(ctx, `DELETE FROM staff_log WHERE staff_player_id = $1`, mod)
		pool.Exec(ctx, `DELETE FROM players WHERE id IN ($1, $2, $3)`, mod, reporter, accused)
	})
	ticket := func(sub, subject string) int64 {
		var id int64
		if err := pool.QueryRow(ctx, `
			INSERT INTO support_tickets (player_id, subject, category_id, subcategory_id)
			SELECT $1, $2, s.parent_id, s.id FROM ticket_categories s WHERE s.key = $3 RETURNING id`, reporter, subject, sub).Scan(&id); err != nil {
			t.Fatal(sub, err)
		}
		pool.Exec(ctx, `INSERT INTO support_ticket_messages (ticket_id, author_player_id, body) VALUES ($1, $2, $3)`, id, reporter, "He ran me over at Kavala twice.")
		return id
	}
	report = ticket("gameplay_player_report", "RDM by Report Accused")
	other = ticket("gameplay_bug", "Car fell through the map")

	news, err := Reports(ctx, pool, "new", 500)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range news {
		if r.TicketID == other {
			t.Error("a bug report is listed as a player report")
		}
		if r.TicketID == report {
			found = r.Report == "He ran me over at Kavala twice." && r.Reporter == "Report Reporter" && strings.Contains(r.Category, "Player Report")
		}
	}
	if !found {
		t.Fatal("the report isn't in the waiting list")
	}
	n, _ := NewReports(ctx, pool)
	if n < 1 {
		t.Errorf("new reports: %d", n)
	}

	actor := Actor{PlayerID: mod, Source: "website"}
	if kase, err = Open(ctx, pool, actor, "rdm_vdm", "RDM by Report Accused", []int64{accused}, 0, ""); err != nil {
		t.Fatal(err)
	}
	if err := LinkReport(ctx, pool, actor, kase, other); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("linking a non-report: %v", err)
	}
	if err := LinkReport(ctx, pool, actor, kase, report); err != nil {
		t.Fatal(err)
	}
	if err := LinkReport(ctx, pool, actor, kase, report); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("linking twice: %v", err)
	}
	var caseID int64
	var status string
	pool.QueryRow(ctx, `SELECT COALESCE(case_id, 0), status FROM support_tickets WHERE id = $1`, report).Scan(&caseID, &status)
	if caseID != kase || status != "pending" {
		t.Errorf("ticket after link: case %d status %s", caseID, status)
	}
	var role string
	pool.QueryRow(ctx, `SELECT role FROM staff_case_participants WHERE case_id = $1 AND player_id = $2`, kase, reporter).Scan(&role)
	if role != "reporter" {
		t.Errorf("reporter's role: %q", role)
	}
	var entry string
	pool.QueryRow(ctx, `SELECT body FROM staff_case_entries WHERE case_id = $1 AND action_ref = $2`, kase, "support_tickets:"+itoa(report)).Scan(&entry)
	if !strings.Contains(entry, "He ran me over at Kavala twice.") {
		t.Errorf("case entry: %q", entry)
	}
	var internalNotes int
	pool.QueryRow(ctx, `SELECT count(*) FROM support_ticket_messages WHERE ticket_id = $1 AND internal`, report).Scan(&internalNotes)
	if internalNotes != 1 {
		t.Errorf("internal notes on the ticket: %d", internalNotes)
	}

	// Closing the case tells the reporter (publicly) and closes the ticket.
	if err := SetStatus(ctx, pool, actor, kase, "closed", "2-day ban", "dealt with"); err != nil {
		t.Fatal(err)
	}
	var body string
	var internal bool
	pool.QueryRow(ctx, `SELECT body, internal FROM support_ticket_messages WHERE ticket_id = $1 AND author_player_id IS NULL ORDER BY id DESC LIMIT 1`, report).Scan(&body, &internal)
	pool.QueryRow(ctx, `SELECT status FROM support_tickets WHERE id = $1`, report).Scan(&status)
	if internal || !strings.Contains(body, "Thanks for your report") || strings.Contains(body, "ban") || status != "closed" {
		t.Errorf("after close: %q internal=%v status=%s", body, internal, status)
	}
}

func itoa(n int64) string {
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
