// Package staff is the staff-management service: listing staff, changing
// rank, status (LOA/suspension), team and region, and staff notes. The
// website's Admin Panel and the Discord bot's staff commands both call
// this package, so the seniority rules below are enforced in one place
// (docs/GAMEPANEL_PARITY.md §2, docs/DISCORD_BOT.md §1).
//
// Every rank/status/team change is attributed with audit.SetActor inside
// its transaction; the database trigger then records it in rank_changes and
// staff_log, which is what posts it to #staff-log and drives role sync.
package staff

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

// ErrNotAllowed is returned (wrapped, with a human-readable reason) when a
// change breaks a seniority rule. Callers show err.Error() to the user.
var ErrNotAllowed = errors.New("not allowed")

func notAllowed(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrNotAllowed, fmt.Sprintf(format, args...))
}

// HeadAdminLevel is the top rank level; only it may appoint its own level.
const HeadAdminLevel = 100

type Rank struct {
	ID    int
	Key   string
	Name  string
	Level int
}

type Member struct {
	ID              int64
	UID             string
	Name            string
	AvatarURL       string
	RankID          int
	RankName        string
	RankLevel       int
	Status          string // active / loa / suspended
	StatusReason    string
	StatusUntil     *time.Time
	Team            string
	Region          string
	DiscordUsername string
	LastSeen        *time.Time
}

type Note struct {
	Kind      string
	Body      string
	Author    string
	CreatedAt time.Time
}

type Change struct {
	Field     string
	Old       string
	New       string
	Source    string
	Actor     string
	Reason    string
	ChangedAt time.Time
}

const memberSelect = `
	SELECT p.id, p.uid,
	       COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id),
	       COALESCE(p.steam_avatar_url, ''),
	       COALESCE(sr.id, 0), COALESCE(sr.display_name, ''), COALESCE(sr.level, 0),
	       p.staff_status, COALESCE(p.staff_status_reason, ''), p.staff_status_until,
	       COALESCE(p.staff_team, ''), COALESCE(p.staff_region, ''),
	       COALESCE(p.discord_username, ''), p.last_seen
	FROM players p LEFT JOIN staff_ranks sr ON sr.id = p.staff_rank_id`

func scanMember(row pgx.Row) (Member, error) {
	var m Member
	err := row.Scan(&m.ID, &m.UID, &m.Name, &m.AvatarURL, &m.RankID, &m.RankName, &m.RankLevel,
		&m.Status, &m.StatusReason, &m.StatusUntil, &m.Team, &m.Region, &m.DiscordUsername, &m.LastSeen)
	return m, err
}

// List returns every staff member, most senior first.
func List(ctx context.Context, pool *pgxpool.Pool) ([]Member, error) {
	rows, err := pool.Query(ctx, memberSelect+` WHERE p.staff_rank_id IS NOT NULL ORDER BY sr.level DESC, 3`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Get returns one player (staff or not -- the profile page is also where a
// non-staff player is added to staff).
func Get(ctx context.Context, pool *pgxpool.Pool, playerID int64) (Member, error) {
	return scanMember(pool.QueryRow(ctx, memberSelect+` WHERE p.id = $1`, playerID))
}

func Ranks(ctx context.Context, pool *pgxpool.Pool) ([]Rank, error) {
	rows, err := pool.Query(ctx, `SELECT id, key, display_name, level FROM staff_ranks ORDER BY level`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Rank
	for rows.Next() {
		var r Rank
		if err := rows.Scan(&r.ID, &r.Key, &r.Name, &r.Level); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Actor identifies who is making a change and from where.
type Actor struct {
	PlayerID int64
	Source   string // audit.SourceWebsite / audit.SourceDiscord
}

func levelOf(ctx context.Context, tx pgx.Tx, playerID int64) (int, error) {
	var level int
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(sr.level, 0) FROM players p LEFT JOIN staff_ranks sr ON sr.id = p.staff_rank_id
		WHERE p.id = $1 FOR UPDATE OF p`, playerID).Scan(&level)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, notAllowed("that player doesn't exist")
	}
	return level, err
}

// checkSeniority applies the rules every change shares: not yourself, and
// you must outrank the target's current rank.
func checkSeniority(ctx context.Context, tx pgx.Tx, actor Actor, targetID int64) (actorLevel, targetLevel int, err error) {
	if actor.PlayerID == targetID {
		return 0, 0, notAllowed("you can't change your own rank or status")
	}
	if actorLevel, err = levelOf(ctx, tx, actor.PlayerID); err != nil {
		return 0, 0, err
	}
	if targetLevel, err = levelOf(ctx, tx, targetID); err != nil {
		return 0, 0, err
	}
	if targetLevel >= actorLevel && actorLevel < HeadAdminLevel {
		return 0, 0, notAllowed("you can only change staff ranked below you")
	}
	return actorLevel, targetLevel, nil
}

func requireReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", notAllowed("a reason is required")
	}
	if len(reason) > 500 {
		return "", notAllowed("the reason is too long (500 characters max)")
	}
	return reason, nil
}

// SetRank sets targetID's staff rank; rankID 0 removes them from staff.
// Promoting a non-staff player adds them to staff. The caller must already
// have checked the staff.edit (or, for rankID 0, staff.remove) permission.
func SetRank(ctx context.Context, pool *pgxpool.Pool, actor Actor, targetID int64, rankID int, reason string) error {
	reason, err := requireReason(reason)
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	actorLevel, _, err := checkSeniority(ctx, tx, actor, targetID)
	if err != nil {
		return err
	}

	var newRank *int
	if rankID != 0 {
		var level int
		if err := tx.QueryRow(ctx, `SELECT level FROM staff_ranks WHERE id = $1`, rankID).Scan(&level); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return notAllowed("that rank doesn't exist")
			}
			return err
		}
		if level >= actorLevel && actorLevel < HeadAdminLevel {
			return notAllowed("you can only assign ranks below your own")
		}
		newRank = &rankID
	}

	if err := audit.SetActor(ctx, tx, actor.PlayerID, actor.Source, reason); err != nil {
		return err
	}
	// Leaving staff also clears their status, team and region, so a
	// returning member starts clean.
	if newRank == nil {
		_, err = tx.Exec(ctx, `
			UPDATE players SET staff_rank_id = NULL, staff_status = 'active', staff_status_reason = NULL,
			       staff_status_until = NULL, staff_team = NULL, staff_region = NULL
			WHERE id = $1`, targetID)
	} else {
		_, err = tx.Exec(ctx, `UPDATE players SET staff_rank_id = $2 WHERE id = $1`, targetID, *newRank)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetStatus puts targetID on LOA, suspends them, or reinstates them
// ('active'). until is optional (nil = indefinite). The caller must have
// checked staff.loa (LOA and ending an LOA) or staff.suspend (suspending
// and lifting a suspension) -- see RequiredStatusPermission.
func SetStatus(ctx context.Context, pool *pgxpool.Pool, actor Actor, targetID int64, status, reason string, until *time.Time) error {
	if status != "active" && status != "loa" && status != "suspended" {
		return notAllowed("unknown status %q", status)
	}
	reason, err := requireReason(reason)
	if err != nil {
		return err
	}
	if until != nil && status == "active" {
		until = nil
	}
	if until != nil && !until.After(time.Now()) {
		return notAllowed("the end date must be in the future")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, targetLevel, err := checkSeniority(ctx, tx, actor, targetID)
	if err != nil {
		return err
	}
	if targetLevel == 0 {
		return notAllowed("that player isn't staff")
	}
	if err := audit.SetActor(ctx, tx, actor.PlayerID, actor.Source, reason); err != nil {
		return err
	}
	var statusReason *string
	if status != "active" {
		statusReason = &reason
	}
	if _, err := tx.Exec(ctx, `
		UPDATE players SET staff_status = $2, staff_status_reason = $3, staff_status_until = $4 WHERE id = $1
	`, targetID, status, statusReason, until); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RequiredStatusPermission is the permission needed to move a staff member
// from their current status to next: suspensions are more serious than
// LOA, so anything touching 'suspended' needs staff.suspend.
func RequiredStatusPermission(current, next string) string {
	if current == "suspended" || next == "suspended" {
		return "staff.suspend"
	}
	return "staff.loa"
}

// SetTeam sets a staff member's team and region (either may be empty).
func SetTeam(ctx context.Context, pool *pgxpool.Pool, actor Actor, targetID int64, team, region string) error {
	team, region = strings.TrimSpace(team), strings.TrimSpace(region)
	if len(team) > 50 || len(region) > 50 {
		return notAllowed("team and region are limited to 50 characters")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, targetLevel, err := checkSeniority(ctx, tx, actor, targetID); err != nil {
		return err
	} else if targetLevel == 0 {
		return notAllowed("that player isn't staff")
	}
	if err := audit.SetActor(ctx, tx, actor.PlayerID, actor.Source, "team/region update"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE players SET staff_team = NULLIF($2, ''), staff_region = NULLIF($3, '') WHERE id = $1`,
		targetID, team, region); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// AddNote appends a note about a staff member. Notes are never edited or
// deleted -- Gamepanel's overwritable notes lost history (GAMEPANEL_PARITY §2.2).
func AddNote(ctx context.Context, pool *pgxpool.Pool, actor Actor, targetID int64, kind, body string) error {
	body = strings.TrimSpace(body)
	if kind != "note" && kind != "promotion" {
		return notAllowed("unknown note type")
	}
	if body == "" || len(body) > 4000 {
		return notAllowed("a note must be between 1 and 4000 characters")
	}
	if actor.PlayerID == targetID {
		return notAllowed("you can't add notes about yourself")
	}
	_, err := pool.Exec(ctx, `INSERT INTO staff_notes (player_id, author_id, kind, body) VALUES ($1, $2, $3, $4)`,
		targetID, actor.PlayerID, kind, body)
	return err
}

func Notes(ctx context.Context, pool *pgxpool.Pool, targetID int64) ([]Note, error) {
	rows, err := pool.Query(ctx, `
		SELECT n.kind, n.body, COALESCE(NULLIF(a.name, ''), NULLIF(a.steam_name, ''), 'Player #' || a.id, 'Unknown'), n.created_at
		FROM staff_notes n LEFT JOIN players a ON a.id = n.author_id
		WHERE n.player_id = $1 ORDER BY n.created_at DESC LIMIT 100`, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.Kind, &n.Body, &n.Author, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// History returns the player's rank/status/team change history from
// rank_changes, newest first, with rank IDs resolved to names.
func History(ctx context.Context, pool *pgxpool.Pool, targetID int64) ([]Change, error) {
	rows, err := pool.Query(ctx, `
		SELECT rc.field,
		       tasdyn_change_label(rc.field, rc.old_value), tasdyn_change_label(rc.field, rc.new_value),
		       rc.source,
		       COALESCE(NULLIF(a.name, ''), NULLIF(a.steam_name, ''), 'Player #' || a.id,
		                CASE rc.source WHEN 'game' THEN 'In-game' WHEN 'manual' THEN 'Manual DB change' ELSE 'System' END),
		       COALESCE(rc.reason, ''), rc.changed_at
		FROM rank_changes rc LEFT JOIN players a ON a.id = rc.actor_id
		WHERE rc.player_id = $1 AND rc.field <> 'discord_id'
		ORDER BY rc.changed_at DESC LIMIT 100`, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Change
	for rows.Next() {
		var c Change
		if err := rows.Scan(&c.Field, &c.Old, &c.New, &c.Source, &c.Actor, &c.Reason, &c.ChangedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// EndExpiredLOAs returns staff whose LOA end date has passed to 'active'.
// Suspensions are deliberately not auto-lifted (GAMEPANEL_PARITY §2.3).
func EndExpiredLOAs(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if err := audit.SetActor(ctx, tx, 0, audit.SourceWebsite, "LOA end date reached"); err != nil {
		return 0, err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE players SET staff_status = 'active', staff_status_reason = NULL, staff_status_until = NULL
		WHERE staff_status = 'loa' AND staff_status_until IS NOT NULL AND staff_status_until <= now()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), tx.Commit(ctx)
}

// RunLOASweep calls EndExpiredLOAs every 5 minutes until ctx is cancelled.
func RunLOASweep(ctx context.Context, pool *pgxpool.Pool, logf func(n int64, err error)) {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		n, err := EndExpiredLOAs(ctx, pool)
		logf(n, err)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
