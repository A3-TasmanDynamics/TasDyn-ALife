package cases

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The Anti-Cheat page's filters and summary. Runs only with TEST_DATABASE_URL.
func TestFlagFiltersAndSummary(t *testing.T) {
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

	var a, b, mod int64
	for _, p := range []struct {
		id   *int64
		uid  string
		name string
	}{{&a, "76561190000000401", "Flagtest Alpha"}, {&b, "76561190000000402", "Flagtest Bravo"}, {&mod, "76561190000000403", "Flagtest Mod"}} {
		if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ($1, $2) RETURNING id`, p.uid, p.name).Scan(p.id); err != nil {
			t.Fatal(err)
		}
	}
	all := []int64{a, b, mod}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM staff_cases WHERE lead_staff_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM staff_log WHERE staff_player_id = ANY($1) OR target_player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM anti_cheat_flags WHERE player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM players WHERE id = ANY($1)`, all)
	})
	// The summary counts every flag in the DB; measure the difference this test makes.
	before, err := FlagSummary(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	flag := func(p int64, typ, conf, age string) int64 {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO anti_cheat_flags (player_id, flag_type, confidence, details, created_at)
			VALUES ($1, $2, $3, '{}', now() - $4::interval) RETURNING id`, p, typ, conf, age).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	f1 := flag(a, "dupe", "high", "1 hour")
	f2 := flag(a, "rate_limit", "medium", "2 days")
	f3 := flag(b, "dupe", "medium", "3 hours")
	flag(b, "gear", "high", "40 days")
	actor := Actor{PlayerID: mod, Source: "website"}
	if err := ReviewFlag(ctx, pool, actor, f2, "watch", "keeping an eye on it"); err != nil {
		t.Fatal(err)
	}
	if err := ReviewFlag(ctx, pool, actor, f3, "dismiss", "lag"); err != nil {
		t.Fatal(err)
	}
	// Watching from a flag puts the player on the watchlist, once.
	var watchFlag int64
	var watchReason string
	if err := pool.QueryRow(ctx, `SELECT COALESCE(flag_id, 0), reason FROM player_watchlist WHERE player_id = $1`, a).Scan(&watchFlag, &watchReason); err != nil || watchFlag != f2 {
		t.Errorf("watchlist after a watch review: flag %d, %q, %v", watchFlag, watchReason, err)
	}
	if err := ReviewFlag(ctx, pool, actor, f1, "watch", "again"); err != nil {
		t.Fatal(err)
	}
	pool.QueryRow(ctx, `SELECT COALESCE(flag_id, 0) FROM player_watchlist WHERE player_id = $1`, a).Scan(&watchFlag)
	if watchFlag != f2 {
		t.Errorf("a second watch review replaced the watchlist entry (flag %d)", watchFlag)
	}
	if err := ReviewFlag(ctx, pool, actor, f1, "", ""); err != nil {
		t.Fatal(err)
	}

	count := func(what string, f FlagFilter, want int) []Flag {
		t.Helper()
		f.Search = "Flagtest" // only this test's players
		got, err := Flags(ctx, pool, f, 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != want {
			t.Errorf("%s: %d flags, want %d", what, len(got), want)
		}
		return got
	}
	count("everything", FlagFilter{}, 4)
	count("open", FlagFilter{Status: "open"}, 2)
	count("old OpenOnly", FlagFilter{OpenOnly: true}, 2)
	count("watching", FlagFilter{Status: "watch"}, 1)
	count("closed", FlagFilter{Status: "closed"}, 1)
	count("dupe", FlagFilter{Type: "dupe"}, 2)
	count("open high", FlagFilter{Status: "open", Confidence: "high"}, 2)
	if got := count("player a", FlagFilter{PlayerID: a}, 2); len(got) == 2 && got[0].ID != f2 && got[1].ID != f1 {
		t.Errorf("player a newest first: %d, %d", got[0].ID, got[1].ID)
	}
	if got, _ := Flags(ctx, pool, FlagFilter{Search: "76561190000000402"}, 100); len(got) != 2 {
		t.Errorf("Steam ID search: %d", len(got))
	}

	after, err := FlagSummary(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if d := after.Open - before.Open; d != 2 {
		t.Errorf("open +%d, want +2", d)
	}
	if d := after.OpenHigh - before.OpenHigh; d != 2 {
		t.Errorf("open high +%d, want +2", d)
	}
	if d := after.Watching - before.Watching; d != 1 {
		t.Errorf("watching +%d, want +1", d)
	}
	if d := after.Last24h - before.Last24h; d != 2 {
		t.Errorf("24 h +%d, want +2", d)
	}
	if d := after.Last7d - before.Last7d; d != 3 {
		t.Errorf("7 d +%d, want +3", d)
	}
	var sawDupe bool
	for _, tc := range after.ByType {
		if tc.Type == "dupe" {
			sawDupe = true
			if tc.Title != "Possible dupe (unexplained pile of gear)" {
				t.Errorf("dupe title %q", tc.Title)
			}
		}
	}
	if !sawDupe {
		t.Error("no dupe row by type")
	}
	for _, p := range after.TopPlayers {
		if p.PlayerID == b && (p.Flags != 1 || p.Open != 0) { // the 40-day one doesn't count
			t.Errorf("bravo 30-day: %+v", p)
		}
	}

	// Banning from a flag opens a case tied to it, which marks the flag handled.
	caseID, err := Open(ctx, pool, actor, "cheating", "Ban: Cheating (anti-cheat flag)", []int64{a}, f1, "Opened from anti-cheat flag")
	if err != nil {
		t.Fatal(err)
	}
	g, err := GetFlag(ctx, pool, f1)
	if err != nil || g.Resolution != "case" || g.CaseID != caseID {
		t.Errorf("flag after case: %+v err=%v", g, err)
	}
}
