package events

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestList(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var a, b int64
	if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000101', 'Evtest Alpha') RETURNING id`).Scan(&a); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000102', 'Evtest Bravo') RETURNING id`).Scan(&b); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM players WHERE id IN ($1, $2)`, a, b) })
	rows := []struct {
		p    int64
		typ  string
		json string
		age  string
	}{
		{a, "shop.sell", `{"shop":"market_1","item":"oil_refined","qty":5,"total":12500}`, "1 hour"},
		{a, "shop.buy_gear", `{"shop":"gun_1","items":[["optic_Hamr",1,500],["30Rnd_65x39_caseless_mag",3,75]],"total":725,"paid_by":"cash"}`, "2 hours"},
		{a, "bank.transfer", `{"to":"76561190000000102","amount":40000}`, "3 hours"},
		{b, "robbery.gas", `{"station":"gas_cashier_3","where":"Kavala","cash":3200}`, "4 hours"},
		{b, "garage.sell", `{"vehicle":"C_Offroad_01_F","id":7,"plate":"ABC123","garage":"kavala","price":6250}`, "5 hours"},
		{b, "police.seize", `{"what":"Heroin x3; MX","position":[1,2,3]}`, "10 days"},
		{b, "death", `{"killer":"76561190000000101"}`, "6 hours"},
	}
	for _, r := range rows {
		if _, err := pool.Exec(ctx, `INSERT INTO player_log (player_id, event_type, details, created_at) VALUES ($1, $2, $3::jsonb, now() - $4::interval)`, r.p, r.typ, r.json, r.age); err != nil {
			t.Fatal(err)
		}
	}

	// One player, newest first, everything.
	ev, more, err := List(ctx, pool, Filter{PlayerID: a})
	if err != nil || more || len(ev) != 3 {
		t.Fatalf("player a: %d events, more=%v, err=%v", len(ev), more, err)
	}
	if ev[0].Type != "shop.sell" || ev[0].Category != "shops" || !strings.Contains(ev[0].Summary, "Sold Refined Oil x5 for $12,500") {
		t.Errorf("first: %+v", ev[0])
	}
	for _, e := range ev {
		t.Logf("%-15s %-10s %s %v", e.Type, e.Category, e.Summary, e.Details)
	}

	// Search by name + category + range: Bravo's crime in the last 7 days.
	ev, _, err = List(ctx, pool, Filter{Search: "evtest bra", Category: "crime", Range: "7d"})
	if err != nil || len(ev) != 1 || ev[0].Type != "robbery.gas" || ev[0].PlayerName != "Evtest Bravo" {
		t.Fatalf("crime filter: %+v err=%v", ev, err)
	}
	t.Logf("%s", ev[0].Summary)

	// The 10-day-old seizure is outside 7 days, inside all time.
	ev, _, _ = List(ctx, pool, Filter{Search: "76561190000000102", Category: "police", Range: "7d"})
	if len(ev) != 0 {
		t.Errorf("7d police: %d", len(ev))
	}
	ev, _, _ = List(ctx, pool, Filter{Search: "76561190000000102", Category: "police", Range: "all"})
	if len(ev) != 1 {
		t.Fatalf("all police: %d", len(ev))
	}
	for _, kv := range ev[0].Details {
		if kv.K == "Position" {
			t.Errorf("position should be hidden: %v", ev[0].Details)
		}
	}
	t.Logf("%s", ev[0].Summary)

	// Paging: 2 per page over Bravo's 4 events.
	p1, more1, _ := List(ctx, pool, Filter{PlayerID: b, Limit: 2})
	p2, more2, _ := List(ctx, pool, Filter{PlayerID: b, Limit: 2, Offset: 2})
	if len(p1) != 2 || !more1 || len(p2) != 2 || more2 || p1[1].ID == p2[0].ID {
		t.Errorf("paging: %d %v %d %v", len(p1), more1, len(p2), more2)
	}
	// An event outside every category still shows (as "other").
	if p2[0].Type != "death" && p2[1].Type != "death" {
		t.Errorf("death missing from page 2")
	}
}
