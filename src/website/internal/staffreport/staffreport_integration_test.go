package staffreport

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestReport(t *testing.T) {
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
	var a, b, idle, player int64
	ins := func(uid, name, rank string) int64 {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name, staff_rank_id) VALUES ($1, $2, (SELECT id FROM staff_ranks WHERE key = NULLIF($3, ''))) RETURNING id`, uid, name, rank).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	a, b, idle = ins("76561190000000881", "Report Alpha", "admin"), ins("76561190000000882", "Report Bravo", "moderator"), ins("76561190000000883", "Report Idle", "moderator")
	player = ins("76561190000000884", "Report Player", "")
	var kase, ticket int64
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM staff_cases WHERE id = $1`, kase)
		pool.Exec(ctx, `DELETE FROM support_tickets WHERE id = $1`, ticket)
		pool.Exec(ctx, `DELETE FROM banlist WHERE uid = '76561190000000884'`)
		pool.Exec(ctx, `DELETE FROM staff_log WHERE staff_player_id IN ($1, $2, $3)`, a, b, idle)
		pool.Exec(ctx, `DELETE FROM players WHERE id IN ($1, $2, $3, $4)`, a, b, idle, player)
	})
	must := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatal(q, err)
		}
	}
	// Alpha: two flag reviews (1 h and 3 h after raised), a case led and closed with an entry, a ban.
	must(`INSERT INTO anti_cheat_flags (player_id, flag_type, confidence, created_at, reviewed_by, reviewed_at, resolution)
	      VALUES ($1, 'gear', 'high', now() - interval '5 hours', $2, now() - interval '4 hours', 'dismiss'),
	             ($1, 'dupe', 'high', now() - interval '6 hours', $2, now() - interval '3 hours', 'watch')`, player, a)
	if err := pool.QueryRow(ctx, `INSERT INTO staff_cases (case_type, summary, lead_staff_id, status, created_at, closed_at) VALUES ('exploit', 'Report test case', $1, 'closed', now() - interval '2 days', now() - interval '1 day') RETURNING id`, a).Scan(&kase); err != nil {
		t.Fatal(err)
	}
	must(`INSERT INTO staff_case_entries (case_id, author_id, kind, body) VALUES ($1, $2, 'note', 'looked into it')`, kase, a)
	must(`INSERT INTO banlist (uid, reason, banned_by) VALUES ('76561190000000884', 'test', $1)`, a)
	// Bravo: answers a ticket 30 min after it was opened, adds an internal note, and a Staff Log action.
	if err := pool.QueryRow(ctx, `INSERT INTO support_tickets (player_id, subject, category_id, created_at) SELECT $1, 'Report ticket', id, now() - interval '2 hours' FROM ticket_categories ORDER BY id LIMIT 1 RETURNING id`, player).Scan(&ticket); err != nil {
		t.Fatal(err)
	}
	must(`INSERT INTO support_ticket_messages (ticket_id, author_player_id, body, created_at) VALUES ($1, $2, 'help', now() - interval '2 hours'), ($1, $3, 'on it', now() - interval '90 minutes')`, ticket, player, b)
	must(`INSERT INTO support_ticket_messages (ticket_id, author_player_id, body, internal) VALUES ($1, $2, 'note to staff', true)`, ticket, b)
	must(`INSERT INTO staff_log (staff_player_id, target_player_id, action) VALUES ($1, $2, 'player.warn')`, b, player)

	ms, err := Members(ctx, pool, time.Now().Add(-7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]Member{}
	order := map[int64]int{}
	for i, m := range ms {
		got[m.PlayerID], order[m.PlayerID] = m, i
	}
	if m := got[a]; m.FlagsReviewed != 2 || m.CasesOpened != 1 || m.CasesClosed != 1 || m.CaseEntries != 1 || m.Bans != 1 || m.LastActive == nil || m.Rank != "Admin" {
		t.Errorf("alpha: %+v", m)
	}
	if m := got[b]; m.TicketReplies != 1 || m.TicketNotes != 1 || m.Actions != 1 {
		t.Errorf("bravo: %+v", m)
	}
	if m, ok := got[idle]; !ok || m.Total() != 0 || m.LastActive != nil {
		t.Errorf("idle staff should be listed with nothing: %+v %v", m, ok)
	}
	if _, ok := got[player]; ok {
		t.Error("a non-staff player is listed")
	}
	if order[idle] < order[a] || order[idle] < order[b] {
		t.Error("idle staff should sort after busy staff")
	}

	team, err := TeamStats(ctx, pool, time.Now().Add(-7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if team.FlagsReviewed < 2 || team.TicketsAnswered < 1 || team.CasesClosed < 1 || team.FlagReviewMedian <= 0 || team.TicketFirstMedian <= 0 || team.CaseCloseMedian <= 0 {
		t.Errorf("team: %+v", team)
	}
	weeks, err := Weekly(ctx, pool, 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(weeks) != 12 {
		t.Fatalf("weeks: %d", len(weeks))
	}
	var flags, replies int
	for _, w := range weeks {
		flags += w.FlagsReviewed
		replies += w.TicketReplies
	}
	if flags < 2 || replies < 1 {
		t.Errorf("weekly: flags %d replies %d", flags, replies)
	}
}
