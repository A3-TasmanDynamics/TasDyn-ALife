// Package economy is the admin panel's Economy & Crime page: where the
// money is (cash vs bank, per faction), what put money into the game and
// took it out (from the event log and the bank), the richest players, the
// dynamic market against its base prices, crime (robberies, bounties) and
// what's in the police evidence vault.
package economy

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/gamenames"
)

// ---- Money supply ----------------------------------------------------------

type Faction struct {
	Key, Label string
	Cash, Bank int64
}

func (f Faction) Total() int64 { return f.Cash + f.Bank }

// Supply is all the money in the game, per faction.
func Supply(ctx context.Context, pool *pgxpool.Pool) ([]Faction, error) {
	out := []Faction{{Key: "civilian", Label: "Civilian"}, {Key: "police", Label: "Police"}, {Key: "medic", Label: "EMS"}}
	if err := pool.QueryRow(ctx, `SELECT COALESCE(sum(civ_cash), 0), COALESCE(sum(cop_cash), 0), COALESCE(sum(medic_cash), 0) FROM players`).
		Scan(&out[0].Cash, &out[1].Cash, &out[2].Cash); err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `SELECT faction, COALESCE(sum(balance), 0) FROM bank_accounts GROUP BY faction`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var f string
		var b int64
		if err := rows.Scan(&f, &b); err != nil {
			return nil, err
		}
		for i := range out {
			if out[i].Key == f {
				out[i].Bank = b
			}
		}
	}
	return out, rows.Err()
}

// ---- Money in and out ------------------------------------------------------

// Flow is one way money enters (faucet) or leaves (sink) the game.
type Flow struct {
	Label  string
	Amount int64
	Count  int
}

// flowSources: event type -> [label, the details key holding the dollars].
var faucets = []struct{ Type, Label, Key string }{
	{"shop.sell", "Selling to traders", "total"},
	{"garage.sell", "Selling vehicles", "price"},
	{"robbery.gas", "Gas station robberies", "cash"},
	{"evidence.robbed", "Evidence vault robberies", "cash"},
}
var sinks = []struct{ Type, Label, Key string }{
	{"shop.buy", "Market & shop items", "total"},
	{"shop.buy_gear", "Weapons & gear", "total"},
	{"shop.buy_clothing", "Clothing", "total"},
	{"shop.buy_vehicle", "Vehicles", "total"},
	{"shop.buy_licence", "Licences", "total"},
	{"gas.refuel", "Fuel", "total"},
	{"garage.retrieve", "Garage fees", "fee"},
	{"process", "Processing fees", "fee"},
}

// Flows is the money that came into and went out of the game since
// `since`: from the event log, plus bank payouts (jobs, paychecks) and
// staff adjustments.
func Flows(ctx context.Context, pool *pgxpool.Pool, since time.Time) (in, out []Flow, err error) {
	sum := func(typ, key string) (Flow, error) {
		var f Flow
		err := pool.QueryRow(ctx, `
			SELECT COALESCE(sum(CASE WHEN jsonb_typeof(details -> $2) = 'number' THEN (details ->> $2)::numeric ELSE 0 END), 0)::bigint, count(*)
			FROM player_log WHERE event_type = $1 AND created_at > $3`, typ, key, since).Scan(&f.Amount, &f.Count)
		return f, err
	}
	for _, s := range faucets {
		f, err := sum(s.Type, s.Key)
		if err != nil {
			return nil, nil, err
		}
		f.Label = s.Label
		in = append(in, f)
	}
	for _, s := range sinks {
		f, err := sum(s.Type, s.Key)
		if err != nil {
			return nil, nil, err
		}
		f.Label = s.Label
		out = append(out, f)
	}
	var pay, payN, adjIn, adjInN, adjOut, adjOutN int64
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(sum(amount) FILTER (WHERE type = 'payout'), 0), count(*) FILTER (WHERE type = 'payout'),
		       COALESCE(sum(amount) FILTER (WHERE type = 'admin_adjustment' AND amount > 0), 0), count(*) FILTER (WHERE type = 'admin_adjustment' AND amount > 0),
		       COALESCE(-sum(amount) FILTER (WHERE type = 'admin_adjustment' AND amount < 0), 0), count(*) FILTER (WHERE type = 'admin_adjustment' AND amount < 0)
		FROM bank_transactions WHERE created_at > $1`, since).Scan(&pay, &payN, &adjIn, &adjInN, &adjOut, &adjOutN); err != nil {
		return nil, nil, err
	}
	in = append(in, Flow{"Bank payouts (jobs, pay)", pay, int(payN)}, Flow{"Staff compensation", adjIn, int(adjInN)})
	out = append(out, Flow{"Staff deductions", adjOut, int(adjOutN)})
	byAmount := func(f []Flow) { sort.SliceStable(f, func(i, j int) bool { return f[i].Amount > f[j].Amount }) }
	byAmount(in)
	byAmount(out)
	return in, out, nil
}

// Total adds up flows.
func Total(f []Flow) int64 {
	var t int64
	for _, x := range f {
		t += x.Amount
	}
	return t
}

// ---- Richest players -------------------------------------------------------

type Rich struct {
	ID         int64
	Name       string
	Cash, Bank int64
	Flagged    bool // has an unreviewed anti-cheat flag
}

func (r Rich) Total() int64 { return r.Cash + r.Bank }

func Richest(ctx context.Context, pool *pgxpool.Pool, n int) ([]Rich, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, name, cash, bank, flagged FROM (
		  SELECT p.id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id) AS name,
		         COALESCE(p.civ_cash, 0) + COALESCE(p.cop_cash, 0) + COALESCE(p.medic_cash, 0) AS cash,
		         COALESCE((SELECT sum(balance) FROM bank_accounts b WHERE b.player_id = p.id), 0)::bigint AS bank,
		         EXISTS (SELECT 1 FROM anti_cheat_flags f WHERE f.player_id = p.id AND f.resolution IS NULL) AS flagged
		  FROM players p) r
		ORDER BY cash + bank DESC, id LIMIT $1`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Rich
	for rows.Next() {
		var r Rich
		if err := rows.Scan(&r.ID, &r.Name, &r.Cash, &r.Bank, &r.Flagged); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---- The market ------------------------------------------------------------

type Price struct {
	Key, Name     string
	Current, Base int64
	Min, Max      int64
	Change        int // percent vs base, rounded
	Updated       *time.Time
}

// Market is every CfgMarket item's current buy price against its base,
// biggest moves first. Items the market hasn't saved a price for are at base.
func Market(ctx context.Context, pool *pgxpool.Pool) ([]Price, error) {
	cur := map[string]float64{}
	when := map[string]time.Time{}
	rows, err := pool.Query(ctx, `SELECT lower(item), price::float8, updated_at FROM market_prices`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var k string
		var p float64
		var t time.Time
		if err := rows.Scan(&k, &p, &t); err != nil {
			rows.Close()
			return nil, err
		}
		cur[k], when[k] = p, t
	}
	rows.Close()
	var out []Price
	for _, m := range gamenames.Market() {
		p := Price{Key: m.Key, Name: gamenames.Item(m.Key), Current: m.Buy, Base: m.Buy, Min: m.Min, Max: m.Max}
		if c, ok := cur[m.Key]; ok {
			p.Current = int64(math.Round(c))
			t := when[m.Key]
			p.Updated = &t
		}
		if p.Base > 0 {
			p.Change = int(math.Round(float64(p.Current-p.Base) * 100 / float64(p.Base)))
		}
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool { return abs(out[i].Change) > abs(out[j].Change) })
	return out, rows.Err()
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// ---- Crime -----------------------------------------------------------------

type Crime struct {
	GasRobberies, VaultRobberies, VaultBreakins int // in the window
	RobbedCash                                  int64
	Seizures                                    int
	WantedPlayers                               int
	Bounties                                    int64 // outstanding
	MostWanted                                  []Wanted
}

type Wanted struct {
	ID     int64
	Name   string
	Bounty int64
	Crimes int
}

func CrimeStats(ctx context.Context, pool *pgxpool.Pool, since time.Time) (Crime, error) {
	var c Crime
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE event_type = 'robbery.gas'),
		       count(*) FILTER (WHERE event_type = 'evidence.robbed'),
		       count(*) FILTER (WHERE event_type = 'evidence.breakin'),
		       COALESCE(sum(CASE WHEN event_type IN ('robbery.gas', 'evidence.robbed') AND jsonb_typeof(details -> 'cash') = 'number'
		                         THEN (details ->> 'cash')::numeric ELSE 0 END), 0)::bigint,
		       count(*) FILTER (WHERE event_type = 'police.seize')
		FROM player_log WHERE created_at > $1 AND event_type IN ('robbery.gas', 'evidence.robbed', 'evidence.breakin', 'police.seize')`, since). // idx_player_log_event_time
		Scan(&c.GasRobberies, &c.VaultRobberies, &c.VaultBreakins, &c.RobbedCash, &c.Seizures); err != nil {
		return c, err
	}
	if err := pool.QueryRow(ctx, `SELECT count(DISTINCT player_id), COALESCE(sum(bounty_amount), 0) FROM wanted_crimes WHERE cleared_at IS NULL`).
		Scan(&c.WantedPlayers, &c.Bounties); err != nil {
		return c, err
	}
	rows, err := pool.Query(ctx, `
		SELECT p.id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id), sum(w.bounty_amount), count(*)
		FROM wanted_crimes w JOIN players p ON p.id = w.player_id
		WHERE w.cleared_at IS NULL GROUP BY p.id ORDER BY 3 DESC, p.id LIMIT 5`)
	if err != nil {
		return c, err
	}
	defer rows.Close()
	for rows.Next() {
		var w Wanted
		if err := rows.Scan(&w.ID, &w.Name, &w.Bounty, &w.Crimes); err != nil {
			return c, err
		}
		c.MostWanted = append(c.MostWanted, w)
	}
	return c, rows.Err()
}

// ---- The evidence vault ----------------------------------------------------

type Vault struct {
	Exists  bool // the game has saved it at least once
	Cash    int64
	Items   []Count // virtual items
	Gear    []Count // weapons, magazines, items, uniforms / vests / bags (with what's in them)
	Updated time.Time
}

type Count struct {
	Name string
	N    int64
}

func EvidenceVault(ctx context.Context, pool *pgxpool.Pool) (Vault, error) {
	var v Vault
	var gear, items []byte
	err := pool.QueryRow(ctx, `SELECT gear::text, items::text, cash, updated_at FROM evidence_vault WHERE id = 1`).Scan(&gear, &items, &v.Cash, &v.Updated)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v, nil
		}
		return v, err
	}
	v.Exists = true
	v.Items = countPairs(items, gamenames.Item)
	v.Gear = vaultGear(gear)
	return v, nil
}

// countPairs reads [[class, count], ...].
func countPairs(raw []byte, name func(string) string) []Count {
	var pairs [][]any
	if json.Unmarshal(raw, &pairs) != nil {
		return nil
	}
	tally := map[string]int64{}
	for _, p := range pairs {
		if len(p) < 2 {
			continue
		}
		c, _ := p[0].(string)
		n, _ := p[1].(float64)
		if c != "" && n > 0 {
			tally[c] += int64(n)
		}
	}
	return sorted(tally, name)
}

// vaultGear reads the vault's inventory as the game saves it:
// [["items", [[class, n], ...]], ["mags", [[class, ammo], ...]],
//
//	["weapons", [[weapon, muzzle, pointer, optic, [mag, ammo], [], bipod], ...]],
//	["containers", [[class, items, mags, weapons], ...]]]
//
// and counts every class in it, including what's inside uniforms, vests
// and bags and the attachments on weapons.
func vaultGear(raw []byte) []Count {
	var top [][]any
	if json.Unmarshal(raw, &top) != nil {
		return nil
	}
	tally := map[string]int64{}
	add := func(c string, n int64) {
		if c != "" && n > 0 {
			tally[c] += n
		}
	}
	var items, mags, weapons func(v any)
	items = func(v any) {
		for _, p := range list(v) {
			pl := list(p)
			if len(pl) >= 2 {
				c, _ := pl[0].(string)
				n, _ := pl[1].(float64)
				add(c, int64(n))
			}
		}
	}
	mags = func(v any) {
		for _, p := range list(v) {
			if pl := list(p); len(pl) >= 1 {
				c, _ := pl[0].(string)
				add(c, 1)
			}
		}
	}
	weapons = func(v any) {
		for _, w := range list(v) {
			for i, part := range list(w) {
				switch x := part.(type) {
				case string:
					add(x, 1) // the weapon (0) and its muzzle / pointer / optic / bipod
				case []any:
					if len(x) >= 1 && i > 0 {
						c, _ := x[0].(string)
						add(c, 1) // a loaded magazine
					}
				}
			}
		}
	}
	for _, kv := range top {
		if len(kv) < 2 {
			continue
		}
		switch kv[0] {
		case "items":
			items(kv[1])
		case "mags":
			mags(kv[1])
		case "weapons":
			weapons(kv[1])
		case "containers":
			for _, c := range list(kv[1]) {
				cl := list(c)
				if len(cl) == 0 {
					continue
				}
				cls, _ := cl[0].(string)
				add(cls, 1)
				if len(cl) > 1 {
					items(cl[1])
				}
				if len(cl) > 2 {
					mags(cl[2])
				}
				if len(cl) > 3 {
					weapons(cl[3])
				}
			}
		}
	}
	return sorted(tally, gamenames.Gear)
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func sorted(tally map[string]int64, name func(string) string) []Count {
	out := make([]Count, 0, len(tally))
	for c, n := range tally {
		out = append(out, Count{name(c), n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return out[i].Name < out[j].Name
	})
	return out
}
