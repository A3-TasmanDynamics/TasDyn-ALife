package devboard

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runs only with TEST_DATABASE_URL (a throwaway database built from schema.sql).
func TestLabels(t *testing.T) {
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
		pool.Exec(ctx, `DELETE FROM dev_tasks WHERE title LIKE 'TL %'`)
		pool.Exec(ctx, `DELETE FROM dev_labels WHERE name LIKE 'TL %'`)
		pool.Close()
	})
	find := func(name string) (Label, bool) {
		all, err := ListLabels(ctx, pool)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range all {
			if l.Name == name {
				return l, true
			}
		}
		return Label{}, false
	}

	var ue UserError
	if err := SaveLabel(ctx, pool, "", "TL Police", "teal"); !errors.As(err, &ue) {
		t.Errorf("bad colour: %v", err)
	}
	if err := SaveLabel(ctx, pool, "", "TL Police", "blue"); err != nil {
		t.Fatal(err)
	}
	if err := SaveLabel(ctx, pool, "", "tl police", "red"); !errors.As(err, &ue) {
		t.Errorf("duplicate name (case-insensitive): %v", err)
	}

	// Cards pick up the saved spelling, and new names become labels.
	id, err := Create(ctx, pool, 0, Task{Title: "TL card", Labels: []string{"tl POLICE", "TL Core"}})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := Get(ctx, pool, id)
	if len(got.Labels) != 2 || got.Labels[0] != "TL Police" || got.Labels[1] != "TL Core" {
		t.Errorf("card labels: %v", got.Labels)
	}
	if l, ok := find("TL Core"); !ok || l.Color != defaultColour("TL Core") || l.Cards != 1 {
		t.Errorf("auto label: %+v %v", l, ok)
	}

	// Rename and recolour follow through to the card.
	if err := SaveLabel(ctx, pool, "TL Police", "TL Faction - Police", "sky"); err != nil {
		t.Fatal(err)
	}
	got, _ = Get(ctx, pool, id)
	if got.Labels[0] != "TL Faction - Police" {
		t.Errorf("renamed on card: %v", got.Labels)
	}
	if l, ok := find("TL Faction - Police"); !ok || l.Color != "sky" {
		t.Errorf("renamed label: %+v", l)
	}

	if err := DeleteLabel(ctx, pool, "TL Core"); err != nil {
		t.Fatal(err)
	}
	got, _ = Get(ctx, pool, id)
	if len(got.Labels) != 1 {
		t.Errorf("deleted label still on card: %v", got.Labels)
	}
	if _, ok := find("TL Core"); ok {
		t.Error("deleted label still listed")
	}
}
