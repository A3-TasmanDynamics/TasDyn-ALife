// Package players backs the Admin Panel's Player Lookup (the layout plan's
// Player Lookup board, GAMEPANEL_PARITY #20-#24): search, profile, vehicles
// and audit trail, plus the two write actions on that page -- setting a
// police/EMS level and compensating a player. Both writes go through the
// same logged paths as everything else: level changes are attributed with
// audit.SetActor (the change trigger logs them), compensation is a
// bank_transactions row plus a staff_log row in one transaction.
package players

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
)

var ErrNotAllowed = errors.New("not allowed")

func notAllowed(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrNotAllowed, fmt.Sprintf(format, args...))
}

// LargeCompensationCents is the amount above which compensation needs
// players.compensate_large ($10,000).
const LargeCompensationCents = 10_000_00

type Result struct {
	ID     int64
	Name   string
	Hint   string // what matched / Steam64
	Status string // "online", "offline", "banned"
}

const nameExpr = `COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id)`

const statusExpr = `
	CASE
		WHEN EXISTS (SELECT 1 FROM banlist b WHERE b.uid = p.uid AND (b.expires_at IS NULL OR b.expires_at > now())) THEN 'banned'
		WHEN EXISTS (SELECT 1 FROM player_sessions s WHERE s.player_id = p.id AND s.disconnected_at IS NULL) THEN 'online'
		ELSE 'offline'
	END`

// Search finds players by current name, past names (player_aliases),
// Steam name, Steam64 ID, BattlEye GUID or player id. Up to 25 results.
func Search(ctx context.Context, pool *pgxpool.Pool, q string) ([]Result, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		rows, err := pool.Query(ctx, `
			SELECT p.id, `+nameExpr+`, p.uid, `+statusExpr+`
			FROM players p ORDER BY p.last_seen DESC NULLS LAST, p.id DESC LIMIT 25`)
		return collect(rows, err, func(r *Result, uid string) { r.Hint = uid })
	}
	like := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
	id, _ := strconv.ParseInt(q, 10, 64)
	rows, err := pool.Query(ctx, `
		SELECT p.id, `+nameExpr+`, p.uid, `+statusExpr+`,
		       (SELECT string_agg(a.name, ', ' ORDER BY a.last_seen_at DESC) FROM player_aliases a
		         WHERE a.player_id = p.id AND a.name ILIKE $1 AND a.name IS DISTINCT FROM p.name)
		FROM players p
		WHERE p.name ILIKE $1 OR p.steam_name ILIKE $1 OR p.uid = $2 OR p.be_guid = lower($2) OR p.id = $3
		   OR EXISTS (SELECT 1 FROM player_aliases a WHERE a.player_id = p.id AND a.name ILIKE $1)
		ORDER BY (p.uid = $2 OR p.id = $3) DESC, p.last_seen DESC NULLS LAST, p.id DESC
		LIMIT 25`, like, q, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Result
	for rows.Next() {
		var r Result
		var uid string
		var alias *string
		if err := rows.Scan(&r.ID, &r.Name, &uid, &r.Status, &alias); err != nil {
			return nil, err
		}
		r.Hint = uid
		if alias != nil {
			r.Hint = "was " + *alias
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func collect(rows pgx.Rows, err error, hint func(*Result, string)) ([]Result, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Result
	for rows.Next() {
		var r Result
		var uid string
		if err := rows.Scan(&r.ID, &r.Name, &uid, &r.Status); err != nil {
			return nil, err
		}
		hint(&r, uid)
		out = append(out, r)
	}
	return out, rows.Err()
}

type Profile struct {
	ID          int64
	UID         string
	BEGUID      string
	Name        string
	AvatarURL   string
	Status      string
	Aliases     []string
	StaffRank   string
	StaffID     int64 // == ID when staff, for the profile link
	Joined      time.Time
	LastSeen    *time.Time
	Playtime    time.Duration
	CashCents   int64
	BankCents   int64
	CopLevel    int
	CopName     string
	MedicLevel  int
	MedicName   string
	GangName    string
	GangRank    string
	Licences    []string
	Discord     string
	SteamVAC    *int
	SteamGame   *int
	SteamAge    *time.Time
	BanReason   string
	BanExpires  *time.Time
	BanActive   bool
}

func (p Profile) TotalCents() int64 { return p.CashCents + p.BankCents }

// Get loads one player's profile.
func Get(ctx context.Context, pool *pgxpool.Pool, id int64) (Profile, error) {
	var p Profile
	var civL, copL, medL []byte
	var playSecs int64
	err := pool.QueryRow(ctx, `
		SELECT p.id, p.uid, COALESCE(p.be_guid, ''), `+nameExpr+`, COALESCE(p.steam_avatar_url, ''), `+statusExpr+`,
		       COALESCE(sr.display_name, ''), p.created_at, p.last_seen,
		       COALESCE(p.civ_playtime_seconds, 0) + COALESCE(p.cop_playtime_seconds, 0) + COALESCE(p.medic_playtime_seconds, 0),
		       COALESCE(p.civ_cash, 0) + COALESCE(p.cop_cash, 0) + COALESCE(p.medic_cash, 0),
		       COALESCE((SELECT sum(balance) FROM bank_accounts WHERE player_id = p.id), 0),
		       COALESCE(p.cop_level, 0), COALESCE(p.medic_level, 0),
		       COALESCE((SELECT name FROM faction_rank_names WHERE faction = 'police' AND level = p.cop_level), ''),
		       COALESCE((SELECT name FROM faction_rank_names WHERE faction = 'ems' AND level = p.medic_level), ''),
		       COALESCE(g.name, ''), COALESCE(gm.rank, ''),
		       p.civ_licence, p.cop_licence, p.medic_licence,
		       COALESCE(p.discord_username, ''), p.steam_vac_bans, p.steam_game_bans, p.steam_created_at
		FROM players p
		LEFT JOIN staff_ranks sr ON sr.id = p.staff_rank_id
		LEFT JOIN gang_members gm ON gm.player_id = p.id
		LEFT JOIN gangs g ON g.id = gm.gang_id
		WHERE p.id = $1`, id).Scan(
		&p.ID, &p.UID, &p.BEGUID, &p.Name, &p.AvatarURL, &p.Status,
		&p.StaffRank, &p.Joined, &p.LastSeen, &playSecs, &p.CashCents, &p.BankCents,
		&p.CopLevel, &p.MedicLevel, &p.CopName, &p.MedicName, &p.GangName, &p.GangRank,
		&civL, &copL, &medL, &p.Discord, &p.SteamVAC, &p.SteamGame, &p.SteamAge)
	if err != nil {
		return p, err
	}
	if p.StaffRank != "" {
		p.StaffID = p.ID
	}
	p.Playtime = time.Duration(playSecs) * time.Second
	p.Licences = mergeLicences(civL, copL, medL)

	rows, err := pool.Query(ctx, `SELECT name FROM player_aliases WHERE player_id = $1 AND name IS DISTINCT FROM $2 ORDER BY last_seen_at DESC LIMIT 10`, id, p.Name)
	if err == nil {
		for rows.Next() {
			var a string
			if rows.Scan(&a) == nil {
				p.Aliases = append(p.Aliases, a)
			}
		}
		rows.Close()
	}

	err = pool.QueryRow(ctx, `
		SELECT reason, expires_at, (expires_at IS NULL OR expires_at > now())
		FROM banlist WHERE uid = $1 ORDER BY created_at DESC LIMIT 1`, p.UID).Scan(&p.BanReason, &p.BanExpires, &p.BanActive)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return p, err
	}
	return p, nil
}

// mergeLicences unions the three per-faction licence sets (DATA_CONTRACT:
// JSON arrays of strings). Anything that isn't a string array is ignored
// rather than shown wrongly.
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
				out = append(out, s)
			}
		}
	}
	return out
}

type Vehicle struct {
	Name   string // classname + plate
	Side   string
	Where  string
	Status string
}

type VehicleEvent struct {
	When time.Time
	What string
}

// Vehicles lists a player's vehicles and their recent vehicle_logs.
func Vehicles(ctx context.Context, pool *pgxpool.Pool, id int64) ([]Vehicle, []VehicleEvent, error) {
	rows, err := pool.Query(ctx, `
		SELECT v.classname || CASE WHEN v.plate IS NOT NULL AND v.plate <> '' THEN ' · ' || v.plate ELSE '' END,
		       COALESCE(v.faction_scope, 'civilian'), COALESCE(gr.name, CASE v.status WHEN 'spawned' THEN 'Out in the world' ELSE '—' END), v.status
		FROM vehicles v LEFT JOIN garages gr ON gr.id = v.garage_id
		WHERE v.owner_player_id = $1 ORDER BY v.status, v.classname`, id)
	if err != nil {
		return nil, nil, err
	}
	var vs []Vehicle
	for rows.Next() {
		var v Vehicle
		if err := rows.Scan(&v.Name, &v.Side, &v.Where, &v.Status); err != nil {
			rows.Close()
			return nil, nil, err
		}
		vs = append(vs, v)
	}
	rows.Close()

	rows, err = pool.Query(ctx, `
		SELECT vl.created_at, vl.action || ' · ' || v.classname ||
		       CASE WHEN vl.amount IS NOT NULL AND vl.amount <> 0 THEN ' · $' || (vl.amount / 100)::text ELSE '' END
		FROM vehicle_logs vl JOIN vehicles v ON v.id = vl.vehicle_id
		WHERE vl.player_id = $1 ORDER BY vl.created_at DESC LIMIT 20`, id)
	if err != nil {
		return vs, nil, err
	}
	defer rows.Close()
	var evs []VehicleEvent
	for rows.Next() {
		var e VehicleEvent
		if err := rows.Scan(&e.When, &e.What); err != nil {
			return vs, nil, err
		}
		evs = append(evs, e)
	}
	return vs, evs, rows.Err()
}

type AuditEvent struct {
	When   time.Time
	Source string // "Game", "Staff", "Kick", "Anti-cheat", "Bank"
	What   string
	By     string
}

// Audit merges everything recorded about a player, newest first: game
// events (player_log), staff actions targeting them (staff_log), kicks,
// anti-cheat flags and bank adjustments.
func Audit(ctx context.Context, pool *pgxpool.Pool, id int64) ([]AuditEvent, error) {
	rows, err := pool.Query(ctx, `
		SELECT created_at, 'Game', event_type, '', NULL::text, NULL::jsonb, NULL::jsonb FROM player_log WHERE player_id = $1
		UNION ALL
		SELECT sl.created_at, 'Staff', sl.action, COALESCE(NULLIF(s.name, ''), NULLIF(s.steam_name, ''), 'Player #' || s.id, sl.source),
		       sl.reason, sl.before_value, sl.after_value
		FROM staff_log sl LEFT JOIN players s ON s.id = sl.staff_player_id WHERE sl.target_player_id = $1
		UNION ALL
		SELECT k.created_at, 'Kick', k.kick_type || COALESCE(': ' || k.reason, ''), COALESCE(NULLIF(s.name, ''), 'System'), NULL, NULL, NULL
		FROM kick_log k LEFT JOIN players s ON s.id = k.kicked_by WHERE k.target_player_id = $1
		UNION ALL
		SELECT f.created_at, 'Anti-cheat', f.flag_type || ' (' || f.confidence || ')' || COALESCE(' → ' || f.resolution, ''), '', NULL, NULL, NULL
		FROM anti_cheat_flags f WHERE f.player_id = $1
		ORDER BY 1 DESC LIMIT 100`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEvent
	for rows.Next() {
		var e AuditEvent
		var reason *string
		var before, after []byte
		if err := rows.Scan(&e.When, &e.Source, &e.What, &e.By, &reason, &before, &after); err != nil {
			return nil, err
		}
		if e.Source == "Staff" {
			e.What = audit.Describe(e.What, before, after)
			if reason != nil && *reason != "" {
				e.What += ": " + *reason
			}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Actor is who performs a write, and from where.
type Actor struct {
	PlayerID int64
	Source   string
}

// SetFactionLevel sets a player's police ("police") or EMS ("ems") level.
// The caller checks players.edit_police / players.edit_medic.
func SetFactionLevel(ctx context.Context, pool *pgxpool.Pool, actor Actor, targetID int64, faction string, level int, reason string) error {
	col := map[string]string{"police": "cop_level", "ems": "medic_level"}[faction]
	if col == "" {
		return notAllowed("unknown faction")
	}
	if level < 0 || level > 20 {
		return notAllowed("the level must be between 0 and 20")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return notAllowed("a reason is required")
	}
	if actor.PlayerID == targetID {
		return notAllowed("you can't change your own faction level")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := audit.SetActor(ctx, tx, actor.PlayerID, actor.Source, reason); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE players SET `+col+` = $2 WHERE id = $1 AND `+col+` IS DISTINCT FROM $2`, targetID, level)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return notAllowed("they're already at that level")
	}
	return tx.Commit(ctx)
}

// Compensate adds amountCents to one of the player's bank accounts as an
// admin_adjustment transaction (the balance follows via the existing
// trigger -- never a direct overwrite) and logs it. The caller checks
// players.compensate, and players.compensate_large above the threshold.
func Compensate(ctx context.Context, pool *pgxpool.Pool, actor Actor, targetID int64, faction string, amountCents int64, reason string) error {
	if faction != "civilian" && faction != "police" && faction != "medic" {
		return notAllowed("unknown account")
	}
	if amountCents <= 0 || amountCents > 100_000_000_00 {
		return notAllowed("the amount must be positive")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 300 {
		return notAllowed("a reason (up to 300 characters) is required")
	}
	if actor.PlayerID == targetID {
		return notAllowed("you can't compensate yourself")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var accountID, balance int64
	err = tx.QueryRow(ctx, `SELECT id, balance FROM bank_accounts WHERE player_id = $1 AND faction = $2 FOR UPDATE`,
		targetID, faction).Scan(&accountID, &balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return notAllowed("that player has no %s bank account yet", faction)
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO bank_transactions (account_id, type, amount, balance_after, memo, created_by)
		VALUES ($1, 'admin_adjustment', $2, $3, $4, $5)`,
		accountID, amountCents, balance+amountCents, "Compensation: "+reason, actor.PlayerID); err != nil {
		return err
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actor.PlayerID, TargetID: targetID, Action: "player.compensate", Source: actor.Source,
		Reason: fmt.Sprintf("+$%s %s bank: %s", dollars(amountCents), faction, reason),
		Before: map[string]any{"account": faction, "balance_cents": balance},
		After:  map[string]any{"account": faction, "balance_cents": balance + amountCents, "amount_cents": amountCents},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// dollars formats cents as "1,234.50" (or "1,234" when whole).
func dollars(cents int64) string {
	neg := cents < 0
	if neg {
		cents = -cents
	}
	whole := strconv.FormatInt(cents/100, 10)
	var b strings.Builder
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	s := b.String()
	if r := cents % 100; r != 0 {
		s += fmt.Sprintf(".%02d", r)
	}
	if neg {
		s = "-" + s
	}
	return s
}

// Dollars is dollars exported for templates.
func Dollars(cents int64) string { return dollars(cents) }
