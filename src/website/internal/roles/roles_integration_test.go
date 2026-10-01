package roles

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runs only with TEST_DATABASE_URL, and must not run while a website with a
// live #staff-log webhook uses the same database (it commits staff_log
// rows, which it deletes afterwards). It only edits ranks it creates.
func TestRoleRules(t *testing.T) {
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

	var players []int64
	var ranks []int
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM staff_log WHERE staff_player_id = ANY($1)`, players)
		pool.Exec(ctx, `DELETE FROM players WHERE id = ANY($1)`, players)
		pool.Exec(ctx, `DELETE FROM staff_ranks WHERE id = ANY($1)`, ranks)
		pool.Exec(ctx, `DELETE FROM faction_rank_names WHERE faction = 'ems' AND name LIKE 'Test %'`)
	})
	mk := func(uid, rankKey string) int64 {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO players (uid, staff_rank_id) VALUES ($1, (SELECT id FROM staff_ranks WHERE key = $2)) RETURNING id`,
			uid, rankKey).Scan(&id); err != nil {
			t.Fatal(err)
		}
		players = append(players, id)
		return id
	}
	head := mk("76561190000000031", "head_admin")
	admin := mk("76561190000000032", "admin")

	denied := func(name string, err error) {
		t.Helper()
		if !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%s: expected ErrNotAllowed, got %v", name, err)
		}
	}
	ok := func(name string, err error) {
		t.Helper()
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	_, err = Create(ctx, pool, admin, "Test Too High", 45)
	denied("admin creates a rank above own level", err)
	a, err := Create(ctx, pool, head, "Test A", 13)
	ok("create A", err)
	b, err := Create(ctx, pool, head, "Test B", 12)
	ok("create B", err)
	ranks = append(ranks, a, b)
	_, err = Create(ctx, pool, head, "Test Clash", 13)
	denied("level clash", err)

	ok("admin grants a key it holds", Update(ctx, pool, admin, a, "Test A", false, true, []string{"cases.view", "players.compensate"}, 0))
	denied("admin grants a key it lacks", Update(ctx, pool, admin, a, "Test A", false, true, []string{"cases.view", "roles.manage"}, 0))
	denied("unknown key", Update(ctx, pool, head, a, "Test A", false, true, []string{"no.such.key"}, 0))
	denied("nothing changed", Update(ctx, pool, admin, a, "Test A", false, true, []string{"cases.view", "players.compensate"}, 0))
	var adminRank int
	pool.QueryRow(ctx, `SELECT id FROM staff_ranks WHERE key = 'admin'`).Scan(&adminRank)
	denied("admin edits own rank", Update(ctx, pool, admin, adminRank, "Admin", true, true, nil, 0))

	// The top rank can edit its own level-100 rank, but not lock itself out
	// or change its level.
	var headRank int
	var headName string
	pool.QueryRow(ctx, `SELECT id, display_name FROM staff_ranks WHERE key = 'head_admin'`).Scan(&headRank, &headName)
	var headPerms []string
	pool.QueryRow(ctx, `SELECT array_agg(command_key) FROM rank_permissions WHERE rank_id = $1`, headRank).Scan(&headPerms)
	ok("top rank renames itself", Update(ctx, pool, head, headRank, "Test Top", true, true, headPerms, 0))
	var without []string
	for _, k := range headPerms {
		if k != "roles.manage" {
			without = append(without, k)
		}
	}
	denied("top rank drops roles.manage from itself", Update(ctx, pool, head, headRank, "Test Top", true, true, without, 0))
	denied("top rank drops its admin panel", Update(ctx, pool, head, headRank, "Test Top", false, true, headPerms, 0))
	denied("top rank changes its own level", Update(ctx, pool, head, headRank, "Test Top", true, true, headPerms, 90))
	ok("restore top rank name", Update(ctx, pool, head, headRank, headName, true, true, headPerms, 0))

	// Levels can be set directly, below your own and unused.
	ok("set A's level directly", Update(ctx, pool, head, a, "Test A", false, true, []string{"cases.view", "players.compensate"}, 14))
	denied("level clash on edit", Update(ctx, pool, head, a, "Test A", false, true, []string{"cases.view", "players.compensate"}, 12))
	denied("level at or above your own", Update(ctx, pool, admin, a, "Test A", false, true, []string{"cases.view", "players.compensate"}, 40))
	ok("set A back to 13", Update(ctx, pool, head, a, "Test A", false, true, []string{"cases.view", "players.compensate"}, 13))

	ok("move B up", Move(ctx, pool, head, b, true))
	var la, lb int
	pool.QueryRow(ctx, `SELECT (SELECT level FROM staff_ranks WHERE id = $1), (SELECT level FROM staff_ranks WHERE id = $2)`, a, b).Scan(&la, &lb)
	if la != 12 || lb != 13 {
		t.Errorf("after move: A=%d B=%d, want 12 and 13", la, lb)
	}

	// A rank with a member can't be deleted; an empty one can.
	var member int64
	pool.QueryRow(ctx, `INSERT INTO players (uid, staff_rank_id) VALUES ('76561190000000033', $1) RETURNING id`, a).Scan(&member)
	players = append(players, member)
	denied("delete rank with members", Delete(ctx, pool, head, a))
	ok("delete empty rank", Delete(ctx, pool, head, b))

	denied("blank level in the middle", SaveFactionNames(ctx, pool, head, "ems", []string{"Test One", "", "Test Three"}))
	ok("faction names", SaveFactionNames(ctx, pool, head, "ems", []string{"Test One", "Test Two", ""}))
	names, _ := FactionNames(ctx, pool)
	if len(names["ems"]) != 2 || names["ems"][1] != "Test Two" {
		t.Errorf("ems names: %v", names["ems"])
	}
}
