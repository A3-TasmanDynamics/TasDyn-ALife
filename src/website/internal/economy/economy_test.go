package economy

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The vault's gear as fn_evidence saves it (SQF str of the inventory).
func TestVaultGear(t *testing.T) {
	raw := `[["items",[["optic_Hamr",2],["FirstAidKit",1]]],
		["mags",[["30Rnd_65x39_caseless_mag",30],["30Rnd_65x39_caseless_mag",12]]],
		["weapons",[["arifle_MX_F","","acc_pointer_IR","optic_Hamr",["30Rnd_65x39_caseless_mag",30],[],""]]],
		["containers",[["V_PlateCarrier1_rgr",[["FirstAidKit",2]],[["30Rnd_65x39_caseless_mag",30]],[]]]]]`
	got := map[string]int64{}
	for _, c := range vaultGear([]byte(raw)) {
		got[c.Name] += c.N
	}
	// 2 loose + 1 on the rifle; 1 loose + 2 in the vest; 2 loose + 1 loaded + 1 in the vest.
	if len(got) == 0 {
		t.Fatal("nothing parsed")
	}
	total := int64(0)
	for _, n := range got {
		total += n
	}
	// optic 3, first aid 3, mags 4, MX 1, pointer 1, vest 1
	if total != 13 {
		t.Errorf("total pieces %d, want 13: %v", total, got)
	}
	if vaultGear([]byte(`not json`)) != nil {
		t.Error("bad JSON should give nothing")
	}
}

func TestEconomy(t *testing.T) {
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

	beforeSupply, err := Supply(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	var a, b int64
	if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name, civ_cash, cop_cash) VALUES ('76561190000000601', 'Econ Alpha', 5000, 700) RETURNING id`).Scan(&a); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name, civ_cash) VALUES ('76561190000000602', 'Econ Bravo', 100) RETURNING id`).Scan(&b); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM players WHERE id IN ($1, $2)`, a, b)
		pool.Exec(ctx, `DELETE FROM market_prices WHERE item IN ('water', 'apple')`)
	})
	var acct int64
	if err := pool.QueryRow(ctx, `INSERT INTO bank_accounts (player_id, faction) VALUES ($1, 'civilian') RETURNING id`, a).Scan(&acct); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO bank_transactions (account_id, type, amount, balance_after, memo) VALUES ($1, 'payout', 90000, 90000, 'job')`, acct); err != nil {
		t.Fatal(err)
	}

	after, err := Supply(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if d := after[0].Cash - beforeSupply[0].Cash; d != 5100 {
		t.Errorf("civ cash +%d, want +5100", d)
	}
	if d := after[1].Cash - beforeSupply[1].Cash; d != 700 {
		t.Errorf("cop cash +%d, want +700", d)
	}
	if d := after[0].Bank - beforeSupply[0].Bank; d != 90000 {
		t.Errorf("civ bank +%d, want +90000 (the trigger keeps balances)", d)
	}

	since := time.Now().Add(-time.Hour)
	for _, e := range [][3]any{
		{"shop.sell", `{"item":"oil_refined","qty":5,"total":12500}`, "10 minutes"},
		{"robbery.gas", `{"where":"Kavala","cash":3200}`, "20 minutes"},
		{"shop.buy_vehicle", `{"vehicle":"C_Offroad_01_F","total":18000}`, "30 minutes"},
		{"garage.retrieve", `{"vehicle":"C_Offroad_01_F","fee":150}`, "35 minutes"},
		{"shop.sell", `{"item":"oil_refined","qty":5,"total":99999}`, "3 hours"}, // outside the window
		{"police.seize", `{"what":"MX"}`, "5 minutes"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO player_log (player_id, event_type, details, created_at) VALUES ($1, $2, $3::jsonb, now() - $4::interval)`, a, e[0], e[1], e[2]); err != nil {
			t.Fatal(err)
		}
	}
	in, out, err := Flows(ctx, pool, since)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{"Selling to traders": 12500, "Gas station robberies": 3200, "Bank payouts (jobs, pay)": 90000, "Vehicles": 18000, "Garage fees": 150}
	for _, f := range append(append([]Flow{}, in...), out...) {
		if w, ok := want[f.Label]; ok && f.Amount != w {
			t.Errorf("%s: $%d, want $%d", f.Label, f.Amount, w)
		}
	}
	if in[0].Label != "Bank payouts (jobs, pay)" {
		t.Errorf("in not sorted by amount: %v", in)
	}

	rich, err := Richest(ctx, pool, 50)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rich {
		if r.ID == a {
			found = true
			if r.Cash != 5700 || r.Bank != 90000 || r.Total() != 95700 {
				t.Errorf("alpha: %+v", r)
			}
		}
	}
	if !found {
		t.Error("alpha not in the rich list")
	}

	pool.Exec(ctx, `INSERT INTO market_prices (item, price) VALUES ('water', 15), ('apple', 40) ON CONFLICT (item) DO UPDATE SET price = EXCLUDED.price`)
	mkt, err := Market(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(mkt) < 30 || mkt[0].Key != "water" || mkt[0].Change != 50 || mkt[0].Name != "Water Bottle" {
		t.Errorf("market top: %+v (of %d)", mkt[0], len(mkt))
	}
	for _, p := range mkt {
		if p.Key == "apple" && p.Change != -20 {
			t.Errorf("apple: %+v", p)
		}
	}

	pool.Exec(ctx, `INSERT INTO wanted_crimes (player_id, crime_key, bounty_amount) VALUES ($1, 'robbery', 5000), ($1, 'evidence_robbery', 20000)`, b)
	c, err := CrimeStats(ctx, pool, since)
	if err != nil {
		t.Fatal(err)
	}
	if c.GasRobberies != 1 || c.RobbedCash != 3200 || c.Seizures != 1 {
		t.Errorf("crime: %+v", c)
	}
	if len(c.MostWanted) == 0 || c.MostWanted[0].ID != b || c.MostWanted[0].Bounty != 25000 || c.MostWanted[0].Crimes != 2 {
		t.Errorf("most wanted: %+v", c.MostWanted)
	}

	// The vault: one row, as the game saves it.
	pool.Exec(ctx, `DELETE FROM evidence_vault`)
	v, err := EvidenceVault(ctx, pool)
	if err != nil || v.Exists {
		t.Errorf("empty vault: %+v err=%v", v, err)
	}
	pool.Exec(ctx, `INSERT INTO evidence_vault (id, gear, items, cash) VALUES (1, '[["items",[["optic_Hamr",2]]]]', '[["oil_refined",3],["water",1]]', 4500)`)
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM evidence_vault`) })
	v, err = EvidenceVault(ctx, pool)
	if err != nil || !v.Exists || v.Cash != 4500 || len(v.Items) != 2 || v.Items[0].Name != "Refined Oil" || v.Items[0].N != 3 || len(v.Gear) != 1 || v.Gear[0].N != 2 {
		t.Errorf("vault: %+v err=%v", v, err)
	}
}
