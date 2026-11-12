package attention

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCollect(t *testing.T) {
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
	var a, b int64
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000921', 'Attn Alpha') RETURNING id`).Scan(&a)
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000922', 'Attn Bravo') RETURNING id`).Scan(&b)
	var quiet, fresh, appeal int64
	pool.QueryRow(ctx, `INSERT INTO staff_cases (case_type, summary, created_at) VALUES ('exploit', 'Attn quiet case', now() - interval '5 days') RETURNING id`).Scan(&quiet)
	pool.QueryRow(ctx, `INSERT INTO staff_cases (case_type, summary, created_at) VALUES ('exploit', 'Attn busy case', now() - interval '5 days') RETURNING id`).Scan(&fresh)
	pool.QueryRow(ctx, `INSERT INTO staff_cases (case_type, summary) VALUES ('ban_appeal', 'Attn appeal') RETURNING id`).Scan(&appeal)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM staff_cases WHERE id IN ($1, $2, $3)`, quiet, fresh, appeal)
		pool.Exec(ctx, `DELETE FROM players WHERE id IN ($1, $2)`, a, b)
	})
	if _, err := pool.Exec(ctx, `INSERT INTO staff_case_entries (case_id, author_id, kind, body) VALUES ($1, $2, 'note', 'still on it')`, fresh, a); err != nil {
		t.Fatal(err)
	}
	pool.Exec(ctx, `INSERT INTO anti_cheat_flags (player_id, flag_type, confidence, created_at) VALUES ($1, 'dupe', 'high', now() - interval '30 hours'), ($1, 'gear', 'medium', now())`, a)
	pool.Exec(ctx, `INSERT INTO player_watchlist (player_id, reason) VALUES ($1, 'test'), ($2, 'test')`, a, b)
	pool.Exec(ctx, `INSERT INTO player_sessions (player_id) VALUES ($1)`, a) // a online, b not

	all := map[string]bool{"anticheat.review": true, "cases.view": true, "players.view": true, "applications.view": true}
	items, err := Collect(ctx, pool, Viewer{Perms: all, Support: true}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, it := range items {
		lines = append(lines, it.Level+" | "+it.Title+" | "+it.Detail+" | "+it.URL)
	}
	got := strings.Join(lines, "\n")
	t.Log("\n" + got)
	// The test DB is shared, so counts can include other tests' rows: check
	// for ours by what's distinctive.
	want := []string{
		"critical | ", "high-confidence anti-cheat flag", // 30 h old: urgent
		"medium-confidence flag",
		"ban appeal",
		"Attn Alpha",
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q", w)
		}
	}
	if !strings.Contains(got, "#"+strconv.FormatInt(quiet, 10)) && !strings.Contains(got, "Attn quiet case") {
		t.Errorf("the quiet case isn't listed")
	}
	if strings.Contains(got, "#"+strconv.FormatInt(fresh, 10)+" ") || strings.Contains(got, "Attn busy case") || strings.Contains(got, "Attn Bravo") {
		t.Error("a case with a recent entry, or an offline watched player, is listed")
	}
	// Most urgent first.
	for i := 1; i < len(items); i++ {
		if rank(items[i].Level) < rank(items[i-1].Level) {
			t.Errorf("out of order at %d: %s after %s", i, items[i].Level, items[i-1].Level)
		}
	}
	// Without permissions: nothing.
	none, err := Collect(ctx, pool, Viewer{Perms: map[string]bool{}}, time.Now())
	if err != nil || len(none) != 0 {
		t.Errorf("no permissions: %v %v", none, err)
	}
}

func rank(l string) int { return map[string]int{Critical: 0, Warning: 1, Info: 2}[l] }
