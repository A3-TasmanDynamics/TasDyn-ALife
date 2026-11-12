package discord

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCollectWatchAlerts(t *testing.T) {
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
	var w, n int64
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000981', 'Watched_Wes') RETURNING id`).Scan(&w)
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000982', 'Normal Nia') RETURNING id`).Scan(&n)
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM players WHERE id IN ($1, $2)`, w, n) })
	if _, err := pool.Exec(ctx, `INSERT INTO player_watchlist (player_id, reason) VALUES ($1, 'Suspected @here duping')`, w); err != nil {
		t.Fatal(err)
	}

	cfg := WatchConfig{Join: true, Money: 20000, General: 100000, SiteBase: "https://example.test"}
	// The test DB is shared, so other tests' rows can show up: only look at ours.
	ours := func(as []Alert) []string {
		var out []string
		for _, a := range as {
			if strings.Contains(a.Text, "Wes") || strings.Contains(a.Text, "Nia") {
				out = append(out, a.Text)
			}
		}
		return out
	}
	pool.Exec(ctx, `INSERT INTO player_sessions (player_id, faction) VALUES ($1, 'cop')`, w)
	got, sc, lc, err := CollectWatchAlerts(ctx, pool, cfg, -1, -1)
	if err != nil || len(ours(got)) != 0 {
		t.Fatalf("first run posts nothing old: %v %v", got, err)
	}

	pool.Exec(ctx, `INSERT INTO player_sessions (player_id, faction) VALUES ($1, 'cop'), ($2, 'civ')`, w, n)
	for _, e := range []struct {
		who  int64
		typ  string
		json string
	}{
		{w, "bank.transfer", `{"to":"76561190000000982","amount":25000}`}, // over the watch bar
		{w, "give.cash", `{"amount":500}`},                                // under it
		{w, "shop.sell", `{"total":150000}`},                              // the general alert has it
		{n, "bank.transfer", `{"amount":50000}`},                          // not watched
	} {
		pool.Exec(ctx, `INSERT INTO player_log (player_id, event_type, details) VALUES ($1, $2, $3::jsonb)`, e.who, e.typ, e.json)
	}
	got, sc2, lc2, err := CollectWatchAlerts(ctx, pool, cfg, sc, lc)
	if err != nil {
		t.Fatal(err)
	}
	texts := ours(got)
	all := strings.Join(texts, "\n")
	t.Log("\n" + all)
	if len(texts) != 2 {
		t.Fatalf("want 2 alerts (join, transfer), got %d", len(texts))
	}
	for _, want := range []string{`**Watched\_Wes** joined as police`, "transferred **$25,000** to **Normal Nia**", "https://example.test/admin/players?id="} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(all, "@here") || strings.Contains(all, "$500") || strings.Contains(all, "$150,000") || strings.Contains(all, "$50,000") {
		t.Error("ping, small move, double-posted move or unwatched player leaked through")
	}
	again, sc3, lc3, _ := CollectWatchAlerts(ctx, pool, cfg, sc2, lc2)
	if len(ours(again)) != 0 || sc3 < sc2 || lc3 < lc2 {
		t.Errorf("repeat: %v", ours(again))
	}
	pool.Exec(ctx, `INSERT INTO player_sessions (player_id) VALUES ($1)`, w)
	off, sc4, _, _ := CollectWatchAlerts(ctx, pool, WatchConfig{}, sc3, lc3)
	if len(off) != 0 || sc4 <= sc3 {
		t.Errorf("off: %d alerts, cursor %d -> %d", len(off), sc3, sc4)
	}
}
