package factions

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Rank rules, settings and the helpers the command tools share (layout plan
// "Police command": Ranks & gear, Discipline, Divisions & quals, Recruits &
// training). Everything here is done under faction command authority
// (ViaCommand); staff can read it, and staff with factions.configure can
// also edit rank rules (ViaStaffOverride).

type dbtx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Column is the players column holding a faction's level.
func Column(faction string) string { return column[faction] }

// levelOf returns a player's level in faction (0 = not a member).
func levelOf(ctx context.Context, q dbtx, faction string, playerID int64) (int, error) {
	var l int
	err := q.QueryRow(ctx, `SELECT COALESCE(`+column[faction]+`, 0) FROM players WHERE id = $1`, playerID).Scan(&l)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, notAllowed("that player doesn't exist")
	}
	return l, err
}

func playerName(ctx context.Context, q dbtx, id int64) string {
	var n string
	_ = q.QueryRow(ctx, `SELECT COALESCE(NULLIF(name, ''), NULLIF(steam_name, ''), 'Player #' || id) FROM players WHERE id = $1`, id).Scan(&n)
	return n
}

// authority is the actor's standing on the command panel.
//
//   - Command: their rank is ticked as command. Rank changes and discipline
//     for members ranked below them (up to "promotes up to" for ranks);
//     records for members below them; maintaining the panel.
//   - Cabinet: their rank is ticked as cabinet (always also command). Can
//     overwrite everything: acts on anyone except themselves.
//   - Administration: a member of the faction's Administration division,
//     whatever their rank. Records for anyone except cabinet members, and
//     maintaining the panel. No rank changes or discipline.
//   - Management: staff with factions.configure (ViaStaffOverride), checked
//     by the caller. Records, maintenance and Administration appointments.
type authority struct {
	Level          int // actor's rank
	UpTo           int // highest level they may set others to (0 = none)
	IsCommand      bool
	Cabinet        bool
	Admin          bool // in the Administration division
	AdminCommander bool // the Administration division's Commander
	Management     bool
	Ranks          []Rank
	TargetLvl      int
	TargetCabinet  bool
}

// standing loads the actor's authority; it refuses anyone with no access
// to the command panel.
func standing(ctx context.Context, q dbtx, actor Actor, faction string) (authority, error) {
	var a authority
	if !Valid(faction) {
		return a, notAllowed("unknown faction")
	}
	var err error
	if a.Ranks, err = Ranks(ctx, q, faction); err != nil {
		return a, err
	}
	if a.Level, err = levelOf(ctx, q, faction, actor.PlayerID); err != nil {
		return a, err
	}
	switch actor.Via {
	case ViaStaffOverride:
		a.Management = true
		return a, nil
	case ViaCommand:
	default:
		return a, notAllowed("unknown authority")
	}
	if a.Level == 0 {
		return a, notAllowed("only %s command can do that", Name(faction))
	}
	r := RankFor(a.Ranks, a.Level)
	a.IsCommand, a.Cabinet = r.IsCommand, r.IsCabinet && r.IsCommand
	if r.CanPromote() {
		a.UpTo = r.PromoteUpTo
	}
	var role, top string
	err = q.QueryRow(ctx, `
		SELECT md.role, d.roles[1] FROM faction_member_divisions md
		JOIN faction_divisions d ON d.faction = md.faction AND d.key = md.division_key
		WHERE md.faction = $1 AND md.player_id = $2 AND d.is_admin`, faction, actor.PlayerID).Scan(&role, &top)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return a, err
	default:
		a.Admin, a.AdminCommander = true, role == top
	}
	if !a.IsCommand && !a.Admin {
		return a, notAllowed("only %s command can do that", Name(faction))
	}
	return a, nil
}

// target loads targetID's rank for a check against the actor. Acting on
// yourself is refused unless allowSelf.
func (a *authority) target(ctx context.Context, q dbtx, actor Actor, faction string, targetID int64, allowSelf bool) error {
	if targetID == actor.PlayerID && !allowSelf {
		return notAllowed("you can't do that to yourself")
	}
	var err error
	if a.TargetLvl, err = levelOf(ctx, q, faction, targetID); err != nil {
		return err
	}
	a.TargetCabinet = a.TargetLvl > 0 && RankFor(a.Ranks, a.TargetLvl).IsCabinet
	return nil
}

// ownRecords reports whether they may keep their own records (cabinet
// overwrites everything; Management overrides). Never rank or discipline.
func (a authority) ownRecords() bool { return a.Cabinet || a.Management }

// canRecord reports whether the actor may change the target's records
// (personnel file, roll call, certifications, training, divisions).
func (a authority) canRecord() error {
	switch {
	case a.Management, a.Cabinet:
		return nil
	case a.Admin && !a.TargetCabinet:
		return nil
	case a.Admin && !a.IsCommand:
		return notAllowed("only cabinet can change a cabinet member's records")
	case a.IsCommand && a.TargetLvl < a.Level:
		return nil
	}
	return notAllowed("you can only act on members ranked below you")
}

// commandOver is for rank authority: rank changes, discipline, discharges
// and blacklists. It needs a command rank; the target (targetID > 0) must
// be someone else ranked below them, unless the actor is cabinet.
func commandOver(ctx context.Context, q dbtx, actor Actor, faction string, targetID int64) (authority, error) {
	if actor.Via != ViaCommand {
		return authority{}, notAllowed("only %s command can do that", Name(faction))
	}
	a, err := standing(ctx, q, actor, faction)
	if err != nil {
		return a, err
	}
	if !a.IsCommand {
		return a, notAllowed("only %s command can do that; Administration keeps the records", Name(faction))
	}
	if targetID > 0 {
		if err := a.target(ctx, q, actor, faction, targetID, false); err != nil {
			return a, err
		}
		if !a.Cabinet && a.TargetLvl >= a.Level {
			return a, notAllowed("you can only act on members ranked below you")
		}
	}
	return a, nil
}

// recordsOver is for keeping records on a member: command (members below
// them), Administration (anyone but cabinet), cabinet and Management.
// Cabinet and Management can keep their own records too.
func recordsOver(ctx context.Context, q dbtx, actor Actor, faction string, targetID int64) (authority, error) {
	a, err := standing(ctx, q, actor, faction)
	if err != nil {
		return a, err
	}
	if err := a.target(ctx, q, actor, faction, targetID, a.ownRecords()); err != nil {
		return a, err
	}
	return a, a.canRecord()
}

// maintainOver is for maintaining the panel itself (rank rules, settings,
// head trainers): command, cabinet, Administration and Management.
func maintainOver(ctx context.Context, q dbtx, actor Actor, faction string) (authority, error) {
	return standing(ctx, q, actor, faction)
}

// logEvent writes a non-rank Command log entry.
func logEvent(ctx context.Context, q dbtx, faction string, actor Actor, targetID int64, level int, kind, detail, reason string) error {
	_, err := q.Exec(ctx, `
		INSERT INTO faction_log (faction, actor_id, target_id, kind, from_level, to_level, reason, via, detail)
		VALUES ($1, $2, NULLIF($3, 0), $4, $5, $5, $6, $7, $8)`,
		faction, actor.PlayerID, targetID, kind, level, reason, actor.Via, detail)
	return err
}

func cleanText(s string, max int, what string, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if required && s == "" {
		return s, notAllowed("%s is required", what)
	}
	if len([]rune(s)) > max {
		return s, notAllowed("%s is too long (%d characters max)", what, max)
	}
	return s, nil
}

// promotionRules enforces the rank rules on a command promotion from cur
// to level: time in the current rank and the new rank's qualifications.
func promotionRules(ctx context.Context, q dbtx, faction string, ranks []Rank, targetID int64, cur, level int, probation bool) error {
	if from := RankFor(ranks, cur); from.MinDays > 0 && !probation {
		var since *time.Time
		if err := q.QueryRow(ctx, `SELECT max(changed_at) FROM rank_changes WHERE player_id = $1 AND field = $2`,
			targetID, column[faction]).Scan(&since); err != nil {
			return err
		}
		if since != nil {
			if days := int(time.Since(*since).Hours() / 24); days < from.MinDays {
				return notAllowed("%s needs %d days in rank before promotion (they have %d)", from.Label(), from.MinDays, days)
			}
		}
	}
	to := RankFor(ranks, level)
	if len(to.Quals) == 0 {
		return nil
	}
	held, err := memberQuals(ctx, q, faction, targetID)
	if err != nil {
		return err
	}
	var missing []string
	for _, k := range to.Quals {
		if !held[k] {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return notAllowed("%s needs the %s qualification%s", to.Label(), strings.Join(missing, ", "), map[bool]string{true: "s"}[len(missing) > 1])
	}
	return nil
}

// afterLevelChange keeps the command records in step with a rank change:
// recruits start probation; leaving ends probation and division membership.
func afterLevelChange(ctx context.Context, q dbtx, faction string, targetID int64, kind string) error {
	switch kind {
	case "recruit":
		s, err := GetSettings(ctx, q, faction)
		if err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `
			INSERT INTO faction_probations (faction, player_id, ends_at) VALUES ($1, $2, now() + make_interval(days => $3))
			ON CONFLICT DO NOTHING`, faction, targetID, s.ProbationDays); err != nil {
			return err
		}
		_, err = q.Exec(ctx, `
			INSERT INTO faction_members (faction, player_id, enrolled_on) VALUES ($1, $2, current_date)
			ON CONFLICT (faction, player_id) DO UPDATE SET enrolled_on = current_date, status = 'active', updated_at = now()`, faction, targetID)
		return err
	case "remove":
		if _, err := q.Exec(ctx, `UPDATE faction_probations SET status = 'ended', decided_at = now(),
			note = CASE WHEN note = '' THEN 'Left the faction.' ELSE note END
			WHERE faction = $1 AND player_id = $2 AND status = 'active'`, faction, targetID); err != nil {
			return err
		}
		_, err := q.Exec(ctx, `DELETE FROM faction_member_divisions WHERE faction = $1 AND player_id = $2`, faction, targetID)
		return err
	}
	return nil
}

// Settings are the per-faction numbers command's tools use.
type Settings struct {
	ProbationDays    int
	PointsExpiryDays int
	MVWDays          int
	BlacklistDays    int
}

func GetSettings(ctx context.Context, q dbtx, faction string) (Settings, error) {
	s := Settings{ProbationDays: 14, PointsExpiryDays: 90, MVWDays: 7, BlacklistDays: 90}
	err := q.QueryRow(ctx, `SELECT probation_days, points_expiry_days, mvw_days, blacklist_days FROM faction_settings WHERE faction = $1`, faction).
		Scan(&s.ProbationDays, &s.PointsExpiryDays, &s.MVWDays, &s.BlacklistDays)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, nil
	}
	return s, err
}

// UpdateSettings is for the faction's top rank.
func UpdateSettings(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, s Settings, reason string) error {
	switch {
	case s.ProbationDays < 1 || s.ProbationDays > 90:
		return notAllowed("probation must be 1 to 90 days")
	case s.PointsExpiryDays < 7 || s.PointsExpiryDays > 730:
		return notAllowed("points must expire after 7 to 730 days")
	case s.MVWDays < 1 || s.MVWDays > 90:
		return notAllowed("warnings must expire after 1 to 90 days")
	case s.BlacklistDays < 1 || s.BlacklistDays > 3650:
		return notAllowed("the default blacklist must be 1 to 3650 days")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	a, err := maintainOver(ctx, tx, actor, faction)
	if err != nil {
		return err
	}
	old, err := GetSettings(ctx, tx, faction)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO faction_settings (faction, probation_days, points_expiry_days, mvw_days, blacklist_days) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (faction) DO UPDATE SET probation_days = $2, points_expiry_days = $3, mvw_days = $4, blacklist_days = $5`,
		faction, s.ProbationDays, s.PointsExpiryDays, s.MVWDays, s.BlacklistDays); err != nil {
		return err
	}
	var ch []string
	diff := func(what string, a, b int) {
		if a != b {
			ch = append(ch, fmt.Sprintf("%s %d → %d days", what, a, b))
		}
	}
	diff("probation", old.ProbationDays, s.ProbationDays)
	diff("points expiry", old.PointsExpiryDays, s.PointsExpiryDays)
	diff("warning expiry", old.MVWDays, s.MVWDays)
	diff("default blacklist", old.BlacklistDays, s.BlacklistDays)
	if len(ch) == 0 {
		return notAllowed("nothing changed")
	}
	if reason, err = cleanText(reason, 300, "a reason", false); err != nil {
		return err
	}
	if err := logEvent(ctx, tx, faction, actor, 0, a.Level, "settings", "Settings: "+strings.Join(ch, "; "), reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RankRules is the editable part of a rank.
type RankRules struct {
	Name, Short string
	Slots       int
	MinDays     int
	PromoteUpTo int
	Quals       []string
	Description string
	Command     bool // command rank (CMD)
	Cabinet     bool // cabinet rank (CAB); must also be command
}

// CanEditRank says whether the viewer may edit rank level's rules: anyone
// who maintains the panel, for ranks other than their own; cabinet, any.
func (c Command) CanEditRank(level int) bool {
	return c.Cabinet || level != c.Level
}

// CanEditRankType says whether they may change the Command / Cabinet ticks
// and "promotes up to" (cabinet only; Management via staff override).
func (c Command) CanEditRankType() bool { return c.Cabinet }

// UpdateRank changes a rank's rules. ViaCommand: command, cabinet and the
// Administration division, for ranks other than their own; only cabinet can
// change the Command / Cabinet ticks and "promotes up to", or edit their
// own rank. ViaStaffOverride: Management (factions.configure, checked by
// the caller), anything.
func UpdateRank(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, level int, rr RankRules) error {
	var err error
	if rr.Name, err = cleanText(rr.Name, 60, "the rank name", true); err != nil {
		return err
	}
	if rr.Short, err = cleanText(rr.Short, 12, "the short name", false); err != nil {
		return err
	}
	if rr.Description, err = cleanText(rr.Description, 500, "the description", false); err != nil {
		return err
	}
	switch {
	case rr.Slots < 0 || rr.Slots > 500:
		return notAllowed("slots must be 0 (no limit) to 500")
	case rr.MinDays < 0 || rr.MinDays > 365:
		return notAllowed("minimum days must be 0 to 365")
	case rr.PromoteUpTo < 0 || rr.PromoteUpTo >= level:
		return notAllowed("a rank can only promote up to ranks below itself")
	case rr.Cabinet && !rr.Command:
		return notAllowed("a cabinet rank must also be a command rank")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	a, err := maintainOver(ctx, tx, actor, faction)
	if err != nil {
		return err
	}
	actorLevel := a.Level
	ranks, err := Ranks(ctx, tx, faction)
	if err != nil {
		return err
	}
	var old Rank
	for _, r := range ranks {
		if r.Level == level {
			old = r
		}
	}
	if old.Level == 0 {
		return notAllowed("that rank isn't set up yet; add it on Roles → Faction rank names")
	}
	if !a.Management && level == a.Level && (!rr.Command || (old.IsCabinet && !rr.Cabinet)) {
		return notAllowed("you can't take Command or Cabinet away from your own rank; Management can")
	}
	if !a.Management && !a.Cabinet {
		if level == a.Level {
			return notAllowed("you can't edit your own rank's rules; cabinet can")
		}
		if rr.Command != old.IsCommand || rr.Cabinet != old.IsCabinet || rr.PromoteUpTo != old.PromoteUpTo {
			return notAllowed("only cabinet or Management can change Command, Cabinet or \"promotes up to\"")
		}
	}
	quals, err := Quals(ctx, tx, faction)
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, q := range quals {
		known[q.Key] = true
	}
	clean := []string{}
	for _, k := range rr.Quals {
		if !known[k] {
			return notAllowed("unknown qualification %q", k)
		}
		if !slices.Contains(clean, k) {
			clean = append(clean, k)
		}
	}
	rr.Quals = clean
	if _, err := tx.Exec(ctx, `
		UPDATE faction_rank_names SET name = $3, short_name = NULLIF($4, ''), slots = NULLIF($5, 0), min_days = $6,
		       promote_up_to = $7, required_quals = $8, description = $9, is_command = $10, is_cabinet = $11
		WHERE faction = $1 AND level = $2`,
		faction, level, rr.Name, rr.Short, rr.Slots, rr.MinDays, rr.PromoteUpTo, rr.Quals, rr.Description, rr.Command, rr.Cabinet); err != nil {
		return err
	}
	var ch []string
	if old.Name != rr.Name {
		ch = append(ch, fmt.Sprintf("name %s → %s", old.Name, rr.Name))
	}
	if old.Short != rr.Short {
		ch = append(ch, fmt.Sprintf("short name %q → %q", old.Short, rr.Short))
	}
	if old.Slots != rr.Slots {
		ch = append(ch, fmt.Sprintf("slots %s → %s", slotsText(old.Slots), slotsText(rr.Slots)))
	}
	if old.MinDays != rr.MinDays {
		ch = append(ch, fmt.Sprintf("min days %d → %d", old.MinDays, rr.MinDays))
	}
	if old.PromoteUpTo != rr.PromoteUpTo {
		ch = append(ch, fmt.Sprintf("promotes up to %s → %s", upToText(ranks, old.PromoteUpTo), upToText(ranks, rr.PromoteUpTo)))
	}
	if strings.Join(old.Quals, ",") != strings.Join(rr.Quals, ",") {
		ch = append(ch, fmt.Sprintf("quals %s → %s", listText(old.Quals), listText(rr.Quals)))
	}
	tick := func(b bool) string { return map[bool]string{true: "yes", false: "no"}[b] }
	if old.IsCommand != rr.Command {
		ch = append(ch, "command rank "+tick(old.IsCommand)+" → "+tick(rr.Command))
	}
	if old.IsCabinet != rr.Cabinet {
		ch = append(ch, "cabinet "+tick(old.IsCabinet)+" → "+tick(rr.Cabinet))
	}
	if old.Description != rr.Description {
		ch = append(ch, "description updated")
	}
	if len(ch) == 0 {
		return notAllowed("nothing changed")
	}
	if err := logEvent(ctx, tx, faction, actor, 0, actorLevel, "rank_rules", old.Label()+": "+strings.Join(ch, "; "), ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func slotsText(n int) string {
	if n == 0 {
		return "no limit"
	}
	return fmt.Sprint(n)
}

func upToText(ranks []Rank, l int) string {
	if l == 0 {
		return "none"
	}
	return RankFor(ranks, l).Label()
}

func listText(l []string) string {
	if len(l) == 0 {
		return "none"
	}
	return strings.Join(l, ", ")
}
