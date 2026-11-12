// Package schedule sends in-game announcements on a schedule: one-off or
// repeating messages staff set up on /admin/schedule, and restart
// countdowns ("Server restart in 10 minutes") -- queued by Server Control
// when a restart is scheduled, or by hand. A loop in the website turns due
// messages into game_commands announcements (internal/gamecmd), which the
// game server shows within a few seconds.
package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
)

// ErrNotAllowed wraps a reason it can't be scheduled.
var ErrNotAllowed = errors.New("not allowed")

func notAllowed(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrNotAllowed, fmt.Sprintf(format, a...))
}

// Stale is how late a message can be and still go out: if the website was
// down for longer, a missed one is skipped rather than sent late.
const Stale = 5 * time.Minute

// Countdown is when restart warnings go out, in minutes before.
var Countdown = []int{15, 10, 5, 1}

type Message struct {
	ID        int64
	Kind      string // announce / restart
	Text      string
	NextAt    time.Time
	Repeat    int // minutes; 0 = once
	Until     *time.Time
	Active    bool
	By        string
	LastSent  *time.Time
	SentCount int
}

// Add schedules a message: at `at`, then every `repeat` minutes (0 = once)
// until `until` (nil = no end).
func Add(ctx context.Context, pool *pgxpool.Pool, staffID int64, text string, at time.Time, repeat int, until *time.Time) (int64, error) {
	text = strings.Join(strings.Fields(text), " ")
	switch {
	case text == "" || len([]rune(text)) > 300:
		return 0, notAllowed("write the message (up to 300 characters)")
	case at.Before(time.Now().Add(-time.Minute)):
		return 0, notAllowed("pick a time that hasn't passed")
	case repeat != 0 && (repeat < 5 || repeat > 10080):
		return 0, notAllowed("repeat every 5 minutes to 7 days")
	case until != nil && !until.After(at):
		return 0, notAllowed("the end has to be after the first time")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO scheduled_messages (text, next_at, repeat_minutes, until_at, created_by) VALUES ($1, $2, NULLIF($3, 0), $4, $5) RETURNING id`,
		text, at, repeat, until, staffID).Scan(&id); err != nil {
		return 0, err
	}
	after := map[string]any{"text": text, "at": at.Format(time.RFC3339)}
	if repeat > 0 {
		after["every_minutes"] = repeat
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{StaffID: staffID, Action: "schedule.add", Reason: text, After: after}); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

// QueueCountdown schedules the restart warnings that are still ahead for a
// restart at `at` (replacing any earlier countdown).
func QueueCountdown(ctx context.Context, pool *pgxpool.Pool, staffID int64, at time.Time, reason string) (int, error) {
	reason = strings.Join(strings.Fields(reason), " ")
	if len([]rune(reason)) > 200 {
		reason = string([]rune(reason)[:200])
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM scheduled_messages WHERE kind = 'restart' AND active`); err != nil {
		return 0, err
	}
	n := 0
	for _, m := range Countdown {
		when := at.Add(-time.Duration(m) * time.Minute)
		if when.Before(time.Now().Add(-30 * time.Second)) {
			continue
		}
		text := fmt.Sprintf("Server restart in %d minute%s. Find a safe spot and log off before then.", m, map[bool]string{true: "", false: "s"}[m == 1])
		if reason != "" {
			text = fmt.Sprintf("Server restart in %d minute%s (%s). Find a safe spot and log off before then.", m, map[bool]string{true: "", false: "s"}[m == 1], reason)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO scheduled_messages (kind, text, next_at, created_by) VALUES ('restart', $1, $2, NULLIF($3, 0))`, text, when, staffID); err != nil {
			return 0, err
		}
		n++
	}
	return n, tx.Commit(ctx)
}

// CancelCountdown removes restart warnings that haven't gone out.
func CancelCountdown(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	tag, err := pool.Exec(ctx, `DELETE FROM scheduled_messages WHERE kind = 'restart' AND active`)
	return int(tag.RowsAffected()), err
}

// SetActive pauses or resumes a message.
func SetActive(ctx context.Context, pool *pgxpool.Pool, id int64, active bool) error {
	tag, err := pool.Exec(ctx, `
		UPDATE scheduled_messages SET active = $2,
		  -- resuming a repeating one that fell behind: its next time is the next step from now
		  next_at = CASE WHEN $2 AND repeat_minutes IS NOT NULL AND next_at < now()
		                 THEN next_at + make_interval(mins => repeat_minutes * (floor(EXTRACT(EPOCH FROM now() - next_at) / 60 / repeat_minutes)::int + 1))
		                 ELSE next_at END
		WHERE id = $1 AND (NOT $2 OR repeat_minutes IS NOT NULL OR (sent_count = 0 AND next_at > now()))`, id, active)
	if err == nil && tag.RowsAffected() == 0 {
		return notAllowed("that message can't be changed (it may have gone out already)")
	}
	return err
}

// Delete removes a message (logged).
func Delete(ctx context.Context, pool *pgxpool.Pool, staffID, id int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var text string
	err = tx.QueryRow(ctx, `DELETE FROM scheduled_messages WHERE id = $1 RETURNING text`, id).Scan(&text)
	if errors.Is(err, pgx.ErrNoRows) {
		return notAllowed("that message doesn't exist any more")
	}
	if err != nil {
		return err
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{StaffID: staffID, Action: "schedule.remove", Reason: text}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// List is the scheduled messages: active first by next time, then the
// finished / paused ones, newest first.
func List(ctx context.Context, pool *pgxpool.Pool) ([]Message, error) {
	rows, err := pool.Query(ctx, `
		SELECT s.id, s.kind, s.text, s.next_at, COALESCE(s.repeat_minutes, 0), s.until_at, s.active,
		       COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'System'), s.last_sent_at, s.sent_count
		FROM scheduled_messages s LEFT JOIN players p ON p.id = s.created_by
		ORDER BY s.active DESC, CASE WHEN s.active THEN s.next_at END, s.created_at DESC
		LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Kind, &m.Text, &m.NextAt, &m.Repeat, &m.Until, &m.Active, &m.By, &m.LastSent, &m.SentCount); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SendDue queues every due message as an announcement and moves each on to
// its next time (or finishes it). It reports how many went out.
func SendDue(ctx context.Context, pool *pgxpool.Pool, now time.Time) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `
		SELECT id, text, next_at, COALESCE(repeat_minutes, 0), until_at, created_by
		FROM scheduled_messages WHERE active AND next_at <= $1 ORDER BY next_at FOR UPDATE SKIP LOCKED`, now)
	if err != nil {
		return 0, err
	}
	type due struct {
		id     int64
		text   string
		at     time.Time
		repeat int
		until  *time.Time
		by     *int64
	}
	var list []due
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.id, &d.text, &d.at, &d.repeat, &d.until, &d.by); err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	sent := 0
	for _, d := range list {
		fresh := now.Sub(d.at) <= Stale
		if fresh {
			if _, err := tx.Exec(ctx, `INSERT INTO game_commands (action, text, issued_by) VALUES ('announce', $1, $2)`, d.text, d.by); err != nil {
				return 0, err
			}
			sent++
		}
		next, active := d.at, false
		if d.repeat > 0 {
			step := time.Duration(d.repeat) * time.Minute
			for !next.After(now) {
				next = next.Add(step)
			}
			active = d.until == nil || !next.After(*d.until)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE scheduled_messages SET next_at = $2, active = $3,
			  last_sent_at = CASE WHEN $4 THEN $5 ELSE last_sent_at END, sent_count = sent_count + CASE WHEN $4 THEN 1 ELSE 0 END
			WHERE id = $1`, d.id, next, active, fresh, now); err != nil {
			return 0, err
		}
	}
	return sent, tx.Commit(ctx)
}

// Run sends due messages every 15 s until ctx ends.
func Run(ctx context.Context, pool *pgxpool.Pool) {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		if n, err := SendDue(ctx, pool, time.Now()); err != nil && ctx.Err() == nil {
			slog.Error("schedule: sending due messages failed (is database/fixes/2026-11-13_scheduled_messages.sql applied?)", "error", err)
		} else if n > 0 {
			slog.Info("schedule: announcements queued", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
