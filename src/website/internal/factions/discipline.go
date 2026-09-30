package factions

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Faction discipline (layout plan "Discipline"): it affects rank and the
// faction whitelist, not server bans; server rule breaks still go to staff
// as a case. Entries are never edited: a correction is a new entry.

// Offence is one entry in the faction's offence guide. Min 0 means a
// marked verbal warning (MVW) is allowed.
type Offence struct {
	ID       int64
	Tier     string
	Name     string
	Min, Max int
}

// MVW reports whether a marked verbal warning is allowed.
func (o Offence) MVW() bool { return o.Min == 0 }

// MinPoints is the fewest points it can carry.
func (o Offence) MinPoints() int { return max(o.Min, min(5, o.Max)) }

// RangeText is the guide shown when issuing, e.g. "MVW to 5 points".
func (o Offence) RangeText() string {
	lo := fmt.Sprintf("%d points", o.Min)
	if o.MVW() {
		lo = "MVW"
	}
	if o.Min == o.Max {
		return lo
	}
	return fmt.Sprintf("%s to %d points", lo, o.Max)
}

func Offences(ctx context.Context, q dbtx, faction string) ([]Offence, error) {
	rows, err := q.Query(ctx, `SELECT id, tier, name, min_points, max_points FROM faction_offences WHERE faction = $1 ORDER BY sort, id`, faction)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Offence
	for rows.Next() {
		var o Offence
		if err := rows.Scan(&o.ID, &o.Tier, &o.Name, &o.Min, &o.Max); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Threshold is one step on the points ladder. Crossing it suggests an
// action; command applies it by ticking the box when issuing.
type Threshold struct {
	At          int
	Label, Body string
	SuspendDays int
	Demote      bool // forced one-rank demotion
	Terminate   bool
	Blacklist   bool
}

// Ladder is the points ladder from the layout plan.
var Ladder = []Threshold{
	{At: 10, Label: "1-day suspension", Body: "Whitelist paused for 24 hours.", SuspendDays: 1},
	{At: 15, Label: "3-day suspension", Body: "Whitelist paused for 3 days.", SuspendDays: 3},
	{At: 20, Label: "7-day suspension, demotion recommended", Body: "Command reviews for a one-rank demotion.", SuspendDays: 7},
	{At: 30, Label: "7-day suspension and forced demotion", Body: "Demoted one rank on top of the suspension.", SuspendDays: 7, Demote: true},
	{At: 40, Label: "Contract termination", Body: "Removed from the faction.", Terminate: true},
	{At: 50, Label: "Termination and blacklist", Body: "Removed and blocked from reapplying until the blacklist ends.", Terminate: true, Blacklist: true},
}

// MVWsPerConversion active warnings become MVWConversionPoints points.
const (
	MVWsPerConversion   = 3
	MVWConversionPoints = 10
)

// crossed returns the highest threshold passed going from before to after.
func crossed(before, after int) *Threshold {
	var hit *Threshold
	for i := range Ladder {
		if before < Ladder[i].At && after >= Ladder[i].At {
			hit = &Ladder[i]
		}
	}
	return hit
}

// Band returns the highest threshold at or below points.
func Band(points int) *Threshold {
	var hit *Threshold
	for i := range Ladder {
		if points >= Ladder[i].At {
			hit = &Ladder[i]
		}
	}
	return hit
}

// Standing is a member's active discipline.
type Standing struct {
	Points    int
	MVWs      int
	Suspended *time.Time // until, if suspended now
}

const activeMVWs = `
	SELECT count(*) FROM faction_discipline d
	WHERE d.faction = $1 AND d.player_id = $2 AND d.kind = 'mvw' AND d.expires_at > now()
	  AND NOT EXISTS (SELECT 1 FROM faction_discipline c WHERE c.corrects_id = d.id)
	  AND d.id > COALESCE((SELECT max(id) FROM faction_discipline v WHERE v.faction = $1 AND v.player_id = $2 AND v.kind = 'mvw_conversion'), 0)`

// StandingOf returns one member's active points, warnings and suspension.
func StandingOf(ctx context.Context, q dbtx, faction string, playerID int64) (Standing, error) {
	var s Standing
	if err := q.QueryRow(ctx, `SELECT COALESCE(sum(points), 0) FROM faction_discipline WHERE faction = $1 AND player_id = $2 AND expires_at > now()`,
		faction, playerID).Scan(&s.Points); err != nil {
		return s, err
	}
	if err := q.QueryRow(ctx, activeMVWs, faction, playerID).Scan(&s.MVWs); err != nil {
		return s, err
	}
	err := q.QueryRow(ctx, `SELECT max(ends_at) FROM faction_suspensions WHERE faction = $1 AND player_id = $2 AND ends_at > now()`,
		faction, playerID).Scan(&s.Suspended)
	return s, err
}

// Standings returns the active points and suspensions of everyone who has
// any, by player ID (for the roster).
func Standings(ctx context.Context, q dbtx, faction string) (map[int64]Standing, error) {
	out := map[int64]Standing{}
	rows, err := q.Query(ctx, `SELECT player_id, sum(points)::int FROM faction_discipline WHERE faction = $1 AND expires_at > now() GROUP BY 1 HAVING sum(points) <> 0`, faction)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var p int
		if err := rows.Scan(&id, &p); err != nil {
			rows.Close()
			return nil, err
		}
		s := out[id]
		s.Points = p
		out[id] = s
	}
	rows.Close()
	rows, err = q.Query(ctx, `SELECT player_id, max(ends_at) FROM faction_suspensions WHERE faction = $1 AND ends_at > now() GROUP BY 1`, faction)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var until time.Time
		if err := rows.Scan(&id, &until); err != nil {
			return nil, err
		}
		s := out[id]
		s.Suspended = &until
		out[id] = s
	}
	return out, rows.Err()
}

// Issue is a discipline entry to add.
type Issue struct {
	TargetID    int64
	OffenceID   int64
	MVW         bool
	Points      int
	Notes       string
	ApplyAction bool
}

// IssueResult says what happened.
type IssueResult struct {
	Name          string
	Before, After int
	Converted     bool       // this was a third warning, converted to points
	Crossed       *Threshold // threshold crossed, if any
	Action        string     // what was applied ("" = nothing)
	Warning       string     // something that couldn't be applied
}

// IssueDiscipline adds a discipline entry and, if asked, applies the
// suggested action for a threshold it crosses. Command, for members
// ranked below them.
func IssueDiscipline(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, is Issue) (IssueResult, error) {
	var res IssueResult
	var err error
	if is.Notes, err = cleanText(is.Notes, 1000, "what happened", true); err != nil {
		return res, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return res, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT 1 FROM players WHERE id = $1 FOR UPDATE`, is.TargetID); err != nil {
		return res, err
	}
	a, err := commandOver(ctx, tx, actor, faction, is.TargetID)
	if err != nil {
		return res, err
	}
	if a.TargetLvl == 0 {
		return res, notAllowed("they aren't in %s", Name(faction))
	}
	var o Offence
	err = tx.QueryRow(ctx, `SELECT id, tier, name, min_points, max_points FROM faction_offences WHERE id = $1 AND faction = $2`, is.OffenceID, faction).
		Scan(&o.ID, &o.Tier, &o.Name, &o.Min, &o.Max)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, notAllowed("pick an offence")
	} else if err != nil {
		return res, err
	}
	if is.MVW && !o.MVW() {
		return res, notAllowed("%s can't be a verbal warning (%s)", o.Name, o.RangeText())
	}
	if !is.MVW && (is.Points < o.MinPoints() || is.Points > o.Max) {
		return res, notAllowed("%s carries %d to %d points", o.Name, o.MinPoints(), o.Max)
	}
	set, err := GetSettings(ctx, tx, faction)
	if err != nil {
		return res, err
	}
	st, err := StandingOf(ctx, tx, faction, is.TargetID)
	if err != nil {
		return res, err
	}
	res.Name = playerName(ctx, tx, is.TargetID)
	res.Before, res.After = st.Points, st.Points
	kind, points, days := "points", is.Points, set.PointsExpiryDays
	if is.MVW {
		kind, points, days = "mvw", 0, set.MVWDays
		res.Converted = st.MVWs+1 >= MVWsPerConversion
	} else {
		res.After += points
	}
	if res.Converted {
		res.After += MVWConversionPoints
	}
	res.Crossed = crossed(res.Before, res.After)

	// Work out the action before writing, so the entry records it.
	var demoteTo, terminate, blacklist = -1, false, false
	suspendDays := 0
	if t := res.Crossed; t != nil && is.ApplyAction {
		var done, notDone []string
		if t.SuspendDays > 0 {
			suspendDays = t.SuspendDays
			done = append(done, fmt.Sprintf("%d-day suspension", t.SuspendDays))
		}
		rankOK := a.TargetLvl <= a.UpTo
		switch {
		case t.Terminate && rankOK:
			terminate = true
			done = append(done, "removed from the faction")
		case t.Terminate:
			notDone = append(notDone, "termination needs higher command")
		case t.Demote && a.TargetLvl <= 1:
			notDone = append(notDone, "already at the lowest rank, so no demotion")
		case t.Demote && rankOK:
			demoteTo = a.TargetLvl - 1
			done = append(done, "demoted to "+RankFor(a.Ranks, demoteTo).Label())
		case t.Demote:
			notDone = append(notDone, "the demotion needs higher command")
		}
		if t.Blacklist {
			blacklist = true
			done = append(done, fmt.Sprintf("blacklisted for %d days", set.BlacklistDays))
		}
		res.Action = strings.Join(done, ", ")
		if len(notDone) > 0 {
			res.Warning = strings.ToUpper(notDone[0][:1]) + notDone[0][1:] + "."
			if res.Action != "" {
				res.Action += " (" + strings.Join(notDone, "; ") + ")"
			} else {
				res.Action = "Not applied: " + strings.Join(notDone, "; ")
			}
		}
	}

	var entryID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO faction_discipline (faction, player_id, level, offence, kind, points, notes, action, by_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now() + make_interval(days => $10)) RETURNING id`,
		faction, is.TargetID, a.TargetLvl, o.Name, kind, points, is.Notes, capitalise(res.Action), actor.PlayerID, days).Scan(&entryID); err != nil {
		return res, err
	}
	if res.Converted {
		if _, err := tx.Exec(ctx, `
			INSERT INTO faction_discipline (faction, player_id, level, offence, kind, points, notes, by_id, expires_at)
			VALUES ($1, $2, $3, $4, 'mvw_conversion', $5, $6, $7, now() + make_interval(days => $8))`,
			faction, is.TargetID, a.TargetLvl, fmt.Sprintf("%d active verbal warnings", MVWsPerConversion), MVWConversionPoints,
			fmt.Sprintf("%d active marked verbal warnings convert to %d points.", MVWsPerConversion, MVWConversionPoints), actor.PlayerID, set.PointsExpiryDays); err != nil {
			return res, err
		}
	}
	if suspendDays > 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO faction_suspensions (faction, player_id, discipline_id, ends_at, by_id) VALUES ($1, $2, $3, now() + make_interval(days => $4), $5)`,
			faction, is.TargetID, entryID, suspendDays, actor.PlayerID); err != nil {
			return res, err
		}
	}
	if blacklist {
		if _, err := tx.Exec(ctx, `INSERT INTO faction_blacklist (faction, player_id, last_level, reason, ends_at, by_id) VALUES ($1, $2, $3, $4, now() + make_interval(days => $5), $6)`,
			faction, is.TargetID, a.TargetLvl, "Reached "+fmt.Sprint(res.After)+" discipline points: "+o.Name, set.BlacklistDays, actor.PlayerID); err != nil {
			return res, err
		}
	}
	given := fmt.Sprintf("%d points", points)
	if is.MVW {
		given = "a marked verbal warning"
	}
	detail := fmt.Sprintf("%s: %s for %s", res.Name, given, o.Name)
	if res.After != res.Before {
		detail += fmt.Sprintf(" (%d → %d points)", res.Before, res.After)
	}
	if res.Converted {
		detail += fmt.Sprintf("; %d warnings converted to %d points", MVWsPerConversion, MVWConversionPoints)
	}
	if res.Action != "" {
		detail += ". " + capitalise(res.Action)
	}
	if err := logEvent(ctx, tx, faction, actor, is.TargetID, a.TargetLvl, "discipline", detail, is.Notes); err != nil {
		return res, err
	}
	if err := tx.Commit(ctx); err != nil {
		return res, err
	}

	// Rank changes go through setLevel (its own transaction and checks).
	reason := "Discipline: " + o.Name
	switch {
	case terminate:
		if _, err := setLevel(ctx, pool, actor, faction, is.TargetID, 0, reason, false); err != nil {
			res.Warning = "The discipline was recorded, but removing them failed: " + errText(err)
		} else if _, err := pool.Exec(ctx, `INSERT INTO faction_discharges (faction, player_id, last_level, type, notes, by_id) VALUES ($1, $2, $3, 'contract_termination', $4, $5)`,
			faction, is.TargetID, a.TargetLvl, fmt.Sprintf("Reached %d discipline points (%s).", res.After, o.Name), actor.PlayerID); err != nil {
			return res, err
		}
	case demoteTo >= 1:
		if _, err := setLevel(ctx, pool, actor, faction, is.TargetID, demoteTo, reason, false); err != nil {
			res.Warning = "The discipline was recorded, but the demotion failed: " + errText(err)
		}
	}
	return res, nil
}

func errText(err error) string {
	return strings.TrimPrefix(err.Error(), ErrNotAllowed.Error()+": ")
}

func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Entry is one discipline log row.
type Entry struct {
	ID         int64
	When       time.Time
	PlayerID   int64
	Player     string
	Rank       string
	Offence    string
	Kind       string
	Points     int
	Notes      string
	Action     string
	By         string
	Expires    time.Time
	CorrectsID int64
	Corrected  bool
}

// Given is the Given column: "5 pts", "MVW", "-5 pts".
func (e Entry) Given() string {
	if e.Kind == "mvw" {
		return "MVW"
	}
	return fmt.Sprintf("%d pts", e.Points)
}

func (e Entry) Active() bool { return time.Now().Before(e.Expires) && !e.Corrected }

// Correctable reports whether a correction can still be issued.
func (e Entry) Correctable() bool { return e.Kind != "correction" && !e.Corrected }

// DisciplineLog returns entries, newest first; playerID 0 = everyone.
func DisciplineLog(ctx context.Context, q dbtx, faction string, playerID int64, limit int) ([]Entry, error) {
	ranks, err := Ranks(ctx, q, faction)
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, `
		SELECT d.id, d.created_at, d.player_id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id), d.level,
		       d.offence, d.kind, d.points, d.notes, d.action, COALESCE(NULLIF(b.name, ''), NULLIF(b.steam_name, ''), 'Unknown'),
		       d.expires_at, COALESCE(d.corrects_id, 0), EXISTS (SELECT 1 FROM faction_discipline c WHERE c.corrects_id = d.id)
		FROM faction_discipline d JOIN players p ON p.id = d.player_id LEFT JOIN players b ON b.id = d.by_id
		WHERE d.faction = $1 AND ($2 = 0 OR d.player_id = $2)
		ORDER BY d.id DESC LIMIT $3`, faction, playerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		var lvl int
		if err := rows.Scan(&e.ID, &e.When, &e.PlayerID, &e.Player, &lvl, &e.Offence, &e.Kind, &e.Points, &e.Notes, &e.Action, &e.By, &e.Expires, &e.CorrectsID, &e.Corrected); err != nil {
			return nil, err
		}
		e.Rank = RankFor(ranks, lvl).Label()
		out = append(out, e)
	}
	return out, rows.Err()
}

// CorrectDiscipline reverses an entry with a new correction entry. It also
// ends a suspension the entry caused; rank changes aren't undone (use the
// roster for that).
func CorrectDiscipline(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, entryID int64, reason string) error {
	var err error
	if reason, err = cleanText(reason, 500, "a reason", true); err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var e Entry
	var lvl int
	err = tx.QueryRow(ctx, `SELECT player_id, level, offence, kind, points, expires_at FROM faction_discipline WHERE id = $1 AND faction = $2 FOR UPDATE`, entryID, faction).
		Scan(&e.PlayerID, &lvl, &e.Offence, &e.Kind, &e.Points, &e.Expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return notAllowed("that entry doesn't exist")
	} else if err != nil {
		return err
	}
	if e.Kind == "correction" {
		return notAllowed("a correction can't be corrected; issue a new entry instead")
	}
	var done bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM faction_discipline WHERE corrects_id = $1)`, entryID).Scan(&done); err != nil {
		return err
	}
	if done {
		return notAllowed("that entry has already been corrected")
	}
	a, err := commandOver(ctx, tx, actor, faction, 0)
	if err != nil {
		return err
	}
	if e.PlayerID == actor.PlayerID {
		return notAllowed("you can't correct your own discipline")
	}
	cur, err := levelOf(ctx, tx, faction, e.PlayerID)
	if err != nil {
		return err
	}
	if !a.Cabinet && max(cur, lvl) >= a.Level {
		return notAllowed("you can only correct discipline for members ranked below you")
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO faction_discipline (faction, player_id, level, offence, kind, points, notes, corrects_id, by_id, expires_at)
		VALUES ($1, $2, $3, $4, 'correction', $5, $6, $7, $8, $9)`,
		faction, e.PlayerID, cur, "Correction: "+e.Offence, -e.Points, reason, entryID, actor.PlayerID, e.Expires); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE faction_suspensions SET ends_at = now() WHERE discipline_id = $1 AND ends_at > now()`, entryID)
	if err != nil {
		return err
	}
	detail := fmt.Sprintf("%s: corrected %s for %s", playerName(ctx, tx, e.PlayerID), Entry{Kind: e.Kind, Points: e.Points}.Given(), e.Offence)
	if tag.RowsAffected() > 0 {
		detail += "; suspension lifted"
	}
	if err := logEvent(ctx, tx, faction, actor, e.PlayerID, cur, "discipline", detail, reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Discharge types a member can be removed with.
var DischargeTypes = []struct{ Key, Label string }{
	{"resigned", "Resigned"},
	{"inactivity", "Inactivity"},
	{"honourable_discharge", "Honourable discharge"},
	{"contract_termination", "Contract termination"},
}

// DischargeLabel is the display name for a discharge type.
func DischargeLabel(t string) string {
	for _, d := range DischargeTypes {
		if d.Key == t {
			return d.Label
		}
	}
	if t == "probation_ended" {
		return "Probation ended"
	}
	return t
}

// Discharge removes a member with a discharge record.
func Discharge(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, targetID int64, typ, notes string) error {
	var err error
	if DischargeLabel(typ) == typ {
		return notAllowed("pick a discharge type")
	}
	if notes, err = cleanText(notes, 500, "notes", true); err != nil {
		return err
	}
	lvl, err := levelOf(ctx, pool, faction, targetID)
	if err != nil {
		return err
	}
	if _, err := setLevel(ctx, pool, actor, faction, targetID, 0, DischargeLabel(typ)+": "+notes, false); err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `INSERT INTO faction_discharges (faction, player_id, last_level, type, notes, by_id) VALUES ($1, $2, $3, $4, $5, $6)`,
		faction, targetID, lvl, typ, notes, actor.PlayerID)
	return err
}

// DischargeRow is one Discharges row.
type DischargeRow struct {
	When     time.Time
	PlayerID int64
	Player   string
	Rank     string
	Type     string
	Notes    string
	By       string
}

func Discharges(ctx context.Context, q dbtx, faction string, limit int) ([]DischargeRow, error) {
	ranks, err := Ranks(ctx, q, faction)
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, `
		SELECT x.created_at, COALESCE(x.player_id, 0), COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Deleted player'), x.last_level,
		       x.type, x.notes, COALESCE(NULLIF(b.name, ''), NULLIF(b.steam_name, ''), 'Unknown')
		FROM faction_discharges x LEFT JOIN players p ON p.id = x.player_id LEFT JOIN players b ON b.id = x.by_id
		WHERE x.faction = $1 ORDER BY x.id DESC LIMIT $2`, faction, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DischargeRow
	for rows.Next() {
		var d DischargeRow
		var lvl int
		if err := rows.Scan(&d.When, &d.PlayerID, &d.Player, &lvl, &d.Type, &d.Notes, &d.By); err != nil {
			return nil, err
		}
		d.Rank, d.Type = RankFor(ranks, lvl).Label(), DischargeLabel(d.Type)
		out = append(out, d)
	}
	return out, rows.Err()
}

// Blacklisting is one blacklist row.
type Blacklisting struct {
	ID         int64
	PlayerID   int64
	Player     string
	LastLevel  int
	LastRank   string
	Reason     string
	From       time.Time
	Until      *time.Time
	By         string
	LiftedAt   *time.Time
	LiftedBy   string
	LiftReason string
}

func (b Blacklisting) Active() bool {
	return b.LiftedAt == nil && (b.Until == nil || time.Now().Before(*b.Until))
}

// UntilText is "until 9 Dec 2026" or "permanently".
func (b Blacklisting) UntilText() string {
	if b.Until == nil {
		return "permanently"
	}
	return "until " + b.Until.Local().Format("2 Jan 2006")
}

const blacklistCols = `
	SELECT x.id, x.player_id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id), x.last_level, x.reason,
	       x.starts_at, x.ends_at, COALESCE(NULLIF(b.name, ''), NULLIF(b.steam_name, ''), 'Unknown'),
	       x.lifted_at, COALESCE(NULLIF(l.name, ''), NULLIF(l.steam_name, ''), ''), COALESCE(x.lift_reason, '')
	FROM faction_blacklist x JOIN players p ON p.id = x.player_id
	LEFT JOIN players b ON b.id = x.by_id LEFT JOIN players l ON l.id = x.lifted_by`

func scanBlacklist(rows pgx.Rows, ranks []Rank) ([]Blacklisting, error) {
	defer rows.Close()
	var out []Blacklisting
	for rows.Next() {
		var x Blacklisting
		if err := rows.Scan(&x.ID, &x.PlayerID, &x.Player, &x.LastLevel, &x.Reason, &x.From, &x.Until, &x.By, &x.LiftedAt, &x.LiftedBy, &x.LiftReason); err != nil {
			return nil, err
		}
		x.LastRank = "Not a member"
		if x.LastLevel > 0 {
			x.LastRank = RankFor(ranks, x.LastLevel).Label()
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func Blacklist(ctx context.Context, q dbtx, faction string, limit int) ([]Blacklisting, error) {
	ranks, err := Ranks(ctx, q, faction)
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, blacklistCols+` WHERE x.faction = $1 ORDER BY x.id DESC LIMIT $2`, faction, limit)
	if err != nil {
		return nil, err
	}
	return scanBlacklist(rows, ranks)
}

// ActiveBlacklist returns the player's current blacklisting, or nil.
func ActiveBlacklist(ctx context.Context, q dbtx, faction string, playerID int64) (*Blacklisting, error) {
	rows, err := q.Query(ctx, blacklistCols+`
		WHERE x.faction = $1 AND x.player_id = $2 AND x.lifted_at IS NULL AND (x.ends_at IS NULL OR x.ends_at > now())
		ORDER BY x.ends_at DESC NULLS FIRST LIMIT 1`, faction, playerID)
	if err != nil {
		return nil, err
	}
	l, err := scanBlacklist(rows, nil)
	if err != nil || len(l) == 0 {
		return nil, err
	}
	return &l[0], nil
}

// AddBlacklist blocks a player who isn't a member from joining. days 0 =
// permanent.
func AddBlacklist(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, targetID int64, days int, reason string) error {
	var err error
	if reason, err = cleanText(reason, 500, "a reason", true); err != nil {
		return err
	}
	if days < 0 || days > 3650 {
		return notAllowed("the blacklist must be 1 to 3650 days, or permanent")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	a, err := commandOver(ctx, tx, actor, faction, targetID)
	if err != nil {
		return err
	}
	if a.TargetLvl > 0 {
		return notAllowed("they're still in %s; discharge them first", Name(faction))
	}
	if days == 0 && !a.Cabinet {
		return notAllowed("only cabinet can blacklist permanently")
	}
	if bl, err := ActiveBlacklist(ctx, tx, faction, targetID); err != nil {
		return err
	} else if bl != nil {
		return notAllowed("they're already blacklisted %s", bl.UntilText())
	}
	var last int
	_ = tx.QueryRow(ctx, `SELECT COALESCE((SELECT from_level FROM faction_log WHERE faction = $1 AND target_id = $2 AND kind = 'remove' ORDER BY id DESC LIMIT 1), 0)`,
		faction, targetID).Scan(&last)
	if last > a.UpTo {
		return notAllowed("they were %s, above what you can act on", RankFor(a.Ranks, last).Label())
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO faction_blacklist (faction, player_id, last_level, reason, ends_at, by_id)
		VALUES ($1, $2, $3, $4, CASE WHEN $5 = 0 THEN NULL ELSE now() + make_interval(days => $5) END, $6)`,
		faction, targetID, last, reason, days, actor.PlayerID); err != nil {
		return err
	}
	how := fmt.Sprintf("for %d days", days)
	if days == 0 {
		how = "permanently"
	}
	if err := logEvent(ctx, tx, faction, actor, targetID, 0, "blacklist", playerName(ctx, tx, targetID)+" blacklisted "+how, reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// LiftBlacklist ends a blacklisting early. Cabinet only (layout plan).
func LiftBlacklist(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, id int64, reason string) error {
	var err error
	if reason, err = cleanText(reason, 500, "a reason", true); err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	a, err := commandOver(ctx, tx, actor, faction, 0)
	if err != nil {
		return err
	}
	var pid int64
	var last int
	var active bool
	err = tx.QueryRow(ctx, `SELECT player_id, last_level, lifted_at IS NULL AND (ends_at IS NULL OR ends_at > now()) FROM faction_blacklist WHERE id = $1 AND faction = $2 FOR UPDATE`,
		id, faction).Scan(&pid, &last, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return notAllowed("that blacklisting doesn't exist")
	} else if err != nil {
		return err
	}
	if !active {
		return notAllowed("that blacklisting has already ended")
	}
	if pid == actor.PlayerID {
		return notAllowed("you can't lift your own blacklisting")
	}
	if !a.Cabinet {
		return notAllowed("only cabinet can lift a blacklist early")
	}
	if _, err := tx.Exec(ctx, `UPDATE faction_blacklist SET lifted_at = now(), lifted_by = $2, lift_reason = $3 WHERE id = $1`, id, actor.PlayerID, reason); err != nil {
		return err
	}
	if err := logEvent(ctx, tx, faction, actor, pid, 0, "blacklist", "Blacklist on "+playerName(ctx, tx, pid)+" lifted early", reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
