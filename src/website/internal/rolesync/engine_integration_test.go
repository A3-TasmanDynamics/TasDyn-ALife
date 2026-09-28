package rolesync

import (
	"context"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// fakeAdapter pretends to be a platform. It registers as "teamspeak" so a
// live website (whose engine only has a Discord adapter) can never pick up
// this test's mappings and apply them to real members.
type fakeAdapter struct {
	ids    map[int64][]string
	groups map[string][]string
}

func (f *fakeAdapter) Platform() string { return "teamspeak" }
func (f *fakeAdapter) Identities(_ context.Context, id int64) ([]string, error) {
	return f.ids[id], nil
}
func (f *fakeAdapter) CurrentGroups(_ context.Context, ident string) ([]string, bool, error) {
	g, ok := f.groups[ident]
	return append([]string(nil), g...), ok, nil
}
func (f *fakeAdapter) AddGroup(_ context.Context, ident, g string) error {
	f.groups[ident] = append(f.groups[ident], g)
	return nil
}
func (f *fakeAdapter) RemoveGroup(_ context.Context, ident, g string) error {
	var keep []string
	for _, x := range f.groups[ident] {
		if x != g {
			keep = append(keep, x)
		}
	}
	f.groups[ident] = keep
	return nil
}

// Runs only with TEST_DATABASE_URL.
func TestEngineSyncsFromDatabase(t *testing.T) {
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

	var pid int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO players (uid, staff_rank_id, staff_team, cop_level)
		VALUES ('76561190000000051', (SELECT id FROM staff_ranks WHERE key = 'moderator'), 'Moderation', 2)
		RETURNING id`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM sync_log WHERE player_id = $1`, pid)
		pool.Exec(ctx, `DELETE FROM platform_group_map WHERE platform = 'teamspeak' AND group_id LIKE 'test-%'`)
		pool.Exec(ctx, `DELETE FROM players WHERE id = $1`, pid)
	})
	for ent, g := range map[string]string{
		"linked": "test-verified", "staff_rank:moderator": "test-mod", "staff_loa": "test-loa",
		"staff_team:moderation": "test-team-mod", "faction_rank:police:2": "test-police",
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO platform_group_map (platform, entitlement, group_id) VALUES ('teamspeak', $1, $2)`, ent, g); err != nil {
			t.Fatal(err)
		}
	}

	fake := &fakeAdapter{
		ids:    map[int64][]string{pid: {"ts-uid-1"}},
		groups: map[string][]string{"ts-uid-1": {"unmanaged-guest", "test-loa"}},
	}
	e := &Engine{Pool: pool, Adapters: []Adapter{fake}}
	if _, err := e.SyncPlayer(ctx, pid, 0); err != nil {
		t.Fatal(err)
	}
	got := append([]string(nil), fake.groups["ts-uid-1"]...)
	sort.Strings(got)
	want := []string{"test-mod", "test-police", "test-team-mod", "test-verified", "unmanaged-guest"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("after sync: %v, want %v", got, want)
	}

	// Going on LOA swaps the rank group for the LOA group; team stays.
	pool.Exec(ctx, `UPDATE players SET staff_status = 'loa' WHERE id = $1`, pid)
	pool.Exec(ctx, `DELETE FROM staff_log WHERE target_player_id = $1`, pid) // trigger's row; keep #staff-log clean
	if _, err := e.SyncPlayer(ctx, pid, 0); err != nil {
		t.Fatal(err)
	}
	got = append([]string(nil), fake.groups["ts-uid-1"]...)
	sort.Strings(got)
	want = []string{"test-loa", "test-police", "test-team-mod", "test-verified", "unmanaged-guest"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("after LOA: %v, want %v", got, want)
	}

	var logged int
	pool.QueryRow(ctx, `SELECT count(*) FROM sync_log WHERE player_id = $1 AND ok`, pid).Scan(&logged)
	// First pass: +mod +police +team +verified -loa (5); LOA pass: +loa -mod (2).
	if logged != 7 {
		t.Errorf("expected 7 sync_log rows, got %d", logged)
	}
}
