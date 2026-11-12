package watchlist

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestWatchlist(t *testing.T) {
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
	var staff, target, other int64
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000961', 'Watch Staff') RETURNING id`).Scan(&staff)
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000962', 'Watch Target') RETURNING id`).Scan(&target)
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000963', 'Watch Other') RETURNING id`).Scan(&other)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM staff_log WHERE target_player_id IN ($1, $2)`, target, other)
		pool.Exec(ctx, `DELETE FROM players WHERE id IN ($1, $2, $3)`, staff, target, other)
	})
	actor := Actor{PlayerID: staff, Source: "website"}

	// Finding players.
	for q, want := range map[string]int64{"76561190000000962": target, "#" + strconv.FormatInt(target, 10): target, "watch target": target} {
		if id, err := FindPlayer(ctx, pool, q); err != nil || id != want {
			t.Errorf("FindPlayer(%q) = %d, %v", q, id, err)
		}
	}
	if _, err := FindPlayer(ctx, pool, "nobody at all 123"); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("unknown player: %v", err)
	}

	// Rules.
	if err := Add(ctx, pool, actor, target, "  "); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("empty reason: %v", err)
	}
	if err := Add(ctx, pool, actor, staff, "me"); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("self: %v", err)
	}
	if err := Add(ctx, pool, actor, -5, "ghost"); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("missing player: %v", err)
	}

	// Add, edit, the same again (no change), list, remove.
	if err := Add(ctx, pool, actor, target, "Suspected duping"); err != nil {
		t.Fatal(err)
	}
	if err := Add(ctx, pool, actor, target, "Suspected duping, case #4"); err != nil {
		t.Fatal(err)
	}
	if err := Add(ctx, pool, actor, target, "Suspected duping, case #4"); err != nil {
		t.Fatal(err)
	}
	e, ok, err := Get(ctx, pool, target)
	if err != nil || !ok || e.Reason != "Suspected duping, case #4" || e.AddedBy != "Watch Staff" {
		t.Errorf("get: %+v %v %v", e, ok, err)
	}
	if _, ok, _ := Get(ctx, pool, other); ok {
		t.Error("other is on the watchlist")
	}
	pool.Exec(ctx, `INSERT INTO player_sessions (player_id) VALUES ($1)`, target)
	pool.Exec(ctx, `INSERT INTO wanted_crimes (player_id, crime_key, bounty_amount) VALUES ($1, 'robbery', 700)`, target)
	pool.Exec(ctx, `INSERT INTO player_log (player_id, event_type) VALUES ($1, 'robbery.gas')`, target)
	list, err := List(ctx, pool, "case #4")
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %+v %v", list, err)
	}
	if l := list[0]; l.PlayerID != target || !l.Online || l.Bounty != 700 || l.Events7d < 1 {
		t.Errorf("entry: %+v", l)
	}
	reasons, _ := Reasons(ctx, pool)
	if reasons[target] == "" {
		t.Error("reasons misses the target")
	}
	if err := Remove(ctx, pool, actor, target, "cleared"); err != nil {
		t.Fatal(err)
	}
	if err := Remove(ctx, pool, actor, target, ""); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("remove twice: %v", err)
	}
	var actions []string
	rows, _ := pool.Query(ctx, `SELECT action FROM staff_log WHERE target_player_id = $1 ORDER BY id`, target)
	for rows.Next() {
		var a string
		rows.Scan(&a)
		actions = append(actions, a)
	}
	rows.Close()
	if len(actions) != 3 || actions[0] != "watchlist.add" || actions[1] != "watchlist.edit" || actions[2] != "watchlist.remove" {
		t.Errorf("staff log: %v", actions)
	}

}
