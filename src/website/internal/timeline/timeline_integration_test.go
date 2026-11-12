package timeline

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBuild(t *testing.T) {
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
	var p, staff, kase int64
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000861', 'Timeline Player') RETURNING id`).Scan(&p)
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000862', 'Timeline Staff') RETURNING id`).Scan(&staff)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM staff_cases WHERE id = $1`, kase)
		pool.Exec(ctx, `DELETE FROM banlist WHERE uid = '76561190000000861'`)
		pool.Exec(ctx, `DELETE FROM staff_log WHERE staff_player_id = $1`, staff)
		pool.Exec(ctx, `DELETE FROM players WHERE id IN ($1, $2)`, p, staff)
	})
	must := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatal(q, err)
		}
	}
	// One of each, an hour apart (10 h ago .. 1 h ago).
	must(`INSERT INTO player_sessions (player_id, faction, connected_at, disconnected_at) VALUES ($1, 'cop', now() - interval '10 hours', now() - interval '9 hours')`, p)
	must(`INSERT INTO player_log (player_id, event_type, details, created_at) VALUES ($1, 'shop.sell', '{"total":500}', now() - interval '9 hours 30 minutes')`, p)
	must(`INSERT INTO wanted_crimes (player_id, crime_key, bounty_amount, created_at) VALUES ($1, 'robbery', 5000, now() - interval '8 hours')`, p)
	must(`INSERT INTO anti_cheat_flags (player_id, flag_type, confidence, created_at) VALUES ($1, 'dupe', 'high', now() - interval '7 hours')`, p)
	pool.QueryRow(ctx, `INSERT INTO staff_cases (case_type, summary, created_at) VALUES ('exploit', 'Timeline case', now() - interval '6 hours') RETURNING id`).Scan(&kase)
	must(`INSERT INTO staff_case_participants (case_id, player_id, role) VALUES ($1, $2, 'subject')`, kase, p)
	must(`INSERT INTO staff_notes (player_id, author_id, kind, body, created_at) VALUES ($1, $2, 'player', 'Keep an eye on him', now() - interval '5 hours')`, p, staff)
	must(`INSERT INTO kick_log (target_player_id, kicked_by, kick_type, reason, created_at) VALUES ($1, $2, 'manual', 'Combat logging', now() - interval '4 hours')`, p, staff)
	must(`INSERT INTO banlist (uid, reason, banned_by, expires_at, created_at) VALUES ('76561190000000861', 'Duping', $1, now() + interval '3 days', now() - interval '3 hours')`, staff)
	must(`INSERT INTO staff_log (staff_player_id, target_player_id, action, reason, created_at) VALUES ($1, $2, 'player.licences', 'removed gun licence', now() - interval '2 hours')`, staff, p)
	must(`INSERT INTO game_commands (action, target_player_id, text, issued_by, created_at, status) VALUES ('message', $1, 'Come to the PD', $2, now() - interval '1 hour', 'done')`, p, staff)

	all := Opts{PlayerID: p, Limit: 50, Notes: true, Flags: true, Cases: true}
	items, more, err := Build(ctx, pool, all)
	if err != nil {
		t.Fatal(err)
	}
	if more {
		t.Error("more with everything on one page")
	}
	var kinds []string
	for i, it := range items {
		kinds = append(kinds, it.Kind)
		if i > 0 && it.When.After(items[i-1].When) {
			t.Errorf("not newest first at %d", i)
		}
	}
	want := []string{"command", "staff", "ban", "ban", "note", "case", "flag", "wanted", "event", "session"}
	if len(kinds) != len(want) {
		t.Fatalf("kinds %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Errorf("item %d: %s, want %s (%v)", i, kinds[i], want[i], kinds)
		}
	}
	if items[len(items)-1].URL != "/admin/players?id="+strconv.FormatInt(p, 10)+"&tab=sessions" || items[len(items)-1].Detail != "played 1h 0m" {
		t.Errorf("session item: %+v", items[len(items)-1])
	}

	// Without the permissions: no notes, flags or cases.
	limited, _, _ := Build(ctx, pool, Opts{PlayerID: p, Limit: 50})
	for _, it := range limited {
		if it.Kind == "note" || it.Kind == "flag" || it.Kind == "case" {
			t.Errorf("hidden kind shown: %+v", it)
		}
	}
	if len(limited) != 7 {
		t.Errorf("limited: %d items", len(limited))
	}
	// Only some kinds.
	only, _, _ := Build(ctx, pool, Opts{PlayerID: p, Limit: 50, Notes: true, Only: map[string]bool{"ban": true, "note": true}})
	if len(only) != 3 {
		t.Errorf("only bans+notes: %d", len(only))
	}
	// Paging: 4 at a time, no overlap, all 10 seen.
	seen := 0
	o := all
	o.Limit = 4
	for page := 0; page < 5; page++ {
		got, more, err := Build(ctx, pool, o)
		if err != nil {
			t.Fatal(err)
		}
		seen += len(got)
		if !more {
			break
		}
		o.Before = got[len(got)-1].When
	}
	if seen != 10 {
		t.Errorf("paged through %d items, want 10", seen)
	}
}
