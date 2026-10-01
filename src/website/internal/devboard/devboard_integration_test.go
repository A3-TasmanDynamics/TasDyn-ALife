package devboard

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runs only with TEST_DATABASE_URL (a throwaway database built from schema.sql).
func TestBoard(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM dev_tasks WHERE title LIKE 'TB %'`)
		pool.Exec(ctx, `DELETE FROM dev_labels WHERE name IN ('Web Site', 'bug')`)
		pool.Close()
	})

	var ue UserError
	if _, err := Create(ctx, pool, 0, Task{Title: "  "}); !errors.As(err, &ue) {
		t.Errorf("blank title: %v", err)
	}
	a, err := Create(ctx, pool, 0, Task{Title: "TB  a", Labels: []string{"Web  Site", "web site", " bug "}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Create(ctx, pool, 0, Task{Title: "TB b"})
	c, _ := Create(ctx, pool, 0, Task{Title: "TB c"})
	got, _ := Get(ctx, pool, a)
	if got.Title != "TB a" || len(got.Labels) != 2 || got.Labels[0] != "Web Site" || got.Labels[1] != "bug" || got.Status != "todo" || got.Priority != "normal" {
		t.Errorf("cleaned task: %+v", got)
	}

	order := func(status string) []int64 {
		cols, err := Board(ctx, pool, Filter{Q: "TB "})
		if err != nil {
			t.Fatal(err)
		}
		var ids []int64
		for _, col := range cols {
			if col.Key == status {
				for _, task := range col.Tasks {
					ids = append(ids, task.ID)
				}
			}
		}
		return ids
	}
	if err := Move(ctx, pool, 0, c, "todo", a); err != nil {
		t.Fatal(err)
	}
	if got := order("todo"); len(got) != 3 || got[0] != c || got[1] != a || got[2] != b {
		t.Errorf("after moving c before a: %v (a=%d b=%d c=%d)", got, a, b, c)
	}
	if err := Move(ctx, pool, 0, b, "done", 0); err != nil {
		t.Fatal(err)
	}
	if done, _ := Get(ctx, pool, b); done.Status != "done" || done.DoneAt == nil {
		t.Errorf("done task: %+v", done)
	}
	if err := Move(ctx, pool, 0, b, "nope", 0); !errors.As(err, &ue) {
		t.Errorf("bad column: %v", err)
	}
	got.Status, got.Priority = "doing", "urgent"
	if err := Update(ctx, pool, 0, got); err != nil {
		t.Fatal(err)
	}
	if got := order("doing"); len(got) != 1 || got[0] != a {
		t.Errorf("updated into doing: %v", got)
	}
	if cols, _ := Board(ctx, pool, Filter{Q: "TB ", Label: "bug"}); len(cols[1].Tasks) != 1 {
		t.Errorf("label filter: %+v", cols)
	}
	if err := Delete(ctx, pool, a); err != nil {
		t.Fatal(err)
	}
	if _, err := Get(ctx, pool, a); err == nil {
		t.Error("deleted task still there")
	}
}
