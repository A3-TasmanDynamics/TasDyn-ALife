package auth

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runs only with TEST_DATABASE_URL. Test players are INSERTed with their
// rank already set (the change-capture trigger only fires on UPDATE), so
// no staff_log rows are produced for a live #staff-log poster to send.
func TestCan(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close) // registered first so it runs last, after the other cleanups

	mk := func(uid, rankKey, status string) int64 {
		var id int64
		err := pool.QueryRow(ctx, `
			INSERT INTO players (uid, staff_rank_id, staff_status)
			VALUES ($1, (SELECT id FROM staff_ranks WHERE key = NULLIF($2, '')), $3)
			RETURNING id`, uid, rankKey, status).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM players WHERE id = $1`, id) })
		return id
	}
	mod := mk("76561190000000001", "moderator", "active")
	admin := mk("76561190000000002", "admin", "active")
	suspendedAdmin := mk("76561190000000003", "admin", "suspended")
	civilian := mk("76561190000000004", "", "active")

	grant := func(player int64, key string, allow bool) {
		if _, err := pool.Exec(ctx, `INSERT INTO staff_permission_overrides (player_id, command_key, allow) VALUES ($1, $2, $3)`, player, key, allow); err != nil {
			t.Fatal(err)
		}
	}
	grant(mod, "bans.permanent", true)  // normal key: override grants it
	grant(mod, "roles.manage", true)    // NoOverride key: override ignored
	grant(admin, "bans.issue", false)   // explicit revoke beats level

	cases := []struct {
		name   string
		player int64
		key    string
		want   Denial
	}{
		{"mod meets level", mod, "bans.issue", Allowed},
		{"mod below level", mod, "bans.revoke", DenyLevel},
		{"override grants", mod, "bans.permanent", Allowed},
		{"NoOverride ignores grant", mod, "roles.manage", DenyLevel},
		{"override revokes", admin, "bans.issue", DenyLevel},
		{"admin by level", admin, "bans.revoke", Allowed},
		{"suspended denied", suspendedAdmin, "players.view", DenyInactive},
		{"civilian", civilian, "players.view", DenyNotStaff},
		{"no such player", -1, "players.view", DenyNotStaff},
	}
	for _, c := range cases {
		got, err := Can(ctx, pool, c.player, c.key)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}

	if _, err := Can(ctx, pool, admin, "no.such.key"); err != ErrUnknownPermission {
		t.Errorf("unknown key: got %v", err)
	}
}
