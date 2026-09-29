package rules

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runs only with TEST_DATABASE_URL, and only on a database with no rules
// published yet (it doesn't touch real rules). Stop the live website
// first: Save writes staff_log rows a running poster would send to Discord.
func TestSaveVersions(t *testing.T) {
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
	var existing int
	pool.QueryRow(ctx, `SELECT count(*) FROM rule_versions`).Scan(&existing)
	if existing > 0 {
		t.Skip("rules already published on this database")
	}
	var actor, maxLog int64
	if err := pool.QueryRow(ctx, `INSERT INTO players (uid) VALUES ('76561190000000071') RETURNING id`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	pool.QueryRow(ctx, `SELECT COALESCE(max(id), 0) FROM staff_log`).Scan(&maxLog)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM rule_versions WHERE created_by = $1`, actor)
		pool.Exec(ctx, `DELETE FROM staff_log WHERE id > $1 AND staff_player_id = $2`, maxLog, actor)
		pool.Exec(ctx, `DELETE FROM players WHERE id = $1`, actor)
	})

	if _, err := Save(ctx, pool, actor, "# A\n1.1 One\n1.2 Two", ""); err != nil {
		t.Fatal("first version without a note:", err)
	}
	if _, err := Save(ctx, pool, actor, "# A\n1.1 One\n1.2 Two", "x"); !errors.Is(err, ErrUnchanged) {
		t.Errorf("identical save: %v", err)
	}
	if _, err := Save(ctx, pool, actor, "# A\n1.1 One\n1.2 Twice", ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("later version without a note: %v", err)
	}
	changed, err := Save(ctx, pool, actor, "# A\n1.1 One\n1.2 Twice\n1.3 Three", "Clarified 1.2, added 1.3")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(changed, []string{"1.2", "1.3"}) {
		t.Errorf("changed = %v", changed)
	}
	doc, ok, err := Current(ctx, pool)
	if err != nil || !ok || !doc.Recent || doc.ChangeNote != "Clarified 1.2, added 1.3" {
		t.Fatalf("current = %+v ok=%v err=%v", doc, ok, err)
	}
	if doc.Sections[0].Rules[0].Changed || !doc.Sections[0].Rules[1].Changed {
		t.Error("changed flags wrong")
	}
	var logged int
	pool.QueryRow(ctx, `SELECT count(*) FROM staff_log WHERE id > $1 AND action = 'rules.update' AND staff_player_id = $2`, maxLog, actor).Scan(&logged)
	if logged != 2 {
		t.Errorf("rules.update rows = %d, want 2", logged)
	}
}
