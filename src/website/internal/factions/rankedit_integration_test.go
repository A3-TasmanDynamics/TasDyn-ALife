package factions

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestAddDeleteRank: cabinet and Management add and delete ranks; members,
// "promotes up to", division minimums and role mappings follow their rank.
func TestAddDeleteRank(t *testing.T) {
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
	var n int
	pool.QueryRow(ctx, `SELECT count(*) FROM faction_rank_names WHERE faction = 'police'`).Scan(&n)
	if n > 0 {
		t.Skip("police ranks already configured on this database")
	}
	// 1 Cadet, 2 Constable, 3 Sergeant (command, up to 2), 4 Commissioner (cabinet, up to 3).
	if _, err := pool.Exec(ctx, `
		INSERT INTO faction_rank_names (faction, level, name, promote_up_to, is_command, is_cabinet) VALUES
		('police', 1, 'Cadet', 0, false, false), ('police', 2, 'Constable', 0, false, false),
		('police', 3, 'Sergeant', 2, true, false), ('police', 4, 'Commissioner', 3, true, true)`); err != nil {
		t.Fatal(err)
	}
	pool.Exec(ctx, `INSERT INTO faction_divisions (faction, key, name, roles, min_level) VALUES ('police', 'TRK', 'Test Rank', '{Lead}', 3)`)
	pool.Exec(ctx, `INSERT INTO platform_group_map (platform, entitlement, group_id) VALUES ('discord', 'faction_rank:police:2', 'r2'), ('discord', 'faction_rank:police:3', 'r3'), ('discord', 'faction_rank:police:4', 'r4')`)
	ids := map[string]int64{}
	for name, lvl := range map[string]int{"comm": 4, "sgt": 3, "con": 2} {
		uid := map[string]string{"comm": "76561190000000281", "sgt": "76561190000000282", "con": "76561190000000283"}[name]
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name, cop_level) VALUES ($1, $2, $3) RETURNING id`, uid, "t_"+name, lvl).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[name] = id
	}
	t.Cleanup(func() {
		all := []int64{ids["comm"], ids["sgt"], ids["con"]}
		pool.Exec(ctx, `DELETE FROM faction_log WHERE target_id = ANY($1) OR actor_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM players WHERE id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM faction_divisions WHERE faction = 'police' AND key = 'TRK'`)
		pool.Exec(ctx, `DELETE FROM platform_group_map WHERE entitlement LIKE 'faction_rank:police:%'`)
		pool.Exec(ctx, `DELETE FROM faction_rank_names WHERE faction = 'police'`)
	})
	comm := Actor{PlayerID: ids["comm"], Source: "website", Via: ViaCommand}
	sgt := Actor{PlayerID: ids["sgt"], Source: "website", Via: ViaCommand}
	mgmt := Actor{PlayerID: ids["con"], Source: "website", Via: ViaStaffOverride}
	level := func(name string) int {
		var l int
		pool.QueryRow(ctx, `SELECT cop_level FROM players WHERE id = $1`, ids[name]).Scan(&l)
		return l
	}
	names := func() []string {
		rs, _ := Ranks(ctx, pool, "police")
		out := []string{}
		for _, r := range rs {
			out = append(out, r.Name)
		}
		return out
	}
	refused := func(err error, what string) {
		t.Helper()
		if !errors.Is(err, ErrNotAllowed) {
			t.Fatalf("%s: want refused, got %v", what, err)
		}
	}

	// Command that isn't cabinet can't; cabinet can't add above itself.
	refused(AddRank(ctx, pool, sgt, "police", 2, "Senior Constable", "SC"), "sergeant adds")
	refused(AddRank(ctx, pool, comm, "police", 5, "Chief", ""), "commissioner adds above own rank")

	// Commissioner adds Senior Constable at 3: Sergeant 3->4, Commissioner 4->5.
	if err := AddRank(ctx, pool, comm, "police", 3, "Senior Constable", "SC"); err != nil {
		t.Fatal(err)
	}
	if got := names(); len(got) != 5 || got[2] != "Senior Constable" || got[3] != "Sergeant" || got[4] != "Commissioner" {
		t.Fatalf("ranks after add: %v", got)
	}
	if level("sgt") != 4 || level("comm") != 5 || level("con") != 2 {
		t.Fatalf("members after add: sgt %d comm %d con %d", level("sgt"), level("comm"), level("con"))
	}
	rs, _ := Ranks(ctx, pool, "police")
	if RankFor(rs, 4).PromoteUpTo != 2 || RankFor(rs, 5).PromoteUpTo != 4 {
		t.Fatalf("promotes up to after add: sergeant %d commissioner %d", RankFor(rs, 4).PromoteUpTo, RankFor(rs, 5).PromoteUpTo)
	}
	var minLevel int
	pool.QueryRow(ctx, `SELECT min_level FROM faction_divisions WHERE faction = 'police' AND key = 'TRK'`).Scan(&minLevel)
	if minLevel != 4 {
		t.Fatalf("division minimum after add: %d", minLevel)
	}
	var group string
	pool.QueryRow(ctx, `SELECT group_id FROM platform_group_map WHERE entitlement = 'faction_rank:police:4'`).Scan(&group)
	if group != "r3" {
		t.Fatalf("Sergeant's role should follow it to level 4, got %q", group)
	}

	// Deleting: not a held rank, not your own; then Management deletes Constable (empty after moving con).
	refused(DeleteRank(ctx, pool, comm, "police", 2), "delete a held rank")
	refused(DeleteRank(ctx, pool, comm, "police", 5), "delete own rank")
	if err := DeleteRank(ctx, pool, comm, "police", 3); err != nil { // Senior Constable, empty
		t.Fatal(err)
	}
	if got := names(); len(got) != 4 || got[2] != "Sergeant" {
		t.Fatalf("ranks after delete: %v", got)
	}
	if level("sgt") != 3 || level("comm") != 4 {
		t.Fatalf("members after delete: sgt %d comm %d", level("sgt"), level("comm"))
	}
	pool.QueryRow(ctx, `SELECT group_id FROM platform_group_map WHERE entitlement = 'faction_rank:police:3'`).Scan(&group)
	if group != "r3" {
		t.Fatalf("Sergeant's role should be back at level 3, got %q", group)
	}
	pool.Exec(ctx, `UPDATE players SET cop_level = 1 WHERE id = $1`, ids["con"])
	if err := DeleteRank(ctx, pool, mgmt, "police", 2); err != nil {
		t.Fatal(err)
	}
	if got := names(); len(got) != 3 || got[1] != "Sergeant" || level("sgt") != 2 || level("comm") != 3 {
		t.Fatalf("after Management delete: %v sgt %d comm %d", got, level("sgt"), level("comm"))
	}
	rs, _ = Ranks(ctx, pool, "police")
	if RankFor(rs, 2).PromoteUpTo != 1 || RankFor(rs, 3).PromoteUpTo != 2 {
		t.Fatalf("promotes up to after delete: sergeant %d commissioner %d", RankFor(rs, 2).PromoteUpTo, RankFor(rs, 3).PromoteUpTo)
	}
	var gone int
	pool.QueryRow(ctx, `SELECT count(*) FROM platform_group_map WHERE group_id = 'r2'`).Scan(&gone)
	if gone != 0 {
		t.Fatal("the deleted rank's role mapping should be gone")
	}
}
