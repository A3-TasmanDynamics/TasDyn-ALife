package discord

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCollectAlerts(t *testing.T) {
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
	var a, b int64
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000991', '@everyone_Al') RETURNING id`).Scan(&a)
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000992', 'Bea') RETURNING id`).Scan(&b)
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM players WHERE id IN ($1, $2)`, a, b) })

	cfg := AlertConfig{Flags: true, Vault: true, Money: 100000, SiteBase: "https://example.test"}
	// First run: starts from now, posts nothing old.
	pool.Exec(ctx, `INSERT INTO anti_cheat_flags (player_id, flag_type, confidence) VALUES ($1, 'dupe', 'high')`, a)
	got, fc, lc, err := CollectAlerts(ctx, pool, cfg, -1, -1)
	if err != nil || len(got) != 0 {
		t.Fatalf("first run: %v %v", got, err)
	}

	pool.Exec(ctx, `INSERT INTO anti_cheat_flags (player_id, flag_type, confidence, details) VALUES ($1, 'cheat_state', 'high', '{"godmode":true}'), ($1, 'rate_limit', 'medium', '{}')`, a)
	for _, e := range [][2]string{
		{"bank.transfer", `{"to":"76561190000000992","amount":250000}`},
		{"bank.transfer", `{"to":"76561190000000992","amount":5000}`}, // under the threshold
		{"shop.sell", `{"total":100000}`},                             // exactly the threshold
		{"evidence.breakin", `{"stage":"entrance"}`},
		{"evidence.robbed", `{"weapons":2,"gear":1,"items":4,"cash":9000}`},
		{"shop.buy", `{"total":900000}`}, // spending isn't alerted
	} {
		pool.Exec(ctx, `INSERT INTO player_log (player_id, event_type, details) VALUES ($1, $2, $3::jsonb)`, a, e[0], e[1])
	}
	got, fc2, lc2, err := CollectAlerts(ctx, pool, cfg, fc, lc)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, x := range got {
		texts = append(texts, x.Text)
	}
	all := strings.Join(texts, "\n")
	t.Log("\n" + all)
	if len(got) != 5 {
		t.Fatalf("want 5 alerts (1 flag, 2 money, breakin, robbery), got %d", len(got))
	}
	for _, want := range []string{"cheat state", "transferred **$250,000** to **Bea**", "sold goods for **$100,000**", "broke into the evidence building", "robbed the evidence vault**: 2 weapons", "https://example.test/admin/anticheat?flag="} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(all, "@everyone") || strings.Contains(all, "rate limit") || strings.Contains(all, "$5,000") {
		t.Error("ping, medium flag or small transfer leaked through")
	}
	// Nothing new: nothing posted, cursors stay.
	again, fc3, lc3, _ := CollectAlerts(ctx, pool, cfg, fc2, lc2)
	if len(again) != 0 || fc3 != fc2 || lc3 != lc2 {
		t.Errorf("repeat: %d alerts", len(again))
	}
	// Turned off: no alerts, but the cursors still move on.
	pool.Exec(ctx, `INSERT INTO anti_cheat_flags (player_id, flag_type, confidence) VALUES ($1, 'gear', 'high')`, b)
	off, fc4, _, _ := CollectAlerts(ctx, pool, AlertConfig{}, fc3, lc3)
	if len(off) != 0 || fc4 <= fc3 {
		t.Errorf("off: %d alerts, cursor %d -> %d", len(off), fc3, fc4)
	}
	// A flood is capped with a summary line.
	for i := 0; i < 30; i++ {
		pool.Exec(ctx, `INSERT INTO anti_cheat_flags (player_id, flag_type, confidence) VALUES ($1, 'gear', 'high')`, b)
	}
	flood, _, _, _ := CollectAlerts(ctx, pool, cfg, fc4, lc3)
	if len(flood) != maxAlertsPerPoll || !strings.Contains(flood[len(flood)-1].Text, "16 more alerts") {
		t.Errorf("flood: %d, last %q", len(flood), flood[len(flood)-1].Text)
	}
}
