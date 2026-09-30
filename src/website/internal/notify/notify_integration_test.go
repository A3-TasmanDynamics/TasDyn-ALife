package notify

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSafeLink(t *testing.T) {
	for in, want := range map[string]string{"/rules#r4": "/rules#r4", "//evil.example": "", "https://evil.example": "", "rules": "", "/x\\y": ""} {
		if got := SafeLink(in); got != want {
			t.Errorf("SafeLink(%q) = %q, want %q", in, got, want)
		}
	}
}

// Runs only with TEST_DATABASE_URL. Stop the live website first: Broadcast
// writes a staff_log row.
func TestNotificationsAndNotices(t *testing.T) {
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
	ids := map[string]int64{}
	for name, uid := range map[string]string{"staffA": "76561190000000701", "staffB": "76561190000000702", "suspended": "76561190000000703", "player": "76561190000000704"} {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ($1, $2) RETURNING id`, uid, "t_"+name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[name] = id
	}
	pool.Exec(ctx, `UPDATE players SET staff_rank_id = (SELECT id FROM staff_ranks ORDER BY level LIMIT 1) WHERE id = ANY($1)`,
		[]int64{ids["staffA"], ids["staffB"], ids["suspended"]})
	pool.Exec(ctx, `UPDATE players SET staff_status = 'suspended' WHERE id = $1`, ids["suspended"])
	all := []int64{ids["staffA"], ids["staffB"], ids["suspended"], ids["player"]}
	var noticeID int64
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM staff_notices WHERE id = $1`, noticeID)
		pool.Exec(ctx, `DELETE FROM staff_log WHERE staff_player_id = ANY($1) OR target_player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM rank_changes WHERE player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM players WHERE id = ANY($1)`, all)
	})

	if err := Send(ctx, pool, ids["player"], "Ticket #9: Priya replied", "Try re-logging.", "/tickets/9"); err != nil {
		t.Fatal(err)
	}
	if err := Send(ctx, pool, ids["player"], "Off-site", "", "https://evil.example"); err != nil {
		t.Fatal(err)
	}
	items, unread, err := List(ctx, pool, ids["player"], 10)
	if err != nil || unread != 2 || len(items) != 2 || items[0].Link != "" || items[1].Link != "/tickets/9" {
		t.Fatalf("list: unread=%d %+v %v", unread, items, err)
	}
	if link, _ := Open(ctx, pool, ids["player"], items[1].ID); link != "/tickets/9" {
		t.Errorf("open link = %q", link)
	}
	if link, _ := Open(ctx, pool, ids["staffA"], items[0].ID); link != "" {
		t.Error("opening someone else's notification must do nothing")
	}
	if err := MarkAllRead(ctx, pool, ids["player"]); err != nil {
		t.Fatal(err)
	}
	if _, unread, _ = List(ctx, pool, ids["player"], 10); unread != 0 {
		t.Errorf("unread after mark all = %d", unread)
	}

	tx, _ := pool.Begin(ctx)
	if _, _, err := Broadcast(ctx, tx, ids["staffA"], "Rule 4.3 changed", "Read it", "https://evil.example", "staff"); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("off-site link should be refused: %v", err)
	}
	tx.Rollback(ctx)
	tx, _ = pool.Begin(ctx)
	id, n, err := Broadcast(ctx, tx, ids["staffA"], "Rule 4.3 changed", "Read it", "/rules#r4", "staff")
	if err != nil {
		t.Fatal(err)
	}
	tx.Commit(ctx)
	noticeID = id
	var mine int
	pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE notice_id = $1 AND player_id = ANY($2)`, id, all).Scan(&mine)
	if mine != 2 || n < 2 {
		t.Errorf("staff notice reached %d of the test players (want the 2 active staff), %d total", mine, n)
	}
	p, err := PendingEssential(ctx, pool, ids["staffB"])
	if err != nil || p == nil || p.Title != "Rule 4.3 changed" || p.Link != "/rules#r4" {
		t.Fatalf("pending: %+v %v", p, err)
	}
	if p2, _ := PendingEssential(ctx, pool, ids["suspended"]); p2 != nil {
		t.Error("suspended staff shouldn't get staff notices")
	}
	if err := Acknowledge(ctx, pool, ids["staffB"], p.ID); err != nil {
		t.Fatal(err)
	}
	if p, _ = PendingEssential(ctx, pool, ids["staffB"]); p != nil {
		t.Error("acknowledged notice should no longer be pending")
	}
	notices, _ := Notices(ctx, pool, 5)
	if len(notices) == 0 || notices[0].ID != id || notices[0].Acked != 1 {
		t.Errorf("notices: %+v", notices)
	}
}
