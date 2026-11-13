package dashboard

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
	var me int64
	if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000901', 'Activity Me') RETURNING id`).Scan(&me); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM players WHERE id = $1`, me) })

	// Two finished sessions (90 min four days ago at local noon, 30 min
	// eight days ago at local noon) and one open session started 20 min ago.
	if _, err := pool.Exec(ctx, `
		INSERT INTO player_sessions (player_id, connected_at, disconnected_at, faction) VALUES
		($1, ((now() AT TIME ZONE $2)::date - 4 + time '12:00') AT TIME ZONE $2, ((now() AT TIME ZONE $2)::date - 4 + time '13:30') AT TIME ZONE $2, 'cop'),
		($1, ((now() AT TIME ZONE $2)::date - 8 + time '12:00') AT TIME ZONE $2, ((now() AT TIME ZONE $2)::date - 8 + time '12:30') AT TIME ZONE $2, 'civ'),
		($1, now() - interval '20 minutes', NULL, 'medic')`, me, Region); err != nil {
		t.Fatal(err)
	}
	var acct int64
	if err := pool.QueryRow(ctx, `INSERT INTO bank_accounts (player_id, faction, balance) VALUES ($1, 'civilian', 0) RETURNING id`, me).Scan(&acct); err != nil {
		t.Fatal(err)
	}
	// One at a time: the balance trigger checks each row in turn.
	for _, tx := range []struct {
		typ, memo, ago string
		amount, after  int64
	}{
		{"payout", "Copper", "2 hours", 500, 500},
		{"purchase", "Quadbike", "1 hour", -200, 300},
		{"admin_adjustment", "staff-only note", "30 minutes", 100, 400},
	} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO bank_transactions (account_id, type, amount, balance_after, memo, created_at)
			VALUES ($1, $2, $3, $4, $5, now() - $6::interval)`, acct, tx.typ, tx.amount, tx.after, tx.memo, tx.ago); err != nil {
			t.Fatal(err)
		}
	}

	a, err := LoadActivity(ctx, pool, me)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Days) != 14 {
		t.Fatalf("want 14 days, got %d", len(a.Days))
	}
	if a.Days[13-4].Minutes != 90 || a.Days[13-8].Minutes != 30 {
		t.Errorf("finished sessions: day-4 %d min, day-8 %d min", a.Days[13-4].Minutes, a.Days[13-8].Minutes)
	}
	// Days with no session must be zero (not a whole day).
	for i, d := range a.Days {
		if i != 13-4 && i != 13-8 && i != 13 && d.Minutes != 0 {
			t.Errorf("day %d has %d min with no session", i, d.Minutes)
		}
	}
	if today := a.Days[13].Minutes; today < 19 || today > 21 {
		// The open session can straddle local midnight only in the first 20 min of a day.
		if a.Days[12].Minutes+today < 19 {
			t.Errorf("open session counted %d min today", today)
		}
	}
	if a.ActiveDays < 3 || a.Days[13-4].Pct != 100 {
		t.Errorf("active days %d, busiest pct %d", a.ActiveDays, a.Days[13-4].Pct)
	}

	if len(a.Recent) != 6 {
		t.Fatalf("want 6 recent events, got %d: %+v", len(a.Recent), a.Recent)
	}
	if a.Recent[0].Title != "Online now" {
		t.Errorf("newest event should be the open session: %+v", a.Recent[0])
	}
	var sawPurchase, sawAdjust bool
	for _, e := range a.Recent {
		switch e.Title {
		case "Purchase":
			sawPurchase = true
			if e.Kind != "money-out" || e.Amount != -200 || e.AbsAmount() != 200 {
				t.Errorf("purchase: %+v", e)
			}
		case "Staff adjustment":
			sawAdjust = true
			if e.Kind != "money-in" || e.Detail != "Civilian bank" {
				t.Errorf("staff adjustment must not show its memo: %+v", e)
			}
		}
	}
	if !sawPurchase || !sawAdjust {
		t.Errorf("missing events: purchase %v, adjustment %v", sawPurchase, sawAdjust)
	}
	for i := 1; i < len(a.Recent); i++ {
		if a.Recent[i].At.After(a.Recent[i-1].At) {
			t.Fatal("recent activity is not newest first")
		}
	}
}
