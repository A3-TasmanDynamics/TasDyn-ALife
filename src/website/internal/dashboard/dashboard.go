// Package dashboard loads the player dashboard (layout plan "Player —
// Dashboard"): greeting, stat tiles, houses, vehicles, licences, tickets
// and the opt-in leaderboards. Money is whole in-game dollars.
package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Overview struct {
	Name            string
	UID             string
	Since           time.Time
	DiscordUsername string

	Cash int64
	Bank int64

	Faction    string // headline: police/EMS rank name, or "Civilian"
	FactionSub string // e.g. "Police · also plays civilian"

	PlaytimeHours int64
	WeekHours     int64

	Banned      bool
	BanReason   string
	BanExpires  *time.Time
	OnlineCount int

	Houses   []House
	Vehicles []Vehicle
	Licences []string
	Tickets  []Ticket
	OptIn    bool

	Sides  []Side // civilian, police, EMS at a glance
	Bounty int64  // outstanding civilian bounty, 0 = not wanted
}

// Side is one of the player's three characters (civilian, police, EMS).
type Side struct {
	Key    string // civilian / police / ems
	Name   string // "Civilian", "Police", "EMS"
	Member bool   // civilian always; police/EMS when ranked
	Rank   string // police/EMS rank name
	Hours  int64
	Cash   int64
	Bank   int64
}

func (o Overview) NetWorth() int64 { return o.Cash + o.Bank }

type House struct {
	Name     string
	Price    int64
	Storage  int
	Unlocked bool
}

type Vehicle struct {
	Name  string
	Side  string // Civilian / Police / EMS
	Where string
	State string // Stored / Out / Impounded
}

type Ticket struct {
	ID      int64
	Subject string
	Label   string // "Staff replied", "Open", "In progress", "Closed"
	Class   string // for the badge colour
}

// Load reads one player's dashboard.
func Load(ctx context.Context, pool *pgxpool.Pool, playerID int64) (Overview, error) {
	var o Overview
	var civL, copL, medL []byte
	var civSecs, copSecs, medSecs int64
	var cop, medic int
	var copName, medName string
	var discord *string
	var civCash, copCash, medCash, civBank, copBank, medBank int64
	err := pool.QueryRow(ctx, `
		SELECT COALESCE(NULLIF(p.display_name, ''), NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id), p.uid, p.created_at, p.discord_username,
		       COALESCE(p.civ_cash, 0), COALESCE(p.cop_cash, 0), COALESCE(p.medic_cash, 0),
		       COALESCE((SELECT sum(balance) FROM bank_accounts WHERE player_id = p.id AND faction = 'civilian'), 0),
		       COALESCE((SELECT sum(balance) FROM bank_accounts WHERE player_id = p.id AND faction = 'police'), 0),
		       COALESCE((SELECT sum(balance) FROM bank_accounts WHERE player_id = p.id AND faction = 'medic'), 0),
		       COALESCE(p.cop_level, 0), COALESCE(p.medic_level, 0),
		       COALESCE((SELECT name FROM faction_rank_names WHERE faction = 'police' AND level = p.cop_level), ''),
		       COALESCE((SELECT name FROM faction_rank_names WHERE faction = 'ems' AND level = p.medic_level), ''),
		       COALESCE(p.civ_playtime_seconds, 0), COALESCE(p.cop_playtime_seconds, 0), COALESCE(p.medic_playtime_seconds, 0),
		       p.civ_licence, p.cop_licence, p.medic_licence, p.leaderboard_opt_in, COALESCE(p.civ_bounty, 0)
		FROM players p WHERE p.id = $1`, playerID).Scan(
		&o.Name, &o.UID, &o.Since, &discord, &civCash, &copCash, &medCash, &civBank, &copBank, &medBank,
		&cop, &medic, &copName, &medName,
		&civSecs, &copSecs, &medSecs, &civL, &copL, &medL, &o.OptIn, &o.Bounty)
	if err != nil {
		return o, err
	}
	o.Cash, o.Bank = civCash+copCash+medCash, civBank+copBank+medBank
	rankName := func(n string, lvl int) string {
		if n != "" {
			return n
		}
		return fmt.Sprintf("Level %d", lvl)
	}
	o.Sides = []Side{
		{Key: "civilian", Name: "Civilian", Member: true, Hours: civSecs / 3600, Cash: civCash, Bank: civBank},
		{Key: "police", Name: "Police", Member: cop > 0, Hours: copSecs / 3600, Cash: copCash, Bank: copBank},
		{Key: "ems", Name: "EMS", Member: medic > 0, Hours: medSecs / 3600, Cash: medCash, Bank: medBank},
	}
	if cop > 0 {
		o.Sides[1].Rank = rankName(copName, cop)
	}
	if medic > 0 {
		o.Sides[2].Rank = rankName(medName, medic)
	}
	if discord != nil {
		o.DiscordUsername = *discord
	}
	o.Faction, o.FactionSub = factionLine(cop, medic, copName, medName, civSecs)
	o.PlaytimeHours = (civSecs + copSecs + medSecs) / 3600
	o.Licences = mergeLicences(civL, copL, medL)

	// Hours this week, from session rows overlapping the last 7 days.
	var weekSecs float64
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(sum(EXTRACT(EPOCH FROM LEAST(COALESCE(disconnected_at, now()), now())
		                               - GREATEST(connected_at, now() - interval '7 days'))), 0)
		FROM player_sessions
		WHERE player_id = $1 AND COALESCE(disconnected_at, now()) > now() - interval '7 days'`, playerID).Scan(&weekSecs); err == nil {
		o.WeekHours = int64(weekSecs) / 3600
	}

	var reason *string
	err = pool.QueryRow(ctx, `
		SELECT reason, expires_at FROM banlist
		WHERE uid = $1 AND (expires_at IS NULL OR expires_at > now())
		ORDER BY expires_at DESC NULLS FIRST LIMIT 1`, o.UID).Scan(&reason, &o.BanExpires)
	if err == nil {
		o.Banned = true
		if reason != nil {
			o.BanReason = *reason
		}
	}

	_ = pool.QueryRow(ctx, `SELECT count(DISTINCT player_id) FROM player_sessions WHERE disconnected_at IS NULL`).Scan(&o.OnlineCount)

	rows, err := pool.Query(ctx, `
		SELECT house_key, price, COALESCE(jsonb_array_length(storage), 0), NOT locked
		FROM houses WHERE owner_player_id = $1 ORDER BY id`, playerID)
	if err != nil {
		return o, err
	}
	for rows.Next() {
		var h House
		if err := rows.Scan(&h.Name, &h.Price, &h.Storage, &h.Unlocked); err == nil {
			h.Name = humanize(h.Name)
			o.Houses = append(o.Houses, h)
		}
	}
	rows.Close()

	rows, err = pool.Query(ctx, `
		SELECT v.classname, COALESCE(v.faction_scope, 'civilian'), v.status, COALESCE(g.name, '')
		FROM vehicles v LEFT JOIN garages g ON g.id = v.garage_id
		WHERE v.owner_player_id = $1 AND v.status <> 'destroyed'
		ORDER BY v.faction_scope NULLS FIRST, v.id`, playerID)
	if err != nil {
		return o, err
	}
	for rows.Next() {
		var v Vehicle
		var scope, status, garage string
		if err := rows.Scan(&v.Name, &scope, &status, &garage); err != nil {
			continue
		}
		v.Name = humanize(v.Name)
		v.Side = map[string]string{"civilian": "Civilian", "police": "Police", "medic": "EMS"}[scope]
		switch status {
		case "garaged":
			v.State, v.Where = "Stored", garage
			if v.Where == "" {
				v.Where = "Garage"
			}
		case "spawned":
			v.State, v.Where = "Out", "In the world"
		case "impounded":
			v.State, v.Where = "Impounded", "Impound lot"
		}
		o.Vehicles = append(o.Vehicles, v)
	}
	rows.Close()

	rows, err = pool.Query(ctx, `
		SELECT t.id, t.subject, t.status,
		       COALESCE((SELECT m.author_player_id IS NOT NULL AND m.author_player_id <> t.player_id
		                 FROM support_ticket_messages m
		                 WHERE m.ticket_id = t.id AND NOT m.internal
		                 ORDER BY m.id DESC LIMIT 1), false)
		FROM support_tickets t WHERE t.player_id = $1
		ORDER BY (t.status = 'closed'), t.updated_at DESC LIMIT 4`, playerID)
	if err != nil {
		return o, err
	}
	for rows.Next() {
		var t Ticket
		var status string
		var staffLast bool
		if err := rows.Scan(&t.ID, &t.Subject, &status, &staffLast); err != nil {
			continue
		}
		switch {
		case status == "closed":
			t.Label, t.Class = "Closed", "closed"
		case staffLast:
			t.Label, t.Class = "Staff replied", "replied"
		case status == "pending":
			t.Label, t.Class = "In progress", "open"
		default:
			t.Label, t.Class = "Open", "open"
		}
		o.Tickets = append(o.Tickets, t)
	}
	rows.Close()
	return o, rows.Err()
}

func factionLine(cop, medic int, copName, medName string, civSecs int64) (string, string) {
	name := func(n string, lvl int) string {
		if n != "" {
			return n
		}
		return fmt.Sprintf("Level %d", lvl)
	}
	also := ""
	if civSecs > 0 {
		also = " · also plays civilian"
	}
	switch {
	case cop > 0 && medic > 0:
		return name(copName, cop), "Police and EMS" + also
	case cop > 0:
		return name(copName, cop), "Police" + also
	case medic > 0:
		return name(medName, medic), "EMS" + also
	}
	return "Civilian", "Apply to Police or EMS from the Factions page"
}

func mergeLicences(sets ...[]byte) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range sets {
		var l []string
		if len(raw) == 0 || json.Unmarshal(raw, &l) != nil {
			continue
		}
		for _, s := range l {
			if s != "" && !seen[s] {
				seen[s] = true
				out = append(out, humanize(s))
			}
		}
	}
	return out
}

// humanize turns a key like "kavala_townhouse" or "C_Offroad_01_F" into
// "Kavala townhouse" / "Offroad 01".
func humanize(key string) string {
	k := strings.TrimSpace(key)
	for _, p := range []string{"C_", "B_", "O_", "I_", "license_", "licence_"} {
		k = strings.TrimPrefix(k, p)
	}
	k = strings.TrimSuffix(k, "_F")
	k = strings.ReplaceAll(k, "_", " ")
	if k == "" {
		return key
	}
	return strings.ToUpper(k[:1]) + k[1:]
}

// ---- Leaderboards (opt-in; wealth as bands, never exact) ----

type Board struct {
	Title string
	Sub   string
	Rows  []BoardRow
	You   string
}

type BoardRow struct {
	Rank  string
	Name  string
	Value string
	Note  string // e.g. a rank name or gang tag, shown small
	You   bool
}

var bands = []struct {
	min   int64
	label string
}{
	{1_000_000, "$1M+"}, {500_000, "$500k–$1M"}, {250_000, "$250k–$500k"}, {100_000, "$100k–$250k"},
	{50_000, "$50k–$100k"}, {25_000, "$25k–$50k"}, {0, "Under $25k"},
}

// Band returns the wealth band index (0 = richest) and its label.
func Band(total int64) (int, string) {
	for i, b := range bands {
		if total >= b.min {
			return i, b.label
		}
	}
	return len(bands) - 1, bands[len(bands)-1].label
}

type entry struct {
	id    int64
	name  string
	score int64 // wealth band index (lower = better) or a count (higher = better)
	label string
}

// rankRows assigns competition ranks ("1", "2=", "2=", "4") to entries
// already sorted best-first, keeping ties together.
func rankRows(es []entry, same func(a, b entry) bool, youID int64, limit int) (rows []BoardRow, yourRank string) {
	for i := range es {
		rank := i + 1
		for rank > 1 && same(es[rank-2], es[i]) {
			rank--
		}
		tied := (i > 0 && same(es[i-1], es[i])) || (i+1 < len(es) && same(es[i+1], es[i]))
		r := fmt.Sprint(rank)
		if tied {
			r += "="
		}
		if es[i].id == youID {
			yourRank = r
		}
		if i < limit {
			rows = append(rows, BoardRow{Rank: r, Name: es[i].name, Value: es[i].label, You: es[i].id == youID})
		}
	}
	return rows, yourRank
}

// Leaderboards builds the three boards from opted-in players only.
func Leaderboards(ctx context.Context, pool *pgxpool.Pool, youID int64, youOptIn bool) ([]Board, error) {
	rows, err := pool.Query(ctx, `
		SELECT p.id, COALESCE(NULLIF(p.display_name, ''), NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id),
		       COALESCE(p.civ_cash, 0) + COALESCE(p.cop_cash, 0) + COALESCE(p.medic_cash, 0)
		         + COALESCE((SELECT sum(balance) FROM bank_accounts b WHERE b.player_id = p.id), 0),
		       (SELECT count(*) FROM houses h WHERE h.owner_player_id = p.id),
		       (SELECT count(*) FROM vehicles v WHERE v.owner_player_id = p.id AND v.status <> 'destroyed'
		          AND COALESCE(v.faction_scope, 'civilian') = 'civilian')
		FROM players p WHERE p.leaderboard_opt_in`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var wealth, houses, cars []entry
	var youBand string
	var youHouses, youCars int64
	for rows.Next() {
		var id, total, h, c int64
		var name string
		if err := rows.Scan(&id, &name, &total, &h, &c); err != nil {
			return nil, err
		}
		bi, bl := Band(total)
		wealth = append(wealth, entry{id, name, int64(bi), bl})
		if h > 0 {
			houses = append(houses, entry{id, name, h, plural(h, "house", "houses")})
		}
		if c > 0 {
			cars = append(cars, entry{id, name, c, plural(c, "vehicle", "vehicles")})
		}
		if id == youID {
			youBand, youHouses, youCars = bl, h, c
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(wealth, func(i, j int) bool {
		if wealth[i].score != wealth[j].score {
			return wealth[i].score < wealth[j].score
		}
		return strings.ToLower(wealth[i].name) < strings.ToLower(wealth[j].name)
	})
	byCount := func(es []entry) {
		sort.Slice(es, func(i, j int) bool {
			if es[i].score != es[j].score {
				return es[i].score > es[j].score
			}
			return strings.ToLower(es[i].name) < strings.ToLower(es[j].name)
		})
	}
	byCount(houses)
	byCount(cars)
	sameScore := func(a, b entry) bool { return a.score == b.score }

	wRows, _ := rankRows(wealth, sameScore, youID, 6)
	hRows, _ := rankRows(houses, sameScore, youID, 6)
	cRows, cRank := rankRows(cars, sameScore, youID, 6)

	hidden := "You're hidden. Tick the box above to appear."
	boards := []Board{
		{Title: "Richest players", Sub: "Cash + bank, in bands. Same band = alphabetical.", Rows: wRows, You: hidden},
		{Title: "Most houses", Sub: "Houses owned right now.", Rows: hRows, You: "You're hidden from this board."},
		{Title: "Most vehicles", Sub: "Personal vehicles only, faction vehicles don't count.", Rows: cRows, You: "You're hidden from this board."},
	}
	if youOptIn {
		boards[0].You = "You're in the " + youBand + " band."
		boards[1].You = "You own " + plural(youHouses, "house", "houses") + "."
		boards[2].You = "You have " + plural(youCars, "personal vehicle", "personal vehicles")
		if cRank != "" {
			boards[2].You += fmt.Sprintf(", rank %s of %d", strings.TrimSuffix(cRank, "="), len(cars))
		}
		boards[2].You += "."
	}
	return boards, nil
}

func plural(n int64, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// SetOptIn records whether the player appears on the leaderboards.
func SetOptIn(ctx context.Context, pool *pgxpool.Pool, playerID int64, on bool) error {
	_, err := pool.Exec(ctx, `UPDATE players SET leaderboard_opt_in = $2 WHERE id = $1`, playerID, on)
	return err
}
