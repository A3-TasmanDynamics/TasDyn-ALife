package replies

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestReplies(t *testing.T) {
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
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000851', 'Replies Staff') RETURNING id`).Scan(&staff)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM saved_replies WHERE title LIKE 'RT %'`)
		pool.Exec(ctx, `DELETE FROM players WHERE id = $1`, staff)
	})
	denied := func(what string, err error) {
		t.Helper()
		if !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%s: want not allowed, got %v", what, err)
		}
	}
	_, err = Save(ctx, pool, staff, 0, "email", "RT x", "y")
	denied("bad scope", err)
	_, err = Save(ctx, pool, staff, 0, "ticket", "  ", "y")
	denied("no title", err)
	_, err = Save(ctx, pool, staff, 0, "ticket", "RT x", "  ")
	denied("no body", err)

	a, err := Save(ctx, pool, staff, 0, "ticket", "RT Compensation approved", "Hi {player},\r\nwe've refunded you.")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Save(ctx, pool, staff, 0, "ticket", "RT Compensation approved", "again")
	denied("duplicate title in a scope", err)
	if _, err := Save(ctx, pool, staff, 0, "case", "RT Compensation approved", "fine in another scope"); err != nil {
		t.Errorf("same title, other scope: %v", err)
	}
	if _, err := Save(ctx, pool, staff, a, "note", "RT Compensation approved", "moved"); err != nil {
		t.Fatal(err)
	}
	_, err = Save(ctx, pool, staff, -1, "note", "RT ghost", "x")
	denied("edit a missing one", err)

	notes, err := List(ctx, pool, "note")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range notes {
		if r.ID == a {
			found = r.Body == "moved" && r.UpdatedBy == "Replies Staff"
		}
		if r.Scope != "note" {
			t.Errorf("scope filter let through %+v", r)
		}
	}
	if !found {
		t.Error("edited reply not in its new scope")
	}
	if err := Delete(ctx, pool, a); err != nil {
		t.Fatal(err)
	}
	denied("delete twice", Delete(ctx, pool, a))
}
