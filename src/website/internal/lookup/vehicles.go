// Package lookup backs the Admin Panel's Vehicle Lookup and Houses &
// Garages pages: read-only views of the vehicles and houses tables, with
// the game's names (internal/gamenames) instead of classnames.
package lookup

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
	"website/internal/gamenames"
)

const ownerExpr = `COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id)`

// Side maps vehicles.faction_scope to what staff call it.
func Side(scope string) string {
	switch scope {
	case "police":
		return "Police"
	case "medic":
		return "EMS"
	default:
		return "Civilian"
	}
}

// StatusLabel is vehicles.status as the game describes it.
func StatusLabel(status string) string {
	switch status {
	case "garaged":
		return "In garage"
	case "spawned":
		return "Out"
	case "impounded":
		return "Impounded"
	case "destroyed":
		return "Destroyed"
	}
	return status
}

var Statuses = []string{"garaged", "spawned", "impounded", "destroyed"}

type VehicleRow struct {
	ID      int64
	Class   string
	Name    string
	Plate   string
	Owner   string
	OwnerID int64 // 0 = a gang's
	Side    string
	Status  string
}

func (v VehicleRow) StatusText() string { return StatusLabel(v.Status) }

// SearchVehicles finds vehicles by plate, vehicle name or classname, owner
// (name, Steam64, gang) or vehicle id; status narrows it ("" = any). With
// no query, the newest vehicles. Up to 50.
func SearchVehicles(ctx context.Context, pool *pgxpool.Pool, q, status string) ([]VehicleRow, error) {
	q = strings.TrimSpace(q)
	like := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
	id, _ := strconv.ParseInt(strings.TrimPrefix(q, "#"), 10, 64)
	classes := gamenames.VehiclesNamed(q)
	rows, err := pool.Query(ctx, `
		SELECT v.id, v.classname, COALESCE(v.plate, ''), COALESCE(`+ownerExpr+`, g.name, ''), COALESCE(v.owner_player_id, 0),
		       COALESCE(v.faction_scope, 'civilian'), v.status
		FROM vehicles v
		LEFT JOIN players p ON p.id = v.owner_player_id
		LEFT JOIN gangs g ON g.id = v.owner_gang_id
		WHERE ($1 = '' OR v.plate ILIKE $2 OR v.classname ILIKE $2 OR p.name ILIKE $2 OR p.uid = $1 OR g.name ILIKE $2
		       OR v.id = $3 OR lower(v.classname) = ANY($5))
		  AND ($4 = '' OR v.status = $4)
		ORDER BY (v.id = $3 OR upper(v.plate) = upper($1)) DESC, v.id DESC
		LIMIT 50`, q, like, id, status, classes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VehicleRow
	for rows.Next() {
		var v VehicleRow
		if err := rows.Scan(&v.ID, &v.Class, &v.Plate, &v.Owner, &v.OwnerID, &v.Side, &v.Status); err != nil {
			return nil, err
		}
		v.Name = gamenames.Vehicle(v.Class)
		v.Side = Side(v.Side)
		out = append(out, v)
	}
	return out, rows.Err()
}

type Part struct {
	Name string
	Pct  int
}

type Stack struct {
	Name  string
	Key   string
	Count int64
}

type VehicleEvent struct {
	When time.Time
	What string
	By   string
}

type Vehicle struct {
	VehicleRow
	OwnerUID string
	Gang     string
	Garage   string
	Fuel     int // percent
	Damage   []Part
	Items    []Stack
	Cash     int64
	Created  time.Time
	History  []VehicleEvent
}

// GetVehicle loads one vehicle with its state (fuel, damage, trunk) and
// history: vehicle_logs (bought, impounded, released, ...) and staff
// actions on it from the game's admin menu.
func GetVehicle(ctx context.Context, pool *pgxpool.Pool, id int64) (Vehicle, error) {
	var v Vehicle
	var fuel float64
	var damage, inv []byte
	err := pool.QueryRow(ctx, `
		SELECT v.id, v.classname, COALESCE(v.plate, ''), COALESCE(`+ownerExpr+`, ''), COALESCE(v.owner_player_id, 0),
		       COALESCE(v.faction_scope, 'civilian'), v.status, COALESCE(p.uid, ''), COALESCE(g.name, ''), COALESCE(gr.name, ''),
		       v.fuel, v.damage, v.inventory, v.created_at
		FROM vehicles v
		LEFT JOIN players p ON p.id = v.owner_player_id
		LEFT JOIN gangs g ON g.id = v.owner_gang_id
		LEFT JOIN garages gr ON gr.id = v.garage_id
		WHERE v.id = $1`, id).Scan(&v.ID, &v.Class, &v.Plate, &v.Owner, &v.OwnerID, &v.Side, &v.Status, &v.OwnerUID, &v.Gang, &v.Garage,
		&fuel, &damage, &inv, &v.Created)
	if err != nil {
		return v, err
	}
	v.Name = gamenames.Vehicle(v.Class)
	v.Side = Side(v.Side)
	v.Fuel = int(fuel*100 + 0.5)
	v.Damage = parseDamage(damage)
	v.Items, v.Cash = parseTrunk(inv)

	rows, err := pool.Query(ctx, `
		SELECT vl.created_at, vl.action, COALESCE(vl.amount, 0), COALESCE(`+ownerExpr+`, ''), NULL::jsonb, NULL::jsonb, ''
		FROM vehicle_logs vl LEFT JOIN players p ON p.id = vl.player_id
		WHERE vl.vehicle_id = $1
		UNION ALL
		SELECT sl.created_at, sl.action, 0, COALESCE(`+ownerExpr+`, 'Staff'), sl.before_value, sl.after_value, COALESCE(sl.reason, '')
		FROM staff_log sl LEFT JOIN players p ON p.id = sl.staff_player_id
		WHERE sl.action LIKE 'game.vehicle%' AND sl.after_value->>'vehicle' = $1::text
		ORDER BY 1 DESC LIMIT 50`, id)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	for rows.Next() {
		var e VehicleEvent
		var action, reason string
		var amount int64
		var before, after []byte
		if err := rows.Scan(&e.When, &action, &amount, &e.By, &before, &after, &reason); err != nil {
			return v, err
		}
		if strings.HasPrefix(action, "game.") {
			e.What = audit.Describe(action, before, after)
			if reason != "" {
				e.What += ": " + reason
			}
		} else {
			e.What = vehicleAction(action)
			if amount != 0 {
				e.What += " · $" + dollars(amount)
			}
		}
		v.History = append(v.History, e)
	}
	return v, rows.Err()
}

func vehicleAction(a string) string {
	switch a {
	case "purchase":
		return "Bought"
	case "rent":
		return "Rented"
	case "sell":
		return "Sold"
	case "chop":
		return "Chopped"
	case "impound":
		return "Impounded"
	case "return":
		return "Released from the impound"
	}
	return a
}

// parseDamage reads vehicles.damage ([[hitpoint, damage], ...]), worst
// first.
func parseDamage(raw []byte) []Part {
	var pairs [][]json.RawMessage
	if json.Unmarshal(raw, &pairs) != nil {
		return nil
	}
	var out []Part
	for _, p := range pairs {
		if len(p) < 2 {
			continue
		}
		var name string
		var d float64
		if json.Unmarshal(p[0], &name) != nil || json.Unmarshal(p[1], &d) != nil || d <= 0 {
			continue
		}
		out = append(out, Part{Name: hitpointName(name), Pct: int(d*100 + 0.5)})
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Pct > out[j-1].Pct; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// "hitlfwheel" -> "LF wheel", "hitengine" -> "Engine".
func hitpointName(h string) string {
	k := strings.TrimPrefix(strings.ToLower(h), "hit")
	switch k {
	case "lfwheel", "lf2wheel", "rfwheel", "rf2wheel", "lbwheel", "lmwheel", "rbwheel", "rmwheel":
		return strings.ToUpper(k[:2]) + " wheel"
	case "hull":
		return "Body"
	}
	if k == "" {
		return h
	}
	return strings.ToUpper(k[:1]) + k[1:]
}

// parseTrunk reads vehicles.inventory: [["items", [[key, count], ...]],
// ["cash", n]] (ALife_fnc_vehicleState). Anything else is ignored.
func parseTrunk(raw []byte) ([]Stack, int64) {
	var pairs [][]json.RawMessage
	if json.Unmarshal(raw, &pairs) != nil {
		return nil, 0
	}
	var items []Stack
	var cash int64
	for _, p := range pairs {
		if len(p) < 2 {
			continue
		}
		var key string
		if json.Unmarshal(p[0], &key) != nil {
			continue
		}
		switch key {
		case "cash":
			var c float64
			if json.Unmarshal(p[1], &c) == nil {
				cash = int64(c)
			}
		case "items":
			var stacks [][]json.RawMessage
			if json.Unmarshal(p[1], &stacks) != nil {
				continue
			}
			for _, s := range stacks {
				if len(s) < 2 {
					continue
				}
				var k string
				var n float64
				if json.Unmarshal(s[0], &k) != nil || json.Unmarshal(s[1], &n) != nil || n <= 0 {
					continue
				}
				items = append(items, Stack{Name: gamenames.Item(k), Key: k, Count: int64(n)})
			}
		}
	}
	return items, cash
}

// ---- Garages ----

type GarageCount struct {
	Side      string
	Garaged   int
	Out       int
	Impounded int
	Destroyed int
}

type ImpoundRow struct {
	VehicleRow
	Since *time.Time
}

type Garages struct {
	Counts  []GarageCount
	Impound []ImpoundRow
	Out     []VehicleRow
	Total   int
}

// GarageOverview is every faction's garages at a glance: how many vehicles
// are stored, out, impounded and destroyed, the impound lot (longest
// there first) and what's out in the world.
func GarageOverview(ctx context.Context, pool *pgxpool.Pool) (Garages, error) {
	var g Garages
	scopes := []string{"civilian", "police", "medic"}
	counts := map[string]*GarageCount{}
	for _, s := range scopes {
		counts[s] = &GarageCount{Side: Side(s)}
	}
	rows, err := pool.Query(ctx, `SELECT COALESCE(faction_scope, 'civilian'), status, count(*) FROM vehicles GROUP BY 1, 2`)
	if err != nil {
		return g, err
	}
	for rows.Next() {
		var scope, status string
		var n int
		if err := rows.Scan(&scope, &status, &n); err != nil {
			rows.Close()
			return g, err
		}
		c := counts[scope]
		if c == nil {
			continue
		}
		switch status {
		case "garaged":
			c.Garaged += n
		case "spawned":
			c.Out += n
		case "impounded":
			c.Impounded += n
		case "destroyed":
			c.Destroyed += n
		}
		g.Total += n
	}
	rows.Close()
	for _, s := range scopes {
		g.Counts = append(g.Counts, *counts[s])
	}

	rows, err = pool.Query(ctx, `
		SELECT v.id, v.classname, COALESCE(v.plate, ''), COALESCE(`+ownerExpr+`, ''), COALESCE(v.owner_player_id, 0),
		       COALESCE(v.faction_scope, 'civilian'), v.status,
		       (SELECT max(vl.created_at) FROM vehicle_logs vl WHERE vl.vehicle_id = v.id AND vl.action = 'impound')
		FROM vehicles v LEFT JOIN players p ON p.id = v.owner_player_id
		WHERE v.status IN ('impounded', 'spawned')
		ORDER BY 8 ASC NULLS LAST, v.id
		LIMIT 300`)
	if err != nil {
		return g, err
	}
	defer rows.Close()
	for rows.Next() {
		var r ImpoundRow
		if err := rows.Scan(&r.ID, &r.Class, &r.Plate, &r.Owner, &r.OwnerID, &r.Side, &r.Status, &r.Since); err != nil {
			return g, err
		}
		r.Name = gamenames.Vehicle(r.Class)
		r.Side = Side(r.Side)
		if r.Status == "impounded" {
			g.Impound = append(g.Impound, r)
		} else {
			g.Out = append(g.Out, r.VehicleRow)
		}
	}
	return g, rows.Err()
}

func dollars(amount int64) string {
	neg := amount < 0
	if neg {
		amount = -amount
	}
	whole := strconv.FormatInt(amount, 10)
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}
