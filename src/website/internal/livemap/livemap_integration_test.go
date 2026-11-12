package livemap

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPoints(t *testing.T) {
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
	var cop, civ, fresh, off int64
	ins := func(uid, name, civPos, copPos string) int64 {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name, civ_position, cop_position) VALUES ($1, $2, $3::jsonb, $4::jsonb) RETURNING id`, uid, name, civPos, copPos).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	cop = ins("76561190000000831", "Map Cop", `[["x",100],["y",100],["z",0]]`, `[["x",3300],["y",12960],["z",0]]`)
	civ = ins("76561190000000832", "Map Civ", `[["x",16830],["y",12650],["z",1]]`, `[]`)
	fresh = ins("76561190000000833", "Map Fresh", `[]`, `[]`)
	off = ins("76561190000000834", "Map Offline", `[["x",5000],["y",5000],["z",0]]`, `[]`)
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM players WHERE id IN ($1, $2, $3, $4)`, cop, civ, fresh, off) })
	pool.Exec(ctx, `INSERT INTO player_sessions (player_id, faction) VALUES ($1, 'cop'), ($2, 'civ'), ($3, 'civ')`, cop, civ, fresh)
	pool.Exec(ctx, `INSERT INTO player_watchlist (player_id, reason) VALUES ($1, 'map test')`, civ)
	pool.Exec(ctx, `INSERT INTO wanted_crimes (player_id, crime_key, bounty_amount) VALUES ($1, 'robbery', 100)`, civ)

	pts, online, err := Points(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]Point{}
	for _, p := range pts {
		got[p.PlayerID] = p
	}
	if p := got[cop]; p.Faction != "cop" || p.X != 3300 || p.Y != 12960 {
		t.Errorf("cop should be at their police position: %+v", p)
	}
	if p := got[civ]; !p.Watched || !p.Wanted || p.X != 16830 {
		t.Errorf("civ: %+v", p)
	}
	if _, ok := got[fresh]; ok {
		t.Error("a player with no saved position is on the map")
	}
	if _, ok := got[off]; ok {
		t.Error("an offline player is on the map")
	}
	if online < 3 || online < len(pts) {
		t.Errorf("online %d, points %d", online, len(pts))
	}
}
