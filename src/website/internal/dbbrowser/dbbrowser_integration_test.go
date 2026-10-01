package dbbrowser

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runs only with TEST_DATABASE_URL (a throwaway database built from schema.sql).
func TestBrowse(t *testing.T) {
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
	b := &Browser{Pool: pool}

	// Composite foreign keys pair up column by column.
	fks, err := b.ForeignKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[FK]bool{}
	var folderDiv bool
	for _, f := range fks {
		if seen[f] {
			t.Errorf("duplicate foreign key %+v", f)
		}
		seen[f] = true
		if f.Table == "faction_drive_folders" && f.Column == "division_key" {
			folderDiv = f.RefTable == "faction_divisions" && f.RefColumn == "key"
		}
	}
	if !folderDiv {
		t.Error("faction_drive_folders.division_key should point at faction_divisions.key")
	}

	// Sort, exact match and NULL cells on staff_ranks (seeded by schema.sql).
	p, err := b.Rows(ctx, "staff_ranks", Opts{Sort: "level", Asc: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if p.Sort != "level" || p.Desc || len(p.Rows) < 2 {
		t.Fatalf("sorted page: %+v", p)
	}
	lvl := -1
	for i, c := range p.Columns {
		if c.Name == "level" {
			lvl = i
			if c.Kind != "num" {
				t.Errorf("level kind = %q", c.Kind)
			}
		}
	}
	if p.Rows[0][lvl].Text > p.Rows[1][lvl].Text && len(p.Rows[0][lvl].Text) >= len(p.Rows[1][lvl].Text) {
		t.Errorf("not ascending: %v then %v", p.Rows[0][lvl], p.Rows[1][lvl])
	}
	one, err := b.Rows(ctx, "staff_ranks", Opts{Col: "key", Eq: "admin", Limit: 10})
	if err != nil || one.Total != 1 {
		t.Errorf("exact match: %v %+v", err, one)
	}
	if _, err := b.Rows(ctx, "staff_ranks", Opts{Sort: "nope; DROP TABLE x", Limit: 1}); err != nil {
		t.Errorf("unknown sort column should be ignored, got %v", err)
	}

	info, err := b.Describe(ctx, "players")
	if err != nil || len(info.Indexes) == 0 || len(info.Incoming) == 0 || info.Size == "" {
		t.Errorf("describe players: %v %+v", err, info)
	}
	if PrettyJSON(`{"a":1}`) != "{\n  \"a\": 1\n}" || PrettyJSON("not json") != "not json" {
		t.Error("PrettyJSON")
	}
}
