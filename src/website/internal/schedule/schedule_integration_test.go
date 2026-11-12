package schedule

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSchedule(t *testing.T) {
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
	var staff int64
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000841', 'Sched Staff') RETURNING id`).Scan(&staff)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM scheduled_messages WHERE created_by = $1`, staff)
		pool.Exec(ctx, `DELETE FROM game_commands WHERE issued_by = $1`, staff)
		pool.Exec(ctx, `DELETE FROM staff_log WHERE staff_player_id = $1`, staff)
		pool.Exec(ctx, `DELETE FROM players WHERE id = $1`, staff)
	})
	now := time.Now()
	denied := func(what string, err error) {
		t.Helper()
		if !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%s: want not allowed, got %v", what, err)
		}
	}
	_, err = Add(ctx, pool, staff, " ", now, 0, nil)
	denied("empty", err)
	_, err = Add(ctx, pool, staff, "x", now.Add(-time.Hour), 0, nil)
	denied("in the past", err)
	_, err = Add(ctx, pool, staff, "x", now, 2, nil)
	denied("repeat too often", err)
	past := now.Add(-time.Minute)
	_, err = Add(ctx, pool, staff, "x", now.Add(time.Hour), 60, &past)
	denied("ends before it starts", err)

	once, _ := Add(ctx, pool, staff, "Sched once", now.Add(time.Minute), 0, nil)
	every, _ := Add(ctx, pool, staff, "Sched every 30", now.Add(time.Minute), 30, nil)
	until := now.Add(40 * time.Minute)
	ending, _ := Add(ctx, pool, staff, "Sched ends", now.Add(time.Minute), 60, &until)
	stale, _ := Add(ctx, pool, staff, "Sched stale", now.Add(time.Minute), 0, nil)
	pool.Exec(ctx, `UPDATE scheduled_messages SET next_at = now() - interval '20 minutes' WHERE id = $1`, stale)

	// Two minutes on: the first three are due; the stale one is skipped.
	sent, err := SendDue(ctx, pool, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if sent < 3 {
		t.Errorf("sent %d, want at least 3", sent)
	}
	var queued int
	pool.QueryRow(ctx, `SELECT count(*) FROM game_commands WHERE issued_by = $1 AND action = 'announce' AND text LIKE 'Sched %'`, staff).Scan(&queued)
	if queued != 3 {
		t.Errorf("queued %d announcements, want 3 (stale skipped)", queued)
	}
	state := func(id int64) (bool, time.Time, int) {
		var a bool
		var next time.Time
		var n int
		pool.QueryRow(ctx, `SELECT active, next_at, sent_count FROM scheduled_messages WHERE id = $1`, id).Scan(&a, &next, &n)
		return a, next, n
	}
	if a, _, n := state(once); a || n != 1 {
		t.Errorf("once: active %v sent %d", a, n)
	}
	if a, next, n := state(every); !a || n != 1 || next.Before(now.Add(30*time.Minute)) {
		t.Errorf("repeating: active %v next %v sent %d", a, next, n)
	}
	if a, _, _ := state(ending); a {
		t.Error("the next repeat is after its end, so it should be finished")
	}
	if a, _, n := state(stale); a || n != 0 {
		t.Errorf("stale: active %v sent %d", a, n)
	}
	// Nothing new due: nothing sent.
	if again, _ := SendDue(ctx, pool, now.Add(3*time.Minute)); again != 0 {
		t.Errorf("sent again: %d", again)
	}
	// Pause / resume.
	if err := SetActive(ctx, pool, every, false); err != nil {
		t.Fatal(err)
	}
	if err := SetActive(ctx, pool, every, true); err != nil {
		t.Fatal(err)
	}
	denied("resume a finished one-off", SetActive(ctx, pool, once, true))

	// Restart countdown 12 minutes out: the 10, 5 and 1 minute warnings (15 has passed).
	n, err := QueueCountdown(ctx, pool, staff, now.Add(12*time.Minute), "update")
	if err != nil || n != 3 {
		t.Fatalf("countdown: %d %v", n, err)
	}
	n, _ = QueueCountdown(ctx, pool, staff, now.Add(30*time.Minute), "")
	var restart int
	pool.QueryRow(ctx, `SELECT count(*) FROM scheduled_messages WHERE kind = 'restart' AND active`).Scan(&restart)
	if n != 4 || restart != 4 {
		t.Errorf("a new countdown should replace the old: queued %d, active %d", n, restart)
	}
	if c, _ := CancelCountdown(ctx, pool); c != 4 {
		t.Errorf("cancelled %d", c)
	}
	if err := Delete(ctx, pool, staff, every); err != nil {
		t.Fatal(err)
	}
	var logs int
	pool.QueryRow(ctx, `SELECT count(*) FROM staff_log WHERE staff_player_id = $1 AND action IN ('schedule.add', 'schedule.remove')`, staff).Scan(&logs)
	if logs != 5 {
		t.Errorf("staff log: %d, want 4 adds + 1 remove", logs)
	}
}
