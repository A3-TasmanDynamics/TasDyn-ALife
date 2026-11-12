package gamecmd

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIssue(t *testing.T) {
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
	var staff, on, off int64
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000871', 'Cmd Staff') RETURNING id`).Scan(&staff)
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000872', 'Cmd Online') RETURNING id`).Scan(&on)
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000873', 'Cmd Offline') RETURNING id`).Scan(&off)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM game_commands WHERE issued_by = $1`, staff)
		pool.Exec(ctx, `DELETE FROM staff_log WHERE staff_player_id = $1`, staff)
		pool.Exec(ctx, `DELETE FROM players WHERE id IN ($1, $2, $3)`, staff, on, off)
	})
	pool.Exec(ctx, `INSERT INTO player_sessions (player_id) VALUES ($1)`, on)

	denied := func(what string, err error) {
		t.Helper()
		if !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%s: want not allowed, got %v", what, err)
		}
	}
	_, err = Issue(ctx, pool, staff, "message", off, "hello")
	denied("offline player", err)
	_, err = Issue(ctx, pool, staff, "message", on, "   ")
	denied("empty text", err)
	_, err = Issue(ctx, pool, staff, "teleport", on, "x")
	denied("unknown action", err)
	_, err = Issue(ctx, pool, staff, "kick", staff, "x")
	denied("kick yourself", err)
	long := make([]rune, 301)
	for i := range long {
		long[i] = 'a'
	}
	_, err = Issue(ctx, pool, staff, "warn", on, string(long))
	denied("too long", err)

	msg, err := Issue(ctx, pool, staff, "message", on, "  please   come to the PD  ")
	if err != nil {
		t.Fatal(err)
	}
	kick, err := Issue(ctx, pool, staff, "kick", on, "Combat logging")
	if err != nil {
		t.Fatal(err)
	}
	ann, err := Issue(ctx, pool, staff, "announce", on, "Restart in 10 minutes") // target ignored
	if err != nil {
		t.Fatal(err)
	}
	var text string
	var target *int64
	pool.QueryRow(ctx, `SELECT text FROM game_commands WHERE id = $1`, msg).Scan(&text)
	if text != "please come to the PD" {
		t.Errorf("text tidied: %q", text)
	}
	pool.QueryRow(ctx, `SELECT target_player_id FROM game_commands WHERE id = $1`, ann).Scan(&target)
	if target != nil {
		t.Error("an announcement has a target")
	}
	var logs, kicks int
	pool.QueryRow(ctx, `SELECT count(*) FROM staff_log WHERE staff_player_id = $1 AND action IN ('game.message', 'player.kick', 'game.announce')`, staff).Scan(&logs)
	pool.QueryRow(ctx, `SELECT count(*) FROM kick_log WHERE kicked_by = $1 AND target_player_id = $2`, staff, on).Scan(&kicks)
	if logs != 3 || kicks != 1 {
		t.Errorf("staff log %d (want 3), kick log %d (want 1)", logs, kicks)
	}

	// The game server's side: an old pending one shows as expired; one it ran shows its outcome.
	pool.Exec(ctx, `UPDATE game_commands SET created_at = now() - interval '5 minutes' WHERE id = $1`, msg)
	pool.Exec(ctx, `UPDATE game_commands SET status = 'done', result = 'kicked', done_at = now() WHERE id = $1`, kick)
	rec, err := Recent(ctx, pool, 50)
	if err != nil {
		t.Fatal(err)
	}
	status := map[int64]string{}
	for _, c := range rec {
		status[c.ID] = c.Status
	}
	if status[msg] != "expired" || status[kick] != "done" || status[ann] != "pending" {
		t.Errorf("statuses: %v", status)
	}
	if c, ok, err := Get(ctx, pool, kick); err != nil || !ok || c.Result != "kicked" {
		t.Errorf("get: %+v %v %v", c, ok, err)
	}
}
