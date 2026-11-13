package factions

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLoadActivity(t *testing.T) {
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
	var busy, quiet, never, outsider int64
	for _, p := range []struct {
		uid, name string
		cop       int
		id        *int64
	}{
		{"76561190000001011", "Duty Busy", 3, &busy},
		{"76561190000001012", "Duty Quiet", 2, &quiet},
		{"76561190000001013", "Duty Never", 1, &never},
		{"76561190000001014", "Duty Outsider", 0, &outsider},
	} {
		if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name, cop_level) VALUES ($1, $2, $3) RETURNING id`, p.uid, p.name, p.cop).Scan(p.id); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM players WHERE id IN ($1, $2, $3, $4)`, busy, quiet, never, outsider)
	})

	// Busy: 2 h as cop two days ago (local noon), plus 1 h as civ (not
	// duty). Quiet: last cop session 20 days ago. Outsider: cop session but
	// not on the roster -- must not count.
	if _, err := pool.Exec(ctx, `
		INSERT INTO player_sessions (player_id, connected_at, disconnected_at, faction) VALUES
		($1, ((now() AT TIME ZONE $4)::date - 2 + time '12:00') AT TIME ZONE $4, ((now() AT TIME ZONE $4)::date - 2 + time '14:00') AT TIME ZONE $4, 'cop'),
		($1, ((now() AT TIME ZONE $4)::date - 2 + time '15:00') AT TIME ZONE $4, ((now() AT TIME ZONE $4)::date - 2 + time '16:00') AT TIME ZONE $4, 'civ'),
		($2, now() - interval '20 days', now() - interval '20 days' + interval '1 hour', 'cop'),
		($3, ((now() AT TIME ZONE $4)::date - 2 + time '12:00') AT TIME ZONE $4, ((now() AT TIME ZONE $4)::date - 2 + time '18:00') AT TIME ZONE $4, 'cop')`,
		busy, quiet, outsider, activityRegion); err != nil {
		t.Fatal(err)
	}

	members := []Member{{ID: busy, Name: "Duty Busy"}, {ID: quiet, Name: "Duty Quiet"}, {ID: never, Name: "Duty Never"}}
	a, err := LoadActivity(ctx, pool, "police", members)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Days) != 14 {
		t.Fatalf("want 14 days, got %d", len(a.Days))
	}
	if a.Days[11].Minutes != 120 {
		t.Errorf("two days ago: %d min on duty, want 120 (civ time and non-members excluded)", a.Days[11].Minutes)
	}
	for i, d := range a.Days {
		if i != 11 && d.Minutes != 0 {
			t.Errorf("day %d: %d min with no duty", i, d.Minutes)
		}
	}
	if a.ActiveWeek != 1 || a.WeekMins != 120 {
		t.Errorf("week: %d active, %d min", a.ActiveWeek, a.WeekMins)
	}
	if a.QuietTotal != 2 || len(a.Quiet) != 2 {
		t.Fatalf("quiet: %d total, %+v", a.QuietTotal, a.Quiet)
	}
	if a.Quiet[0].ID != never || a.Quiet[0].LastDuty != nil || a.Quiet[1].ID != quiet || a.Quiet[1].LastDuty == nil {
		t.Errorf("quiet order (never first, then longest ago): %+v", a.Quiet)
	}

	if _, err := LoadActivity(ctx, pool, "fire", members); err == nil {
		t.Error("an unknown faction should be refused")
	}
}
