// Package watchlist is the staff watchlist: players staff want to keep an
// eye on, with why. The admin panel tags them wherever they appear and
// lists them on /admin/watchlist; the Discord bot posts to #staff-alerts
// when one joins or moves money (internal/discord/watch_alerts.go).
// Adding and removing is logged to the staff log.
package watchlist

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

// ErrNotAllowed wraps a reason the change can't be made, for the page.
var ErrNotAllowed = errors.New("not allowed")

func notAllowed(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrNotAllowed, fmt.Sprintf(format, a...))
}

const playerName = `COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id)`

// Actor is the staff member making a change.
type Actor struct {
	PlayerID int64
	Source   string // audit.SourceWebsite / audit.SourceDiscord
}

// Entry is one watched player.
type Entry struct {
	PlayerID  int64
	Name, UID string
	Reason    string
	AddedBy   string // "" when the staff member's account is gone
	AddedAt   time.Time
	FlagID    int64 // the anti-cheat flag it came from, 0 if none
	Online    bool
	LastSeen  *time.Time
	OpenFlags int
	Bounty    int64
	Events7d  int // event log entries in the last 7 days
}

// List is every watched player, online first, then most recently added.
// search matches the name, Steam ID or reason.
func List(ctx context.Context, pool *pgxpool.Pool, search string) ([]Entry, error) {
	rows, err := pool.Query(ctx, `
		SELECT p.id, `+playerName+`, p.uid, w.reason, COALESCE(NULLIF(s.name, ''), NULLIF(s.steam_name, ''), ''), w.added_at, COALESCE(w.flag_id, 0),
		       EXISTS (SELECT 1 FROM player_sessions ps WHERE ps.player_id = p.id AND ps.disconnected_at IS NULL) AS is_on,
		       p.last_seen,
		       (SELECT count(*) FROM anti_cheat_flags f WHERE f.player_id = p.id AND f.resolution IS NULL)::int,
		       COALESCE((SELECT sum(c.bounty_amount) FROM wanted_crimes c WHERE c.player_id = p.id AND c.cleared_at IS NULL), 0)::bigint,
		       (SELECT count(*) FROM player_log l WHERE l.player_id = p.id AND l.created_at > now() - interval '7 days')::int
		FROM player_watchlist w JOIN players p ON p.id = w.player_id
		LEFT JOIN players s ON s.id = w.added_by
		WHERE $1 = '' OR p.name ILIKE '%' || $1 || '%' OR p.steam_name ILIKE '%' || $1 || '%' OR p.uid = $1 OR w.reason ILIKE '%' || $1 || '%'
		ORDER BY is_on DESC, w.added_at DESC`, strings.TrimSpace(search))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.PlayerID, &e.Name, &e.UID, &e.Reason, &e.AddedBy, &e.AddedAt, &e.FlagID,
			&e.Online, &e.LastSeen, &e.OpenFlags, &e.Bounty, &e.Events7d); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Get is one player's watchlist entry, if they're on it.
func Get(ctx context.Context, pool *pgxpool.Pool, playerID int64) (Entry, bool, error) {
	var e Entry
	err := pool.QueryRow(ctx, `
		SELECT w.player_id, w.reason, COALESCE(NULLIF(s.name, ''), NULLIF(s.steam_name, ''), ''), w.added_at, COALESCE(w.flag_id, 0)
		FROM player_watchlist w LEFT JOIN players s ON s.id = w.added_by
		WHERE w.player_id = $1`, playerID).Scan(&e.PlayerID, &e.Reason, &e.AddedBy, &e.AddedAt, &e.FlagID)
	if errors.Is(err, pgx.ErrNoRows) {
		return e, false, nil
	}
	return e, err == nil, err
}

// Reasons maps every watched player's id to the reason, for tagging lists.
func Reasons(ctx context.Context, pool *pgxpool.Pool) (map[int64]string, error) {
	rows, err := pool.Query(ctx, `SELECT player_id, reason FROM player_watchlist`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var r string
		if err := rows.Scan(&id, &r); err != nil {
			return nil, err
		}
		out[id] = r
	}
	return out, rows.Err()
}

// Count is how many players are watched.
func Count(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `SELECT count(*) FROM player_watchlist`).Scan(&n)
	return n, err
}

// FindPlayer resolves a Steam ID, "#123" / "123" (player id) or an exact
// name to a player id.
func FindPlayer(ctx context.Context, pool *pgxpool.Pool, q string) (int64, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return 0, notAllowed("say who: a Steam ID, player number or exact name")
	}
	var ids []int64
	rows, err := pool.Query(ctx, `
		SELECT id FROM players
		WHERE uid = $1 OR id::text = ltrim($1, '#') OR lower(name) = lower($1) OR lower(steam_name) = lower($1)
		LIMIT 2`, q)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	switch len(ids) {
	case 0:
		return 0, notAllowed("no player matches %q", q)
	case 1:
		return ids[0], nil
	}
	return 0, notAllowed("more than one player matches %q; use their Steam ID", q)
}

// Add puts a player on the watchlist, or changes the reason if they're
// already on it.
func Add(ctx context.Context, pool *pgxpool.Pool, actor Actor, playerID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 500 {
		return notAllowed("give a reason (up to 500 characters)")
	}
	if playerID == actor.PlayerID {
		return notAllowed("you can't put yourself on the watchlist")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var exists bool
	var before *string
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM players WHERE id = $1)`, playerID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return notAllowed("that player doesn't exist")
	}
	err = tx.QueryRow(ctx, `SELECT reason FROM player_watchlist WHERE player_id = $1 FOR UPDATE`, playerID).Scan(&before)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if before != nil && *before == reason {
		return nil // nothing changed
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO player_watchlist (player_id, reason, added_by) VALUES ($1, $2, NULLIF($3, 0))
		ON CONFLICT (player_id) DO UPDATE SET reason = EXCLUDED.reason`, playerID, reason, actor.PlayerID); err != nil {
		return err
	}
	action, entry := "watchlist.add", audit.Entry{StaffID: actor.PlayerID, TargetID: playerID, Source: actor.Source, Reason: reason}
	if before != nil {
		action = "watchlist.edit"
		entry.Before = map[string]string{"reason": *before}
	}
	entry.Action = action
	entry.After = map[string]string{"reason": reason}
	if err := audit.LogStaffAction(ctx, tx, entry); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// AddFromFlag puts the flag's player on the watchlist inside a flag review
// (anti-cheat "watch"), unless they're already on it. It reports whether it
// added them.
func AddFromFlag(ctx context.Context, tx pgx.Tx, actorID, playerID, flagID int64, reason string) (bool, error) {
	if len(reason) > 500 {
		reason = reason[:497] + "..."
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO player_watchlist (player_id, reason, added_by, flag_id) VALUES ($1, $2, NULLIF($3, 0), $4)
		ON CONFLICT (player_id) DO NOTHING`, playerID, reason, actorID, flagID)
	return err == nil && tag.RowsAffected() == 1, err
}

// Remove takes a player off the watchlist. note says why (optional).
func Remove(ctx context.Context, pool *pgxpool.Pool, actor Actor, playerID int64, note string) error {
	note = strings.TrimSpace(note)
	if len(note) > 500 {
		return notAllowed("the note is too long (500 characters max)")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var reason string
	err = tx.QueryRow(ctx, `DELETE FROM player_watchlist WHERE player_id = $1 RETURNING reason`, playerID).Scan(&reason)
	if errors.Is(err, pgx.ErrNoRows) {
		return notAllowed("that player isn't on the watchlist")
	}
	if err != nil {
		return err
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{StaffID: actor.PlayerID, TargetID: playerID, Action: "watchlist.remove",
		Source: actor.Source, Reason: note, Before: map[string]string{"reason": reason}}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
