package players

// The Player Lookup tabs beyond the overview: money (accounts and bank
// history), inventory (virtual items and the Arma loadout per faction),
// sessions, wanted / jail, staff notes, and editing licences.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
	"website/internal/gamenames"
)

// factions in display order: [players column prefix, bank_accounts.faction, label].
var factions = [][3]string{{"civ", "civilian", "Civilian"}, {"cop", "police", "Police"}, {"medic", "medic", "EMS"}}

// ---- Money ----

type Account struct {
	Side string
	Cash int64
	Bank int64
}

type BankTx struct {
	When    time.Time
	Side    string
	Type    string
	Amount  int64
	Balance int64
	Memo    string
	Other   string // the other player in a transfer
	By      string // who made it, when not the player (staff, the game)
}

// TypeLabel is bank_transactions.type in words.
func (t BankTx) TypeLabel() string {
	switch t.Type {
	case "transfer_in":
		return "Transfer in"
	case "transfer_out":
		return "Transfer out"
	case "admin_adjustment":
		return "Staff adjustment"
	}
	return strings.ToUpper(t.Type[:1]) + t.Type[1:]
}

// Money is a player's cash and bank per faction and their latest bank
// transactions (newest first, up to 200).
func Money(ctx context.Context, pool *pgxpool.Pool, id int64) ([]Account, []BankTx, error) {
	var cash [3]int64
	if err := pool.QueryRow(ctx, `SELECT civ_cash, cop_cash, medic_cash FROM players WHERE id = $1`, id).Scan(&cash[0], &cash[1], &cash[2]); err != nil {
		return nil, nil, err
	}
	bank := map[string]int64{}
	rows, err := pool.Query(ctx, `SELECT faction, balance FROM bank_accounts WHERE player_id = $1`, id)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var f string
		var b int64
		if err := rows.Scan(&f, &b); err != nil {
			rows.Close()
			return nil, nil, err
		}
		bank[f] = b
	}
	rows.Close()
	var accounts []Account
	for i, f := range factions {
		accounts = append(accounts, Account{Side: f[2], Cash: cash[i], Bank: bank[f[1]]})
	}

	rows, err = pool.Query(ctx, `
		SELECT t.created_at, a.faction, t.type, t.amount, t.balance_after, COALESCE(t.memo, ''),
		       COALESCE(`+nameOf("op")+`, ''), CASE WHEN t.created_by IS NULL OR t.created_by = a.player_id THEN '' ELSE COALESCE(`+nameOf("cb")+`, '') END
		FROM bank_transactions t
		JOIN bank_accounts a ON a.id = t.account_id
		LEFT JOIN bank_accounts oa ON oa.id = t.related_account_id
		LEFT JOIN players op ON op.id = oa.player_id
		LEFT JOIN players cb ON cb.id = t.created_by
		WHERE a.player_id = $1
		ORDER BY t.created_at DESC, t.id DESC LIMIT 200`, id)
	if err != nil {
		return accounts, nil, err
	}
	defer rows.Close()
	var txs []BankTx
	for rows.Next() {
		var t BankTx
		if err := rows.Scan(&t.When, &t.Side, &t.Type, &t.Amount, &t.Balance, &t.Memo, &t.Other, &t.By); err != nil {
			return accounts, nil, err
		}
		t.Side = sideLabel(t.Side)
		txs = append(txs, t)
	}
	return accounts, txs, rows.Err()
}

func nameOf(alias string) string {
	return `COALESCE(NULLIF(` + alias + `.name, ''), NULLIF(` + alias + `.steam_name, ''), 'Player #' || ` + alias + `.id)`
}

func sideLabel(f string) string {
	for _, x := range factions {
		if x[0] == f || x[1] == f {
			return x[2]
		}
	}
	return f
}

// ---- Inventory ----

type Stack struct {
	Name  string
	Count int64
}

type Slot struct {
	Label string   // "Primary", "Uniform", ...
	Name  string   // the item
	Extra []string // attachments, loaded magazine
	Items []Stack  // what's in it (uniform / vest / backpack)
}

type Inventory struct {
	Side  string
	Alive bool
	Items []Stack // virtual items (CfgItems)
	Slots []Slot  // the Arma loadout
	Links []string
	Food  int
	Water int
	Saved bool // false = nothing saved for this faction yet
}

// Inventories is each faction's saved inventory: virtual items and the
// Arma loadout (players.<faction>_items / _gear), as of the last save.
func Inventories(ctx context.Context, pool *pgxpool.Pool, id int64) ([]Inventory, error) {
	var raw [3][2][]byte
	var alive [3]bool
	err := pool.QueryRow(ctx, `
		SELECT civ_items, civ_gear, civ_alive, cop_items, cop_gear, cop_alive, medic_items, medic_gear, medic_alive
		FROM players WHERE id = $1`, id).Scan(&raw[0][0], &raw[0][1], &alive[0], &raw[1][0], &raw[1][1], &alive[1], &raw[2][0], &raw[2][1], &alive[2])
	if err != nil {
		return nil, err
	}
	var out []Inventory
	for i, f := range factions {
		inv := Inventory{Side: f[2], Alive: alive[i], Items: vItems(raw[i][0])}
		parseGear(raw[i][1], &inv)
		inv.Saved = len(inv.Items) > 0 || len(inv.Slots) > 0
		out = append(out, inv)
	}
	return out, nil
}

// vItems reads [[class, count], ...].
func vItems(raw []byte) []Stack {
	var pairs [][]json.RawMessage
	if json.Unmarshal(raw, &pairs) != nil {
		return nil
	}
	var out []Stack
	for _, p := range pairs {
		if len(p) < 2 {
			continue
		}
		var k string
		var n float64
		if json.Unmarshal(p[0], &k) != nil || json.Unmarshal(p[1], &n) != nil || n <= 0 {
			continue
		}
		out = append(out, Stack{Name: gamenames.Item(k), Count: int64(n)})
	}
	return out
}

// parseGear reads <faction>_gear: [["loadout", getUnitLoadout], ["food", n],
// ["water", n]].
func parseGear(raw []byte, inv *Inventory) {
	var pairs [][]json.RawMessage
	if json.Unmarshal(raw, &pairs) != nil {
		return
	}
	for _, p := range pairs {
		if len(p) < 2 {
			continue
		}
		var key string
		if json.Unmarshal(p[0], &key) != nil {
			continue
		}
		switch key {
		case "food", "water":
			var n float64
			if json.Unmarshal(p[1], &n) == nil {
				if key == "food" {
					inv.Food = int(n + 0.5)
				} else {
					inv.Water = int(n + 0.5)
				}
			}
		case "loadout":
			parseLoadout(p[1], inv)
		}
	}
}

// parseLoadout reads an Arma getUnitLoadout array:
// [primary, launcher, handgun, uniform, vest, backpack, headgear, goggles,
// binoculars, [map, gps, radio, compass, watch, nvg]].
func parseLoadout(raw json.RawMessage, inv *Inventory) {
	var l []json.RawMessage
	if json.Unmarshal(raw, &l) != nil {
		return
	}
	at := func(i int) json.RawMessage {
		if i < len(l) {
			return l[i]
		}
		return nil
	}
	weapon := func(label string, r json.RawMessage) {
		var w []json.RawMessage
		if json.Unmarshal(r, &w) != nil || len(w) == 0 {
			return
		}
		var class string
		if json.Unmarshal(w[0], &class) != nil || class == "" {
			return
		}
		s := Slot{Label: label, Name: gamenames.Gear(class)}
		for _, part := range w[1:] {
			var att string
			if json.Unmarshal(part, &att) == nil {
				if att != "" {
					s.Extra = append(s.Extra, gamenames.Gear(att))
				}
				continue
			}
			var mag []json.RawMessage // [magazine, rounds]
			if json.Unmarshal(part, &mag) == nil && len(mag) >= 2 {
				var m string
				var n float64
				if json.Unmarshal(mag[0], &m) == nil && m != "" && json.Unmarshal(mag[1], &n) == nil {
					s.Extra = append(s.Extra, fmt.Sprintf("%s (%d rounds)", gamenames.Gear(m), int(n)))
				}
			}
		}
		inv.Slots = append(inv.Slots, s)
	}
	container := func(label string, r json.RawMessage) {
		var c []json.RawMessage
		if json.Unmarshal(r, &c) != nil || len(c) == 0 {
			return
		}
		var class string
		if json.Unmarshal(c[0], &class) != nil || class == "" {
			return
		}
		s := Slot{Label: label, Name: gamenames.Gear(class)}
		if len(c) > 1 {
			var items [][]json.RawMessage
			if json.Unmarshal(c[1], &items) == nil {
				for _, it := range items {
					if len(it) < 2 {
						continue
					}
					var name string
					var n float64
					if json.Unmarshal(it[0], &name) != nil {
						// A weapon inside: [[weapon, ...], count].
						var w []json.RawMessage
						if json.Unmarshal(it[0], &w) != nil || len(w) == 0 || json.Unmarshal(w[0], &name) != nil {
							continue
						}
					}
					if json.Unmarshal(it[1], &n) != nil || name == "" {
						continue
					}
					s.Items = append(s.Items, Stack{Name: gamenames.Gear(name), Count: int64(n)})
				}
			}
		}
		inv.Slots = append(inv.Slots, s)
	}
	single := func(label string, r json.RawMessage) {
		var class string
		if json.Unmarshal(r, &class) == nil && class != "" {
			inv.Slots = append(inv.Slots, Slot{Label: label, Name: gamenames.Gear(class)})
		}
	}
	weapon("Primary", at(0))
	weapon("Launcher", at(1))
	weapon("Handgun", at(2))
	container("Uniform", at(3))
	container("Vest", at(4))
	container("Backpack", at(5))
	single("Headgear", at(6))
	single("Facewear", at(7))
	weapon("Binoculars", at(8))
	var linked []string
	if json.Unmarshal(at(9), &linked) == nil {
		for _, c := range linked {
			if c != "" {
				inv.Links = append(inv.Links, gamenames.Gear(c))
			}
		}
	}
}

// ---- Sessions ----

type Session struct {
	Start time.Time
	End   *time.Time
}

func (s Session) Length() time.Duration {
	end := time.Now()
	if s.End != nil {
		end = *s.End
	}
	return end.Sub(s.Start).Round(time.Minute)
}

type Playtime struct {
	Side string
	Time time.Duration
}

type SessionInfo struct {
	Playtime []Playtime
	Sessions []Session
	Last30   time.Duration // time online in the last 30 days
	Count30  int
	Online   *time.Time // connected since, nil = offline
}

// Sessions is a player's playtime per faction and connection history
// (newest first, up to 100). IP addresses aren't shown.
func Sessions(ctx context.Context, pool *pgxpool.Pool, id int64) (SessionInfo, error) {
	var si SessionInfo
	var secs [3]int64
	if err := pool.QueryRow(ctx, `SELECT civ_playtime_seconds, cop_playtime_seconds, medic_playtime_seconds FROM players WHERE id = $1`, id).
		Scan(&secs[0], &secs[1], &secs[2]); err != nil {
		return si, err
	}
	for i, f := range factions {
		si.Playtime = append(si.Playtime, Playtime{Side: f[2], Time: time.Duration(secs[i]) * time.Second})
	}
	var last30 float64
	if err := pool.QueryRow(ctx, `
		SELECT count(*), COALESCE(sum(EXTRACT(EPOCH FROM (COALESCE(disconnected_at, now()) - GREATEST(connected_at, now() - interval '30 days')))), 0)
		FROM player_sessions WHERE player_id = $1 AND COALESCE(disconnected_at, now()) > now() - interval '30 days'`, id).
		Scan(&si.Count30, &last30); err != nil {
		return si, err
	}
	si.Last30 = (time.Duration(last30) * time.Second).Round(time.Minute)
	rows, err := pool.Query(ctx, `SELECT connected_at, disconnected_at FROM player_sessions WHERE player_id = $1 ORDER BY connected_at DESC LIMIT 100`, id)
	if err != nil {
		return si, err
	}
	defer rows.Close()
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.Start, &s.End); err != nil {
			return si, err
		}
		if s.End == nil && si.Online == nil {
			start := s.Start
			si.Online = &start
		}
		si.Sessions = append(si.Sessions, s)
	}
	return si, rows.Err()
}

// ---- Wanted & jail ----

type Crime struct {
	Name   string
	Bounty int64
	By     string
	When   time.Time
}

type Wanted struct {
	Bounty int64
	Crimes []Crime
	Jail   time.Duration // time left to serve, 0 = free
	Bail   int64
}

// WantedInfo is a player's outstanding crimes (wanted_crimes), bounty and
// jail time left / bail. Jail columns arrived with the jail fix script;
// before that they read as free.
func WantedInfo(ctx context.Context, pool *pgxpool.Pool, id int64) (Wanted, error) {
	var w Wanted
	var jail int64
	err := pool.QueryRow(ctx, `SELECT civ_bounty, civ_jail_seconds, civ_bail FROM players WHERE id = $1`, id).Scan(&w.Bounty, &jail, &w.Bail)
	if err != nil {
		return w, err
	}
	w.Jail = time.Duration(jail) * time.Second
	rows, err := pool.Query(ctx, `
		SELECT w.crime_key, w.bounty_amount, COALESCE(`+nameOf("i")+`, ''), w.created_at
		FROM wanted_crimes w LEFT JOIN players i ON i.id = w.issued_by
		WHERE w.player_id = $1 AND w.cleared_at IS NULL ORDER BY w.created_at DESC`, id)
	if err != nil {
		return w, err
	}
	defer rows.Close()
	for rows.Next() {
		var c Crime
		var key string
		if err := rows.Scan(&key, &c.Bounty, &c.By, &c.When); err != nil {
			return w, err
		}
		c.Name = gamenames.Crime(key)
		w.Crimes = append(w.Crimes, c)
	}
	return w, rows.Err()
}

// ---- Staff notes ----

type Note struct {
	Body   string
	Author string
	When   time.Time
}

// Notes are staff notes about a player (staff_notes kind 'player'), newest
// first. Never edited or deleted.
func Notes(ctx context.Context, pool *pgxpool.Pool, id int64) ([]Note, error) {
	rows, err := pool.Query(ctx, `
		SELECT n.body, COALESCE(`+nameOf("a")+`, 'Unknown'), n.created_at
		FROM staff_notes n LEFT JOIN players a ON a.id = n.author_id
		WHERE n.player_id = $1 AND n.kind = 'player' ORDER BY n.created_at DESC LIMIT 200`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.Body, &n.Author, &n.When); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// AddNote adds a staff note about a player. The caller checks players.notes.
func AddNote(ctx context.Context, pool *pgxpool.Pool, actor Actor, targetID int64, body string) error {
	body = strings.TrimSpace(body)
	if body == "" || len(body) > 4000 {
		return notAllowed("a note must be between 1 and 4000 characters")
	}
	if actor.PlayerID == targetID {
		return notAllowed("you can't add notes about yourself")
	}
	_, err := pool.Exec(ctx, `INSERT INTO staff_notes (player_id, author_id, kind, body) VALUES ($1, $2, 'player', $3)`, targetID, actor.PlayerID, body)
	return err
}

// ---- Licences ----

// LicenceChoice is one licence for the edit dialog.
type LicenceChoice struct {
	Key  string
	Name string
	Has  bool
}

type LicenceGroup struct {
	Faction string // "civ", "cop", "medic"
	Side    string
	Choices []LicenceChoice
}

// LicenceGroups is every licence per faction (CfgLicences), ticked where
// the player has it.
func LicenceGroups(ctx context.Context, pool *pgxpool.Pool, id int64) ([]LicenceGroup, error) {
	var raw [3][]byte
	if err := pool.QueryRow(ctx, `SELECT civ_licence, cop_licence, medic_licence FROM players WHERE id = $1`, id).Scan(&raw[0], &raw[1], &raw[2]); err != nil {
		return nil, err
	}
	var out []LicenceGroup
	for i, f := range factions {
		var have []string
		_ = json.Unmarshal(raw[i], &have)
		g := LicenceGroup{Faction: f[0], Side: f[2]}
		for _, l := range gamenames.Licences(f[0]) {
			g.Choices = append(g.Choices, LicenceChoice{Key: l.Key, Name: l.Name, Has: slices.Contains(have, l.Key)})
		}
		out = append(out, g)
	}
	return out, nil
}

// SetLicences replaces a player's licences for one faction with keys (only
// CfgLicences keys for that faction; any others they hold are kept). Only
// while they're offline: the game holds their licences in memory while
// they play and would write its copy back. The caller checks
// players.licences. Returns what was added and removed.
func SetLicences(ctx context.Context, pool *pgxpool.Pool, actor Actor, targetID int64, faction string, keys []string) (added, removed []string, err error) {
	col := map[string]string{"civ": "civ_licence", "cop": "cop_licence", "medic": "medic_licence"}[faction]
	if col == "" {
		return nil, nil, notAllowed("unknown faction")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)
	var raw []byte
	var online bool
	err = tx.QueryRow(ctx, `
		SELECT `+col+`, EXISTS (SELECT 1 FROM player_sessions s WHERE s.player_id = p.id AND s.disconnected_at IS NULL)
		FROM players p WHERE p.id = $1 FOR UPDATE`, targetID).Scan(&raw, &online)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, notAllowed("that player doesn't exist")
	}
	if err != nil {
		return nil, nil, err
	}
	if online {
		return nil, nil, notAllowed("they're in game right now: use the in-game admin menu (Give / take licence), or wait until they log off")
	}
	var before []string
	_ = json.Unmarshal(raw, &before)
	want := map[string]bool{}
	for _, k := range keys {
		if gamenames.IsLicence(faction, k) {
			want[k] = true
		}
	}
	var after []string
	for _, k := range before {
		if !gamenames.IsLicence(faction, k) || want[k] { // keep unknown keys as they are
			after = append(after, k)
		} else {
			removed = append(removed, k)
		}
	}
	for _, l := range gamenames.Licences(faction) {
		if want[l.Key] && !slices.Contains(before, l.Key) {
			after = append(after, l.Key)
			added = append(added, l.Key)
		}
	}
	if len(added) == 0 && len(removed) == 0 {
		return nil, nil, notAllowed("nothing changed")
	}
	if after == nil {
		after = []string{}
	}
	js, _ := json.Marshal(after)
	if _, err := tx.Exec(ctx, `UPDATE players SET `+col+` = $2 WHERE id = $1`, targetID, js); err != nil {
		return nil, nil, err
	}
	names := func(ks []string) string {
		var n []string
		for _, k := range ks {
			n = append(n, gamenames.Licence(k))
		}
		return strings.Join(n, ", ")
	}
	var parts []string
	if len(added) > 0 {
		parts = append(parts, "gave "+names(added))
	}
	if len(removed) > 0 {
		parts = append(parts, "took "+names(removed))
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actor.PlayerID, TargetID: targetID, Action: "player.licences", Source: actor.Source,
		Reason: sideLabel(faction) + ": " + strings.Join(parts, "; "),
		Before: map[string]any{"faction": faction, "licences": before},
		After:  map[string]any{"faction": faction, "licences": after},
	}); err != nil {
		return nil, nil, err
	}
	return added, removed, tx.Commit(ctx)
}
