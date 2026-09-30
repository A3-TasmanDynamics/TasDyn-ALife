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

// authority is the actor's standing for a command action.
type authority struct {
	Level     int // actor's rank
	UpTo      int // highest level they may set others to (0 = none)
	Top       bool
	Cabinet   bool
	Ranks     []Rank
	TargetLvl int
}

// commandOver checks the actor is faction command and, when targetID > 0,
// that the target is someone else ranked below them.
func commandOver(ctx context.Context, q dbtx, actor Actor, faction string, targetID int64) (authority, error) {
	var a authority
	if !Valid(faction) {
		return a, notAllowed("unknown faction")
	}
	if actor.Via != ViaCommand {
		return a, notAllowed("only %s command can do that", Name(faction))
	}
	var err error
	if a.Ranks, err = Ranks(ctx, q, faction); err != nil {
		return a, err
	}
	if a.Level, err = levelOf(ctx, q, faction, actor.PlayerID); err != nil {
		return a, err
	}
	r := RankFor(a.Ranks, a.Level)
	if a.Level == 0 || !r.Command() {
		return a, notAllowed("only %s command can do that", Name(faction))
	}
	if r.CanPromote() {
		a.UpTo = r.PromoteUpTo
	}
	a.Cabinet = r.IsCabinet
	a.Top = len(a.Ranks) > 0 && a.Level >= a.Ranks[len(a.Ranks)-1].Level
	if targetID > 0 {
		if targetID == actor.PlayerID {
			return a, notAllowed("you can't do that to yourself")
		}
		if a.TargetLvl, err = levelOf(ctx, q, faction, targetID); err != nil {
			return a, err
		}
		if a.TargetLvl >= a.Level {
			return a, notAllowed("you can only act on members ranked below you")
		}
	}
	return a, nil
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
	a, err := commandOver(ctx, tx, actor, faction, 0)
	if err != nil {
		return err
	}
	if !a.Top {
		return notAllowed("only the top rank can change these settings")
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

// CanEditRank says whether the actor may edit rank level's rules: the
// faction's top rank can edit every rank below their own.
func (c Command) CanEditRank(ranks []Rank, level int) bool {
	return len(ranks) > 0 && c.Level >= ranks[len(ranks)-1].Level && level < c.Level
}

// UpdateRank changes a rank's rules. ViaCommand: the top rank, for ranks
// below their own. ViaStaffOverride: staff with factions.configure (checked
// by the caller), any rank.
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
	actorLevel := 0
	switch actor.Via {
	case ViaCommand:
		a, err := commandOver(ctx, tx, actor, faction, 0)
		if err != nil {
			return err
		}
		if !a.Top {
			return notAllowed("only the top rank can edit rank rules")
		}
		if level >= a.Level {
			return notAllowed("you can't edit your own rank's rules; staff with factions.configure can")
		}
		actorLevel = a.Level
	case ViaStaffOverride:
	default:
		return notAllowed("unknown authority")
	}
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
