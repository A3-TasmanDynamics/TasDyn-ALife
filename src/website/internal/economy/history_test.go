package economy

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSnapshots(t *testing.T) {
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
	pool.Exec(ctx, `DELETE FROM economy_snapshots`)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM economy_snapshots`)
		pool.Exec(ctx, `DELETE FROM market_prices WHERE item = 'water'`)
	})
	pool.Exec(ctx, `INSERT INTO market_prices (item, price) VALUES ('water', 20) ON CONFLICT (item) DO UPDATE SET price = 20`)

	wrote, err := TakeSnapshot(ctx, pool)
	if err != nil || !wrote {
		t.Fatalf("first snapshot: wrote=%v err=%v", wrote, err)
	}
	if wrote, _ := TakeSnapshot(ctx, pool); wrote {
		t.Error("a second snapshot straight away should be skipped")
	}
	// An older one, as if taken a day ago with no market moves.
	pool.Exec(ctx, `INSERT INTO economy_snapshots (taken_at, civ_cash, cop_cash, medic_cash, civ_bank, cop_bank, medic_bank, bounties)
		VALUES (now() - interval '1 day', 100, 0, 0, 1000, 0, 0, 0)`)
	h, err := History(ctx, pool, time.Now().Add(-48*time.Hour))
	if err != nil || len(h) != 2 {
		t.Fatalf("history: %d err=%v", len(h), err)
	}
	if h[0].Cash != 100 || h[0].Bank != 1000 || h[0].MarketIndex != 0 {
		t.Errorf("old point: %+v", h[0])
	}
	// water is 20 against a base of 10 (+100%), one of 36 items: index ~ +2.8%.
	if h[1].MarketIndex < 2.5 || h[1].MarketIndex > 3.1 {
		t.Errorf("market index %v", h[1].MarketIndex)
	}
	if h[1].At.Before(h[0].At) {
		t.Error("not oldest first")
	}
}
