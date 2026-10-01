package devboard

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runs only with TEST_DATABASE_URL (a throwaway database built from schema.sql).
func TestCards(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM dev_tasks WHERE title LIKE 'TC %'`); pool.Close() })

	due := time.Now().AddDate(0, 0, -1)
	a, err := Create(ctx, pool, 0, Task{Title: "TC a", Due: &due})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Create(ctx, pool, 0, Task{Title: "TC b"})
	var ue UserError

	// Checklists and items.
	if err := AddChecklist(ctx, pool, 0, a, "  "); err != nil {
		t.Fatal(err)
	}
	c, _ := GetCard(ctx, pool, a)
	if len(c.Checklists) != 1 || c.Checklists[0].Title != "Checklist" {
		t.Fatalf("checklist: %+v", c.Checklists)
	}
	cl := c.Checklists[0].ID
	if err := AddItem(ctx, pool, a, cl, "one"); err != nil {
		t.Fatal(err)
	}
	AddItem(ctx, pool, a, cl, "two")
	if err := AddItem(ctx, pool, b, cl, "wrong card"); !errors.As(err, &ue) {
		t.Errorf("adding to another card's checklist: %v", err)
	}
	c, _ = GetCard(ctx, pool, a)
	done, total, err := SetItem(ctx, pool, 0, a, c.Checklists[0].Items[0].ID, true)
	if err != nil || done != 1 || total != 2 {
		t.Errorf("tick: %d/%d %v", done, total, err)
	}
	if task, _ := Get(ctx, pool, a); task.ChecksDone != 1 || task.ChecksTotal != 2 || task.DueState() != "overdue" {
		t.Errorf("card front: %+v due %s", task, task.DueState())
	}

	// Links read both ways.
	if err := AddLink(ctx, pool, 0, a, b, "blocked_by"); err != nil {
		t.Fatal(err)
	}
	if err := AddLink(ctx, pool, 0, b, a, "relates"); !errors.As(err, &ue) {
		t.Errorf("second link between the same cards: %v", err)
	}
	if err := AddLink(ctx, pool, 0, a, a, "relates"); !errors.As(err, &ue) {
		t.Errorf("self link: %v", err)
	}
	ca, _ := GetCard(ctx, pool, a)
	cb, _ := GetCard(ctx, pool, b)
	if len(ca.Links) != 1 || ca.Links[0].Label != "Blocked by" || len(cb.Links) != 1 || cb.Links[0].Label != "Blocks" {
		t.Errorf("links: a=%+v b=%+v", ca.Links, cb.Links)
	}

	// Comments: only the author deletes.
	if err := AddComment(ctx, pool, 0, a, "hello"); err != nil {
		t.Fatal(err)
	}
	ca, _ = GetCard(ctx, pool, a)
	if len(ca.Comments) != 1 || len(ca.Activity) < 3 {
		t.Errorf("comments %+v activity %+v", ca.Comments, ca.Activity)
	}
	if err := DeleteComment(ctx, pool, 999999, a, ca.Comments[0].ID); !errors.As(err, &ue) {
		t.Errorf("someone else deleting a comment: %v", err)
	}

	if err := RemoveLink(ctx, pool, 0, b, a); err != nil {
		t.Fatal(err)
	}
	if cb, _ = GetCard(ctx, pool, b); len(cb.Links) != 0 {
		t.Errorf("link not removed: %+v", cb.Links)
	}
	if err := DeleteChecklist(ctx, pool, 0, a, cl); err != nil {
		t.Fatal(err)
	}
	if task, _ := Get(ctx, pool, a); task.ChecksTotal != 0 {
		t.Errorf("items should go with their checklist: %+v", task)
	}
}
