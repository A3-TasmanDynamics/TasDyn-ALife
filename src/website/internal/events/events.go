// Package events reads the game's event log (player_log): every economy
// action the game server records (ALife_fnc_anticheat "log") -- buying and
// selling, giving, the bank, containers, garages, fuel, robberies, police
// seizures -- for the admin panel's Event Log and Player Lookup's Events tab.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/gamenames"
)

// Category groups event types for the filter: key -> [label, SQL LIKE pattern(s)].
var Categories = []Category{
	{"shops", "Shops", []string{"shop.%"}},
	{"money", "Money (give, bank)", []string{"give.%", "bank.%"}},
	{"storage", "Storage (containers, garages)", []string{"container.%", "garage.%"}},
	{"crime", "Crime (robberies, the vault)", []string{"robbery.%", "evidence.%"}},
	{"police", "Police", []string{"police.%"}},
	{"fuel", "Fuel", []string{"gas.%"}},
	{"process", "Processing", []string{"process"}},
	{"anticheat", "Anti-cheat refusals", []string{"anticheat.%"}},
}

type Category struct {
	Key      string
	Label    string
	Patterns []string
}

// Ranges for the filter: key -> how far back ("" = all time).
var Ranges = []Range{{"24h", "Last 24 hours", 24 * time.Hour}, {"7d", "Last 7 days", 7 * 24 * time.Hour}, {"30d", "Last 30 days", 30 * 24 * time.Hour}, {"all", "All time", 0}}

type Range struct {
	Key   string
	Label string
	Back  time.Duration
}

// Filter narrows the list. PlayerID 0 = everyone; Search matches a name or
// Steam ID (ignored when PlayerID is set).
type Filter struct {
	PlayerID int64
	Search   string
	Category string
	Range    string
	From, To time.Time // optional window (zero = open-ended)
	Limit    int
	Offset   int
}

// Event is one row, ready to show.
type Event struct {
	ID         int64
	When       time.Time
	PlayerID   int64
	PlayerName string
	UID        string
	Type       string
	Category   string // its Categories key ("" = other)
	Summary    string // e.g. "Sold Heroin x5 for $12,500"
	Details    []KV   // the rest, as label: value
}

type KV struct{ K, V string }

// List returns one page of events, newest first, and whether there's more.
func List(ctx context.Context, pool *pgxpool.Pool, f Filter) ([]Event, bool, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	where := []string{"true"}
	args := []any{}
	arg := func(v any) string { args = append(args, v); return "$" + strconv.Itoa(len(args)) }
	if f.PlayerID > 0 {
		where = append(where, "l.player_id = "+arg(f.PlayerID))
	} else if s := strings.TrimSpace(f.Search); s != "" {
		where = append(where, "(p.uid = "+arg(s)+" OR p.name ILIKE "+arg("%"+s+"%")+" OR p.steam_name ILIKE "+arg("%"+s+"%")+")")
	}
	for _, c := range Categories {
		if c.Key == f.Category {
			var or []string
			for _, pat := range c.Patterns {
				or = append(or, "l.event_type LIKE "+arg(pat))
			}
			where = append(where, "("+strings.Join(or, " OR ")+")")
		}
	}
	for _, r := range Ranges {
		if r.Key == f.Range && r.Back > 0 {
			where = append(where, "l.created_at > "+arg(time.Now().Add(-r.Back)))
		}
	}
	if !f.From.IsZero() {
		where = append(where, "l.created_at >= "+arg(f.From))
	}
	if !f.To.IsZero() {
		where = append(where, "l.created_at <= "+arg(f.To))
	}
	q := `SELECT l.id, l.created_at, l.player_id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id), p.uid,
	             l.event_type, COALESCE(l.details::text, '')
	      FROM player_log l JOIN players p ON p.id = l.player_id
	      WHERE ` + strings.Join(where, " AND ") + `
	      ORDER BY l.created_at DESC, l.id DESC LIMIT ` + arg(f.Limit+1) + ` OFFSET ` + arg(f.Offset)
	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var details string
		if err := rows.Scan(&e.ID, &e.When, &e.PlayerID, &e.PlayerName, &e.UID, &e.Type, &details); err != nil {
			return nil, false, err
		}
		describe(&e, details)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	more := len(out) > f.Limit
	if more {
		out = out[:f.Limit]
	}
	nameRecipients(ctx, pool, out)
	return out, more, nil
}

// nameRecipients swaps the Steam IDs in "To" (gives, bank transfers) for
// player names, in one query for the page.
func nameRecipients(ctx context.Context, pool *pgxpool.Pool, evs []Event) {
	var uids []string
	for _, e := range evs {
		for _, kv := range e.Details {
			if kv.K == "To" {
				uids = append(uids, kv.V)
			}
		}
	}
	if len(uids) == 0 {
		return
	}
	names := map[string]string{}
	rows, err := pool.Query(ctx, `SELECT uid, COALESCE(NULLIF(name, ''), NULLIF(steam_name, ''), uid) FROM players WHERE uid = ANY($1)`, uids)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var u, n string
		if rows.Scan(&u, &n) == nil {
			names[u] = n
		}
	}
	for i := range evs {
		for j, kv := range evs[i].Details {
			if n, ok := names[kv.V]; ok && kv.K == "To" {
				evs[i].Details[j].V = n + " (" + kv.V + ")"
			}
		}
	}
}

// ---- Turning a type and its details into words ----------------------------

func money(v any) string {
	n, ok := num(v)
	if !ok {
		return fmt.Sprint(v)
	}
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		return "-$" + s
	}
	return "$" + s
}

func num(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case string:
		n, err := strconv.ParseInt(t, 10, 64)
		return n, err == nil
	}
	return 0, false
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if n, ok := num(v); ok {
		return strconv.FormatInt(n, 10)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// name is a class's in-game name, whatever kind of thing it is (or the
// classname, when the mission doesn't know it).
func name(class string) string {
	n, _ := gamenames.Any(class)
	return n
}

// items turns [[class, qty, ...], ...] (or [class, ...]) into "Name x2, Name".
func items(v any) string {
	list, ok := v.([]any)
	if !ok {
		return str(v)
	}
	var parts []string
	for _, it := range list {
		switch t := it.(type) {
		case string:
			parts = append(parts, name(t))
		case []any:
			if len(t) == 0 {
				continue
			}
			c, _ := t[0].(string)
			p := name(c)
			if len(t) > 1 {
				if n, ok := num(t[1]); ok && n > 1 {
					p += fmt.Sprintf(" x%d", n)
				}
			}
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, ", ")
}

var labels = map[string]string{
	"shop.buy": "Bought", "shop.sell": "Sold", "shop.buy_gear": "Bought gear", "shop.buy_clothing": "Bought clothing",
	"shop.buy_vehicle": "Bought a vehicle", "shop.buy_licence": "Bought a licence",
	"give.cash": "Gave cash", "give.item": "Gave items", "bank.transfer": "Bank transfer", "bank.deposit": "Deposited", "bank.withdraw": "Withdrew",
	"container.store": "Put in storage", "container.take": "Took from storage",
	"garage.retrieve": "Took a vehicle out", "garage.store": "Stored a vehicle", "garage.sell": "Sold a vehicle",
	"gas.refuel": "Filled up", "robbery.gas_start": "Started robbing a gas station", "robbery.gas": "Robbed a gas station",
	"police.seize": "Seized evidence", "police.vault_secured": "Secured the evidence vault", "police.evidence_destroyed": "Cleared the evidence",
	"evidence.breakin": "Broke into the evidence vault", "evidence.robbed": "Robbed the evidence vault",
	"process": "Processed", "anticheat.refused": "Refused (too fast)",
}

var keyLabels = map[string]string{
	"paid_by": "Paid by", "to": "To", "garage": "Garage", "shop": "Shop", "plate": "Plate", "id": "Vehicle id",
	"litres": "Litres", "station": "Station", "where": "Where", "faction": "Faction", "kind": "Kind", "container": "Container",
	"batches": "Batches", "stage": "Stage", "action": "Action", "requests": "Requests", "by": "By",
}

var moneyKeys = map[string]bool{"total": true, "amount": true, "cash": true, "price": true, "fee": true}

func describe(e *Event, raw string) {
	for _, c := range Categories {
		for _, p := range c.Patterns {
			if (strings.HasSuffix(p, "%") && strings.HasPrefix(e.Type, strings.TrimSuffix(p, "%"))) || p == e.Type {
				e.Category = c.Key
			}
		}
	}
	d := map[string]any{}
	_ = json.Unmarshal([]byte(raw), &d)
	used := map[string]bool{}
	take := func(k string) any { used[k] = true; return d[k] }
	label := labels[e.Type]
	if label == "" {
		label = strings.ReplaceAll(strings.ReplaceAll(e.Type, ".", " "), "_", " ")
		if label != "" {
			label = strings.ToUpper(label[:1]) + label[1:]
		}
	}
	switch e.Type {
	case "shop.buy", "shop.sell":
		e.Summary = fmt.Sprintf("%s %s x%s for %s", label, name(str(take("item"))), str(take("qty")), money(take("total")))
	case "shop.buy_gear", "shop.buy_clothing":
		e.Summary = fmt.Sprintf("%s: %s for %s", label, items(take("items")), money(take("total")))
	case "shop.buy_vehicle":
		e.Summary = fmt.Sprintf("Bought a %s for %s", name(str(take("vehicle"))), money(take("total")))
	case "shop.buy_licence":
		e.Summary = fmt.Sprintf("Bought the %s licence for %s", gamenames.Licence(str(take("licence"))), money(take("total")))
	case "give.cash":
		e.Summary = fmt.Sprintf("Gave %s cash", money(take("amount")))
	case "give.item":
		e.Summary = fmt.Sprintf("Gave %s x%s", name(str(take("item"))), str(take("qty")))
	case "bank.transfer", "bank.deposit", "bank.withdraw":
		e.Summary = fmt.Sprintf("%s %s", label, money(take("amount")))
	case "container.store", "container.take":
		e.Summary = fmt.Sprintf("%s: %s", label, items(take("moved")))
	case "garage.retrieve", "garage.store", "garage.sell":
		e.Summary = fmt.Sprintf("%s: %s", label, name(str(take("vehicle"))))
		if p, ok := d["price"]; ok {
			e.Summary += " for " + money(p)
			used["price"] = true
		}
	case "gas.refuel":
		e.Summary = fmt.Sprintf("Filled up: %s, %s L for %s", name(str(take("vehicle"))), str(take("litres")), money(take("total")))
	case "robbery.gas":
		e.Summary = fmt.Sprintf("Robbed %s for %s", str(take("where")), money(take("cash")))
	case "police.seize":
		e.Summary = fmt.Sprintf("Seized %s", str(take("what")))
		used["position"] = true
	case "evidence.breakin":
		stage := map[string]string{"entrance": "Broke into the evidence building", "vaultDoor": "Broke the vault room door"}[str(take("stage"))]
		if stage == "" {
			stage = label
		}
		e.Summary = stage
	case "evidence.robbed":
		e.Summary = fmt.Sprintf("Robbed the evidence vault: %s weapons, %s gear, %s items, %s", str(take("weapons")), str(take("gear")), str(take("items")), money(take("cash")))
	case "anticheat.refused":
		e.Summary = fmt.Sprintf("Refused (too fast): %s, %s requests", str(take("action")), str(take("requests")))
	case "process":
		e.Summary = fmt.Sprintf("Processed %s x%s (fee %s)", strings.ReplaceAll(str(take("process")), "_", " "), str(take("batches")), money(take("fee")))
	default:
		e.Summary = label
	}
	keys := make([]string, 0, len(d))
	for k := range d {
		if !used[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		l := keyLabels[k]
		if l == "" {
			l = strings.ToUpper(k[:1]) + strings.ReplaceAll(k[1:], "_", " ")
		}
		v := d[k]
		switch {
		case moneyKeys[k]:
			e.Details = append(e.Details, KV{l, money(v)})
		case k == "items" || k == "moved":
			e.Details = append(e.Details, KV{l, items(v)})
		case k == "vehicle" || k == "item":
			e.Details = append(e.Details, KV{l, name(str(v))})
		default:
			e.Details = append(e.Details, KV{l, str(v)})
		}
	}
}
