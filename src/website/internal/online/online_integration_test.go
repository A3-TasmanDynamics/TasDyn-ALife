package online

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOnline(t *testing.T) {
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
	for name, uid := range map[string]string{"cop": "76561190000000951", "civ": "76561190000000952", "gone": "76561190000000953", "old": "76561190000000954"} {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name, civ_cash, cop_cash, cop_level) VALUES ($1, $2, 100, 500, 3) RETURNING id`, uid, "Online "+name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[name] = id
	}
	all := []int64{ids["cop"], ids["civ"], ids["gone"], ids["old"]}
	var gid int64
	if err := pool.QueryRow(ctx, `INSERT INTO gangs (name, tag, leader_player_id) VALUES ('Online Test Crew', 'OTC', $1) RETURNING id`, ids["civ"]).Scan(&gid); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM gangs WHERE id = $1`, gid)
		pool.Exec(ctx, `DELETE FROM players WHERE id = ANY($1)`, all)
	})
	must := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatal(q, err)
		}
	}
	must(`INSERT INTO gang_members (gang_id, player_id, rank) VALUES ($1, $2, 'leader')`, gid, ids["civ"])
	must(`INSERT INTO bank_accounts (player_id, faction, balance) VALUES ($1, 'police', 2000), ($1, 'civilian', 9999)`, ids["cop"])
	must(`INSERT INTO player_sessions (player_id, faction, connected_at) VALUES ($1, 'cop', now() - interval '3 hours')`, ids["cop"])
	must(`INSERT INTO player_sessions (player_id, faction, connected_at) VALUES ($1, 'civ', now() - interval '10 minutes')`, ids["civ"])
	must(`INSERT INTO player_sessions (player_id, faction, connected_at, disconnected_at) VALUES ($1, 'medic', now() - interval '90 minutes', now() - interval '30 minutes')`, ids["gone"])
	must(`INSERT INTO player_sessions (player_id, faction, connected_at, disconnected_at) VALUES ($1, 'civ', now() - interval '110 minutes', now() - interval '100 minutes')`, ids["gone"]) // an earlier one: listed once
	must(`INSERT INTO player_sessions (player_id, connected_at, disconnected_at) VALUES ($1, now() - interval '20 hours', now() - interval '19 hours')`, ids["old"])
	must(`INSERT INTO wanted_crimes (player_id, crime_key, bounty_amount) VALUES ($1, 'evidence_robbery', 1500)`, ids["civ"])
	must(`INSERT INTO anti_cheat_flags (player_id, flag_type, confidence) VALUES ($1, 'movement', 'high')`, ids["civ"])

	players, err := Now(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]Player{}
	for _, p := range players {
		got[p.PlayerID] = p
	}
	cop, civ := got[ids["cop"]], got[ids["civ"]]
	if cop.Faction != "cop" || cop.Cash != 500 || cop.Bank != 2000 || cop.Level != 3 || cop.For(time.Now()) < 2*time.Hour {
		t.Errorf("cop: %+v", cop)
	}
	if civ.Faction != "civ" || civ.Cash != 100 || civ.Bounty != 1500 || civ.OpenFlags != 1 || civ.Gang != "Online Test Crew" || civ.Level != 0 {
		t.Errorf("civ: %+v", civ)
	}
	if _, ok := got[ids["gone"]]; ok {
		t.Error("a player who left is listed as online")
	}

	left, err := RecentlyLeft(ctx, pool, 2*time.Hour, 50)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, l := range left {
		if l.PlayerID == ids["gone"] {
			if found {
				t.Error("a player is listed twice in recently left")
			}
			found = true
			if l.Faction != "medic" || l.Length < 59*time.Minute || l.Length > 61*time.Minute {
				t.Errorf("left: %+v", l)
			}
		}
		if l.PlayerID == ids["old"] {
			t.Error("a session that ended 19 h ago is in the last 2 h")
		}
	}
	if !found {
		t.Error("recently left misses the player who left")
	}

	times, counts, err := History(ctx, pool, 24*time.Hour, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(times) != 97 || len(counts) != 97 {
		t.Fatalf("history: %d points", len(times))
	}
	if counts[len(counts)-1] < 2 {
		t.Errorf("history now: %v, want at least 2", counts[len(counts)-1])
	}
	// 19.5 h ago only "old" (of ours) was on: index 18 is 19.5 h before the last point.
	if counts[18] < 1 {
		t.Errorf("history 19.5 h ago: %v", counts[18])
	}

	sum, err := Summarize(ctx, pool, players, times, counts)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Online != len(players) || sum.Cop < 1 || sum.Civ < 1 || sum.Peak24 < 2 || sum.Unique24 < 3 || sum.AvgSession <= 0 {
		t.Errorf("summary: %+v", sum)
	}
}
