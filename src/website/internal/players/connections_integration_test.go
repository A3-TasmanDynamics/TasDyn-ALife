package players

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConnections(t *testing.T) {
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

	ids := map[string]int64{}
	for name, uid := range map[string]string{"main": "76561190000000801", "alt": "76561190000000802", "friend": "76561190000000803", "stranger": "76561190000000804"} {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ($1, $2) RETURNING id`, uid, "Conn "+name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[name] = id
	}
	all := []int64{ids["main"], ids["alt"], ids["friend"], ids["stranger"]}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM players WHERE id = ANY($1)`, all) })

	// Name history: the insert and a rename are both recorded by the trigger.
	if _, err := pool.Exec(ctx, `UPDATE players SET name = 'Conn main renamed' WHERE id = $1`, ids["main"]); err != nil {
		t.Fatal(err)
	}
	// IPs: website logins, kept after the session is deleted (logout).
	login := func(who, ip string) {
		if _, err := pool.Exec(ctx, `INSERT INTO web_sessions (player_id, token_hash, ip_address, expires_at) VALUES ($1, md5(random()::text), $2::inet, now() + interval '1 day')`, ids[who], ip); err != nil {
			t.Fatal(err)
		}
	}
	login("main", "203.0.113.5")
	login("main", "203.0.113.5")
	login("main", "198.51.100.7")
	login("alt", "203.0.113.5")
	login("alt", "198.51.100.7")
	login("stranger", "192.0.2.99")
	pool.Exec(ctx, `DELETE FROM web_sessions WHERE player_id = ANY($1)`, all) // everyone logs out

	logEv := func(from, typ, details string) {
		if _, err := pool.Exec(ctx, `INSERT INTO player_log (player_id, event_type, details) VALUES ($1, $2, $3::jsonb)`, ids[from], typ, details); err != nil {
			t.Fatal(err)
		}
	}
	logEv("main", "bank.transfer", `{"to":"76561190000000802","amount":400000}`)
	logEv("main", "give.cash", `{"to":"76561190000000802","amount":50000}`)
	logEv("main", "give.item", `{"to":"76561190000000802","item":"oil_refined","qty":20}`)
	logEv("friend", "give.cash", `{"to":"76561190000000801","amount":1500}`)
	logEv("stranger", "shop.sell", `{"total":10}`) // not a move

	c, err := GetConnections(ctx, pool, ids["main"])
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Aliases) != 2 {
		t.Errorf("aliases: %+v", c.Aliases)
	}
	if len(c.Shared) != 1 || c.Shared[0].PlayerID != ids["alt"] || c.Shared[0].IPs != 2 || c.Shared[0].Sources != "website" {
		t.Errorf("shared: %+v", c.Shared)
	}
	if len(c.Partners) != 2 {
		t.Fatalf("partners: %+v", c.Partners)
	}
	alt, fr := c.Partners[0], c.Partners[1]
	if alt.PlayerID != ids["alt"] || alt.Sent != 450000 || alt.Recv != 0 || alt.ItemsSent != 20 || alt.Times != 3 || !alt.SharesIP {
		t.Errorf("alt: %+v", alt)
	}
	if fr.PlayerID != ids["friend"] || fr.Recv != 1500 || fr.Sent != 0 || fr.SharesIP || time.Since(fr.Last) > time.Minute {
		t.Errorf("friend: %+v", fr)
	}
	// And from the alt's side: received from main.
	c, _ = GetConnections(ctx, pool, ids["alt"])
	if len(c.Partners) != 1 || c.Partners[0].Recv != 450000 || c.Partners[0].ItemsRecv != 20 {
		t.Errorf("alt's view: %+v", c.Partners)
	}
	var times int
	pool.QueryRow(ctx, `SELECT times_seen FROM player_ips WHERE player_id = $1 AND ip_address = '203.0.113.5'`, ids["main"]).Scan(&times)
	if times != 2 {
		t.Errorf("times seen %d, want 2", times)
	}
}
