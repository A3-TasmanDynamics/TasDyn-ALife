// Package gamecmd lets staff act on players in game from the admin panel:
// a private message, a warning, a kick, or an announcement to everyone.
// The website checks the permission (the same ones as the in-game admin
// menu), queues the command (game_commands) and writes the Staff Log; the
// game server claims it within a few seconds and records the outcome
// (ALife_fnc_gameCommands, the extension's "gamecmd").
package gamecmd

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

// ErrNotAllowed wraps a reason the command can't be sent, for the page.
var ErrNotAllowed = errors.New("not allowed")

func notAllowed(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrNotAllowed, fmt.Sprintf(format, a...))
}

// Actions and the permission each needs (as in the in-game admin menu).
var Perms = map[string]string{
	"message":  "game.admin",
	"warn":     "game.warn",
	"kick":     "game.kick",
	"announce": "announce.post",
}

// Expiry is how long a command waits for the game server before expiring
// (the extension expires it; the page shows it).
const Expiry = 2 * time.Minute

// Issue queues a command. target is the player's id (0 for announce).
func Issue(ctx context.Context, pool *pgxpool.Pool, staffID int64, action string, target int64, text string) (int64, error) {
	if _, ok := Perms[action]; !ok {
		return 0, notAllowed("unknown action")
	}
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return 0, notAllowed("write a message")
	}
	if len([]rune(text)) > 300 {
		return 0, notAllowed("keep it under 300 characters")
	}
	if action == "announce" {
		target = 0
	} else {
		if target <= 0 {
			return 0, notAllowed("choose a player")
		}
		if target == staffID && action != "message" {
			return 0, notAllowed("you can't %s yourself", action)
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if target > 0 {
		var online bool
		err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM player_sessions WHERE player_id = $1 AND disconnected_at IS NULL)`, target).Scan(&online)
		if err != nil {
			return 0, err
		}
		if !online {
			return 0, notAllowed("they aren't in game right now")
		}
	}
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO game_commands (action, target_player_id, text, issued_by) VALUES ($1, NULLIF($2, 0), $3, $4) RETURNING id`,
		action, target, text, staffID).Scan(&id); err != nil {
		return 0, err
	}
	logAction := map[string]string{"message": "game.message", "warn": "game.warn", "kick": "player.kick", "announce": "game.announce"}[action]
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{StaffID: staffID, TargetID: target, Action: logAction, Reason: text,
		After: map[string]any{"via": "admin panel", "command": id}}); err != nil {
		return 0, err
	}
	if action == "kick" {
		if _, err := tx.Exec(ctx, `INSERT INTO kick_log (target_player_id, kicked_by, kick_type, reason) VALUES ($1, $2, 'manual', $3)`, target, staffID, text); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit(ctx)
}

// Command is a queued command, for the page.
type Command struct {
	ID       int64
	Action   string
	Target   string // "" for announce
	TargetID int64
	Text     string
	By       string
	Status   string // pending / claimed / done / failed / expired
	Result   string
	Created  time.Time
}

// Recent is the last n commands, newest first. Ones still pending past
// the expiry show as expired even before the game server tidies them.
func Recent(ctx context.Context, pool *pgxpool.Pool, n int) ([]Command, error) {
	rows, err := pool.Query(ctx, `
		SELECT g.id, g.action, COALESCE(NULLIF(t.name, ''), NULLIF(t.steam_name, ''), ''), COALESCE(g.target_player_id, 0), g.text,
		       COALESCE(NULLIF(s.name, ''), NULLIF(s.steam_name, ''), 'Staff'),
		       CASE WHEN g.status = 'pending' AND g.created_at < now() - make_interval(secs => $2) THEN 'expired' ELSE g.status END,
		       COALESCE(g.result, ''), g.created_at
		FROM game_commands g
		LEFT JOIN players t ON t.id = g.target_player_id
		LEFT JOIN players s ON s.id = g.issued_by
		ORDER BY g.id DESC LIMIT $1`, n, Expiry.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Command
	for rows.Next() {
		var c Command
		if err := rows.Scan(&c.ID, &c.Action, &c.Target, &c.TargetID, &c.Text, &c.By, &c.Status, &c.Result, &c.Created); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Get is one command's status (for the page to follow it).
func Get(ctx context.Context, pool *pgxpool.Pool, id int64) (Command, bool, error) {
	var c Command
	err := pool.QueryRow(ctx, `
		SELECT id, action, text, CASE WHEN status = 'pending' AND created_at < now() - make_interval(secs => $2) THEN 'expired' ELSE status END,
		       COALESCE(result, ''), created_at
		FROM game_commands WHERE id = $1`, id, Expiry.Seconds()).Scan(&c.ID, &c.Action, &c.Text, &c.Status, &c.Result, &c.Created)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, false, nil
	}
	return c, err == nil, err
}
