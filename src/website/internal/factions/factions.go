// Package factions is the one place police/EMS levels change on the
// website, whoever makes the change (docs/GAMEPANEL_PARITY.md §5.2, §6.1):
//
//   - Faction command, on the command panel. Access comes from the
//     officer's faction rank being ticked as a command rank
//     (faction_rank_names.is_command), never from a staff role. How far
//     they can promote is the rank's promote_up_to: they can only act on
//     members whose current AND new level are within it, which is always
//     below their own rank.
//   - Management, as a staff override from Player Lookup (permissions
//     players.edit_police / players.edit_medic, Head Admin only and never
//     granted per-player). Meant for fixing mistakes and settling disputes.
//
// Conflict-of-interest rules, for both: nobody changes their own level; an
// override by a staff member who is in that faction is allowed but flagged
// "own faction"; and every change records which authority it was made under
// (via command / via staff override) in faction_log, the Command log.
package factions

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
)

// ErrNotAllowed wraps a rule violation with a human-readable reason.
var ErrNotAllowed = errors.New("not allowed")

func notAllowed(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrNotAllowed, fmt.Sprintf(format, args...))
}

// Factions are "police" and "ems" (players.cop_level / medic_level).
var Factions = []string{"police", "ems"}

var (
	column = map[string]string{"police": "cop_level", "ems": "medic_level"}
	names  = map[string]string{"police": "Police", "ems": "EMS"}
)

// Valid reports whether f is a known faction.
func Valid(f string) bool { return column[f] != "" }

// Name is the faction's display name ("Police", "EMS").
func Name(f string) string { return names[f] }

// Via says which authority a change was made under.
const (
	ViaCommand       = "command"
	ViaStaffOverride = "staff_override"
)

// Rank is one configured faction rank.
type Rank struct {
	Level       int
	Name        string
	Short       string
	Slots       int  // 0 = no limit
	PromoteUpTo int  // highest level it may set others to (0 = none)
	IsCommand   bool // ticked: gets the command panel (CMD)
	IsCabinet   bool // ticked: senior leadership (CAB)

	// Rank rules (Ranks & gear).
	MinDays     int      // days in this rank before promotion
	Quals       []string // qualifications needed to hold it
	Description string
}

// Command reports whether the rank is ticked as a command rank.
func (r Rank) Command() bool { return r.IsCommand }

// CanPromote reports whether the rank can change others' ranks.
func (r Rank) CanPromote() bool { return r.IsCommand && r.PromoteUpTo > 0 && r.PromoteUpTo < r.Level }

// Label is "Name" or "Level n" when the rank has no name configured.
func (r Rank) Label() string {
	if r.Name != "" {
		return r.Name
	}
	return fmt.Sprintf("Level %d", r.Level)
}

// Ranks returns a faction's configured ranks, lowest first.
func Ranks(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, faction string) ([]Rank, error) {
	rows, err := q.Query(ctx, `
		SELECT level, name, COALESCE(short_name, ''), COALESCE(slots, 0), promote_up_to, min_days, required_quals, description, is_command, is_cabinet
		FROM faction_rank_names WHERE faction = $1 ORDER BY level`, faction)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Rank
	for rows.Next() {
		var r Rank
		if err := rows.Scan(&r.Level, &r.Name, &r.Short, &r.Slots, &r.PromoteUpTo, &r.MinDays, &r.Quals, &r.Description, &r.IsCommand, &r.IsCabinet); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RankFor finds level in ranks, or a bare "Level n" rank.
func RankFor(ranks []Rank, level int) Rank {
	for _, r := range ranks {
		if r.Level == level {
			return r
		}
	}
	return Rank{Level: level}
}

// Command is one faction a player commands.
type Command struct {
	Faction   string
	Level     int
	Rank      Rank
	Authority int  // highest level they may set others to (0 = can't change ranks)
	Cabinet   bool // their rank is ticked as cabinet

	// Administration division membership: administrate and maintain the
	// panel whatever their rank.
	Admin          bool
	AdminRole      string
	AdminCommander bool
}

// IsCommand reports whether their rank is a command rank (rank changes and
// discipline), as opposed to Administration-only access.
func (c Command) IsCommand() bool { return c.Rank.IsCommand }

// CanRecord reports whether they may keep records (personnel file, roll
// call, certifications, training, divisions) on a member at targetLevel.
func (c Command) CanRecord(targetLevel int, targetCabinet bool) bool {
	switch {
	case c.Cabinet:
		return true
	case c.Admin && !targetCabinet:
		return true
	}
	return c.Rank.IsCommand && targetLevel < c.Level
}

// CanDiscipline reports whether they may discipline a member at targetLevel.
func (c Command) CanDiscipline(targetLevel int) bool {
	return c.Rank.IsCommand && (c.Cabinet || targetLevel < c.Level)
}

// CanAppointAdmin reports whether they may appoint to Administration.
func (c Command) CanAppointAdmin() bool { return c.Cabinet || c.AdminCommander }

// CommandOf returns the factions the player has command authority in.
func CommandOf(ctx context.Context, pool *pgxpool.Pool, playerID int64) ([]Command, error) {
	var out []Command
	for _, f := range Factions {
		c, ok, err := CommandIn(ctx, pool, playerID, f)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, c)
		}
	}
	return out, nil
}

// CommandIn reports whether the player has command authority in faction.
func CommandIn(ctx context.Context, pool *pgxpool.Pool, playerID int64, faction string) (Command, bool, error) {
	if !Valid(faction) {
		return Command{}, false, nil
	}
	c := Command{Faction: faction}
	err := pool.QueryRow(ctx, `
		SELECT p.`+column[faction]+`, COALESCE(r.name, ''), COALESCE(r.short_name, ''), COALESCE(r.slots, 0), COALESCE(r.promote_up_to, 0),
		       COALESCE(r.is_command, false), COALESCE(r.is_cabinet, false)
		FROM players p LEFT JOIN faction_rank_names r ON r.faction = $2 AND r.level = p.`+column[faction]+`
		WHERE p.id = $1`, playerID, faction).Scan(&c.Level, &c.Rank.Name, &c.Rank.Short, &c.Rank.Slots, &c.Rank.PromoteUpTo, &c.Rank.IsCommand, &c.Rank.IsCabinet)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, false, nil
	}
	if err != nil {
		return c, false, err
	}
	c.Rank.Level = c.Level
	if c.Rank.CanPromote() {
		c.Authority = c.Rank.PromoteUpTo
	}
	c.Cabinet = c.Rank.IsCabinet && c.Rank.IsCommand
	var top string
	err = pool.QueryRow(ctx, `
		SELECT md.role, d.roles[1] FROM faction_member_divisions md
		JOIN faction_divisions d ON d.faction = md.faction AND d.key = md.division_key
		WHERE md.faction = $1 AND md.player_id = $2 AND d.is_admin`, faction, playerID).Scan(&c.AdminRole, &top)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return c, false, err
	}
	c.Admin = err == nil && c.Level > 0
	c.AdminCommander = c.Admin && c.AdminRole == top
	return c, c.Level > 0 && (c.Rank.IsCommand || c.Admin), nil
}

// Actor is who is making a change, and under which authority.
type Actor struct {
	PlayerID int64
	Source   string // audit.SourceWebsite / audit.SourceDiscord
	Via      string // ViaCommand or ViaStaffOverride
}

// Change is the result of a SetLevel call.
type Change struct {
	Kind       string // recruit / promote / demote / remove
	From, To   int
	OwnFaction bool // a staff override by a member of this faction
}

// SetLevel changes targetID's level in faction (0 = remove from the
// faction). For ViaCommand the actor's command authority is checked here;
// for ViaStaffOverride the caller must already have checked the override
// permission.
func SetLevel(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, targetID int64, level int, reason string) (Change, error) {
	return setLevel(ctx, pool, actor, faction, targetID, level, reason, false)
}

// setLevel is SetLevel; probation skips the time-in-rank rule when
// confirming a recruit (probation has its own length).
func setLevel(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, targetID int64, level int, reason string, probation bool) (Change, error) {
	var ch Change
	col := column[faction]
	if col == "" {
		return ch, notAllowed("unknown faction")
	}
	if actor.Via != ViaCommand && actor.Via != ViaStaffOverride {
		return ch, fmt.Errorf("factions: unknown authority %q", actor.Via)
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ch, notAllowed("a reason is required")
	}
	if len(reason) > 300 {
		return ch, notAllowed("the reason is too long (300 characters max)")
	}
	if actor.PlayerID == targetID {
		return ch, notAllowed("you can't change your own %s rank", Name(faction))
	}
	if level < 0 || level > 30 {
		return ch, notAllowed("the level must be between 0 and 30")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return ch, err
	}
	defer tx.Rollback(ctx)

	// Lock both rows in id order so two changes can't deadlock or race.
	levels := map[int64]int{}
	rows, err := tx.Query(ctx, `SELECT id, COALESCE(`+col+`, 0) FROM players WHERE id = ANY($1) ORDER BY id FOR UPDATE`,
		[]int64{actor.PlayerID, targetID})
	if err != nil {
		return ch, err
	}
	for rows.Next() {
		var id int64
		var l int
		if err := rows.Scan(&id, &l); err != nil {
			rows.Close()
			return ch, err
		}
		levels[id] = l
	}
	rows.Close()
	cur, ok := levels[targetID]
	if !ok {
		return ch, notAllowed("that player doesn't exist")
	}
	actorLevel := levels[actor.PlayerID]
	if cur == level {
		if level == 0 {
			return ch, notAllowed("they aren't in %s", Name(faction))
		}
		return ch, notAllowed("they're already at that rank")
	}

	ranks, err := Ranks(ctx, tx, faction)
	if err != nil {
		return ch, err
	}
	if len(ranks) > 0 && level > ranks[len(ranks)-1].Level {
		return ch, notAllowed("%s only has %d ranks", Name(faction), ranks[len(ranks)-1].Level)
	}

	switch actor.Via {
	case ViaCommand:
		ar := RankFor(ranks, actorLevel)
		if actorLevel == 0 || !ar.Command() {
			return ch, notAllowed("only %s command can change ranks", Name(faction))
		}
		if !ar.CanPromote() {
			return ch, notAllowed("%s can't change ranks", ar.Label())
		}
		authority := ar.PromoteUpTo
		if cur > authority {
			return ch, notAllowed("%s is above what you can change (up to %s)", RankFor(ranks, cur).Label(), RankFor(ranks, authority).Label())
		}
		if level > authority {
			return ch, notAllowed("you can set ranks up to %s", RankFor(ranks, authority).Label())
		}
		// Rank rules bind command; Management can go past them.
		if level > cur && cur > 0 {
			if err := promotionRules(ctx, tx, faction, ranks, targetID, cur, level, probation); err != nil {
				return ch, err
			}
		}
		if cur == 0 {
			if bl, err := ActiveBlacklist(ctx, tx, faction, targetID); err != nil {
				return ch, err
			} else if bl != nil {
				return ch, notAllowed("they're blacklisted from %s %s", Name(faction), bl.UntilText())
			}
		}
		// Slot limits bind command; Management can go over them.
		if r := RankFor(ranks, level); level > 0 && r.Slots > 0 {
			var filled int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM players WHERE `+col+` = $1`, level).Scan(&filled); err != nil {
				return ch, err
			}
			if filled >= r.Slots {
				return ch, notAllowed("%s is full (%d of %d slots)", r.Label(), filled, r.Slots)
			}
		}
	case ViaStaffOverride:
		ch.OwnFaction = actorLevel > 0
	}

	switch {
	case cur == 0:
		ch.Kind = "recruit"
	case level == 0:
		ch.Kind = "remove"
	case level > cur:
		ch.Kind = "promote"
	default:
		ch.Kind = "demote"
	}
	ch.From, ch.To = cur, level

	logReason := reason
	if actor.Via == ViaStaffOverride {
		logReason = "Staff override: " + reason
		if ch.OwnFaction {
			logReason = "Staff override (own faction): " + reason
		}
	}
	if err := audit.SetActor(ctx, tx, actor.PlayerID, actor.Source, logReason); err != nil {
		return ch, err
	}
	if _, err := tx.Exec(ctx, `UPDATE players SET `+col+` = $2 WHERE id = $1`, targetID, level); err != nil {
		return ch, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO faction_log (faction, actor_id, target_id, kind, from_level, to_level, reason, via, own_faction)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		faction, actor.PlayerID, targetID, ch.Kind, cur, level, reason, actor.Via, ch.OwnFaction); err != nil {
		return ch, err
	}
	if err := afterLevelChange(ctx, tx, faction, targetID, ch.Kind); err != nil {
		return ch, err
	}
	return ch, tx.Commit(ctx)
}

// Member is one roster row.
type Member struct {
	ID        int64
	Name      string
	Level     int
	Rank      Rank
	InRank    time.Duration // since their last level change (0 if unknown)
	RankSince *time.Time    // their last level change, if recorded
	LastSeen  *time.Time
	Online    bool
	Hours     int64 // playtime in this faction
	Staff     string
}

// Roster returns the faction's members, highest rank first.
func Roster(ctx context.Context, pool *pgxpool.Pool, faction string) ([]Member, error) {
	col := column[faction]
	if col == "" {
		return nil, notAllowed("unknown faction")
	}
	ranks, err := Ranks(ctx, pool, faction)
	if err != nil {
		return nil, err
	}
	play := map[string]string{"police": "cop_playtime_seconds", "ems": "medic_playtime_seconds"}[faction]
	rows, err := pool.Query(ctx, `
		SELECT p.id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id), p.`+col+`,
		       (SELECT max(changed_at) FROM rank_changes rc WHERE rc.player_id = p.id AND rc.field = $1),
		       p.last_seen,
		       EXISTS (SELECT 1 FROM player_sessions s WHERE s.player_id = p.id AND s.disconnected_at IS NULL),
		       COALESCE(p.`+play+`, 0) / 3600, COALESCE(sr.display_name, '')
		FROM players p LEFT JOIN staff_ranks sr ON sr.id = p.staff_rank_id
		WHERE p.`+col+` > 0
		ORDER BY p.`+col+` DESC, 2`, col)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		var since *time.Time
		if err := rows.Scan(&m.ID, &m.Name, &m.Level, &since, &m.LastSeen, &m.Online, &m.Hours, &m.Staff); err != nil {
			return nil, err
		}
		m.Rank = RankFor(ranks, m.Level)
		if since != nil {
			m.InRank, m.RankSince = time.Since(*since), since
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// LogEntry is one Command log row.
type LogEntry struct {
	ID         int64
	When       time.Time
	Kind       string
	By         string
	Target     string
	TargetID   int64
	From, To   string
	Reason     string
	Via        string
	OwnFaction bool
	Detail     string // for non-rank entries
}

// Change describes the entry, e.g. "Constable → Senior Constable".
func (e LogEntry) Change() string {
	switch e.Kind {
	case "recruit", "promote", "demote", "remove":
	default:
		return e.Detail
	}
	switch e.Kind {
	case "recruit":
		return "Joined as " + e.To
	case "remove":
		return "Removed (was " + e.From + ")"
	}
	return e.From + " → " + e.To
}

// Log returns the faction's command log, newest first. kind and q filter
// (either may be ""); targetID > 0 limits it to one member.
func Log(ctx context.Context, pool *pgxpool.Pool, faction, kind, q string, targetID int64, limit int) ([]LogEntry, error) {
	ranks, err := Ranks(ctx, pool, faction)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		SELECT l.id, l.created_at, l.kind,
		       COALESCE(NULLIF(a.name, ''), NULLIF(a.steam_name, ''), 'Player #' || a.id, 'Unknown'),
		       COALESCE(NULLIF(t.name, ''), NULLIF(t.steam_name, ''), 'Player #' || t.id, 'Deleted player'),
		       COALESCE(l.target_id, 0), l.from_level, l.to_level, l.reason, l.via, l.own_faction, l.detail
		FROM faction_log l
		LEFT JOIN players a ON a.id = l.actor_id
		LEFT JOIN players t ON t.id = l.target_id
		WHERE l.faction = $1 AND ($2 = '' OR l.kind = $2) AND ($3 = 0 OR l.target_id = $3)
		  AND ($4 = '' OR l.reason ILIKE '%' || $4 || '%' OR t.name ILIKE '%' || $4 || '%' OR a.name ILIKE '%' || $4 || '%')
		ORDER BY l.id DESC LIMIT $5`, faction, kind, targetID, strings.TrimSpace(q), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LogEntry
	for rows.Next() {
		var e LogEntry
		var from, to int
		if err := rows.Scan(&e.ID, &e.When, &e.Kind, &e.By, &e.Target, &e.TargetID, &from, &to, &e.Reason, &e.Via, &e.OwnFaction, &e.Detail); err != nil {
			return nil, err
		}
		e.From, e.To = RankFor(ranks, from).Label(), RankFor(ranks, to).Label()
		out = append(out, e)
	}
	return out, rows.Err()
}
