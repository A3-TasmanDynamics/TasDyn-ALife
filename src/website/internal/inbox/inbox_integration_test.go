package inbox

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCollect(t *testing.T) {
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
	var me, other, player int64
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000801', 'Inbox Me') RETURNING id`).Scan(&me)
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000802', 'Inbox Other') RETURNING id`).Scan(&other)
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000803', 'Inbox Player') RETURNING id`).Scan(&player)
	var mine, nobodys, report, closed, myCase, quiet, freshCase int64
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM support_tickets WHERE id IN ($1, $2, $3, $4)`, mine, nobodys, report, closed)
		pool.Exec(ctx, `DELETE FROM staff_cases WHERE id IN ($1, $2, $3)`, myCase, quiet, freshCase)
		pool.Exec(ctx, `DELETE FROM players WHERE id IN ($1, $2, $3)`, me, other, player)
	})
	ticket := func(sub, subject, status string, assignee any, prio string) int64 {
		var id int64
		if err := pool.QueryRow(ctx, `
			INSERT INTO support_tickets (player_id, subject, category_id, subcategory_id, status, assigned_staff_id, priority)
			SELECT $1, $2, s.parent_id, s.id, $4, $5, $6 FROM ticket_categories s WHERE s.key = $3 RETURNING id`, player, subject, sub, status, assignee, prio).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	mine = ticket("gameplay_bug", "Inbox mine", "pending", me, "normal")
	nobodys = ticket("panel_login", "Inbox nobody", "open", nil, "urgent")
	report = ticket("gameplay_player_report", "Inbox report", "open", nil, "normal")
	closed = ticket("gameplay_bug", "Inbox closed", "closed", nil, "normal")
	pool.QueryRow(ctx, `INSERT INTO staff_cases (case_type, summary, lead_staff_id) VALUES ('conduct', 'Inbox my case', $1) RETURNING id`, me).Scan(&myCase)
	pool.QueryRow(ctx, `INSERT INTO staff_cases (case_type, summary, lead_staff_id, created_at) VALUES ('conduct', 'Inbox quiet case', $1, now() - interval '5 days') RETURNING id`, other).Scan(&quiet)
	pool.QueryRow(ctx, `INSERT INTO staff_cases (case_type, summary, lead_staff_id) VALUES ('conduct', 'Inbox fresh case', $1) RETURNING id`, other).Scan(&freshCase)
	pool.Exec(ctx, `INSERT INTO anti_cheat_flags (player_id, flag_type, confidence, created_at) VALUES ($1, 'dupe', 'high', now() - interval '2 days'), ($1, 'gear', 'medium', now())`, player)

	find := func(items []Item, title string) *Item {
		for i := range items {
			if items[i].Title == title || items[i].Title == "#"+itoa(myCase)+" "+title || items[i].Title == "#"+itoa(quiet)+" "+title || items[i].Title == "#"+itoa(freshCase)+" "+title {
				return &items[i]
			}
		}
		return nil
	}
	everything := Viewer{PlayerID: me, Support: true, Perms: map[string]bool{"cases.view": true, "anticheat.review": true, "bans.appeal_review": true, "applications.view": true}}
	items, err := Collect(ctx, pool, everything)
	if err != nil {
		t.Fatal(err)
	}
	if it := find(items, "Inbox mine"); it == nil || !it.Mine || it.CanClaim {
		t.Errorf("my ticket: %+v", it)
	}
	if it := find(items, "Inbox nobody"); it == nil || it.Owner != "" || !it.CanClaim || it.Urgency != 3 {
		t.Errorf("unassigned urgent ticket: %+v", it)
	}
	if find(items, "Inbox closed") != nil {
		t.Error("a closed ticket is in the inbox")
	}
	var reportAsTicket, reportAsReport bool
	for _, it := range items {
		if it.Title == "Inbox report" {
			reportAsTicket = reportAsTicket || it.Kind == "ticket"
			reportAsReport = reportAsReport || it.Kind == "report"
		}
	}
	if reportAsTicket || !reportAsReport {
		t.Errorf("a report should show once, as a report (ticket %v, report %v)", reportAsTicket, reportAsReport)
	}
	if it := find(items, "Inbox my case"); it == nil || !it.Mine {
		t.Errorf("the case I lead: %+v", it)
	}
	if it := find(items, "Inbox quiet case"); it == nil || it.Mine {
		t.Errorf("someone else's quiet case: %+v", it)
	}
	if find(items, "Inbox fresh case") != nil {
		t.Error("someone else's active case shouldn't be in my inbox")
	}
	highFlag := false
	for _, it := range items {
		if it.Kind == "flag" && it.Detail == "Inbox Player · high confidence" {
			highFlag = true
			if it.Urgency != 3 {
				t.Errorf("a 2-day-old high flag should be urgent: %+v", it)
			}
		}
	}
	if !highFlag {
		t.Error("the high-confidence flag is missing")
	}
	for i := 1; i < len(items); i++ {
		if items[i].Urgency > items[i-1].Urgency {
			t.Fatalf("not most urgent first at %d", i)
		}
	}

	// Support-only: tickets (the report among them, as a ticket), nothing else.
	support, _ := Collect(ctx, pool, Viewer{PlayerID: me, Support: true, Perms: map[string]bool{}})
	for _, it := range support {
		if it.Kind != "ticket" {
			t.Errorf("support-only staff saw a %s", it.Kind)
		}
	}
	if find(support, "Inbox report") == nil {
		t.Error("support-only staff should still see the report ticket")
	}
	// Moderation-only: no tickets.
	mod, _ := Collect(ctx, pool, Viewer{PlayerID: me, Perms: map[string]bool{"cases.view": true}})
	for _, it := range mod {
		if it.Kind == "ticket" {
			t.Error("staff without Support access saw a ticket")
		}
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
