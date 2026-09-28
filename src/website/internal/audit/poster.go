package audit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/discord/webhook"
)

// Poster sends staff_log rows to the #staff-log webhook. It wakes on the
// 'staff_log' NOTIFY from the database trigger and also polls every
// minute, and marks a row posted only after Discord accepts it -- so rows
// written by any source (website, bot, game, psql) get posted exactly
// once, even across restarts or a Discord outage.
type Poster struct {
	Pool       *pgxpool.Pool
	WebhookURL string
	// MaxAge overrides the default posting window (tests only).
	MaxAge time.Duration
}

// defaultMaxAge: rows older than this are never posted -- e.g. when a webhook is
// first configured, the channel isn't flooded with weeks of history.
const defaultMaxAge = 24 * time.Hour

func (p *Poster) Run(ctx context.Context) {
	if p.WebhookURL == "" {
		slog.Info("audit: DISCORD_STAFF_LOG_WEBHOOK not set, #staff-log posting disabled")
		return
	}
	backoff := time.Second
	for ctx.Err() == nil {
		err := p.listen(ctx)
		if ctx.Err() != nil {
			return
		}
		slog.Warn("audit: staff-log listener stopped, reconnecting", "error", err, "in", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < time.Minute {
			backoff *= 2
		}
	}
}

func (p *Poster) listen(ctx context.Context) error {
	conn, err := p.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `LISTEN staff_log`); err != nil {
		return err
	}
	for {
		p.postPending(ctx)
		waitCtx, cancel := context.WithTimeout(ctx, time.Minute)
		_, err := conn.Conn().WaitForNotification(waitCtx)
		cancel()
		if err != nil && ctx.Err() == nil && waitCtx.Err() == nil {
			return err // the connection itself failed, not just the poll timeout
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

type pendingRow struct {
	id          int64
	staff       string
	target      string
	action      string
	reason      string
	before      []byte
	after       []byte
	source      string
}

func (p *Poster) postPending(ctx context.Context) {
	maxAge := p.MaxAge
	if maxAge == 0 {
		maxAge = defaultMaxAge
	}
	rows, err := p.Pool.Query(ctx, `
		SELECT sl.id,
		       COALESCE(NULLIF(s.name, ''), NULLIF(s.steam_name, ''), 'Player #' || s.id::text, ''),
		       COALESCE(NULLIF(t.name, ''), NULLIF(t.steam_name, ''), 'Player #' || t.id::text, ''),
		       sl.action, COALESCE(sl.reason, ''), sl.before_value, sl.after_value, sl.source
		FROM staff_log sl
		LEFT JOIN players s ON s.id = sl.staff_player_id
		LEFT JOIN players t ON t.id = sl.target_player_id
		WHERE sl.discord_posted_at IS NULL AND sl.created_at > now() - make_interval(secs => $1)
		ORDER BY sl.id
		LIMIT 25
	`, maxAge.Seconds())
	if err != nil {
		slog.Error("audit: pending staff-log query failed", "error", err)
		return
	}
	var pending []pendingRow
	for rows.Next() {
		var r pendingRow
		if err := rows.Scan(&r.id, &r.staff, &r.target, &r.action, &r.reason, &r.before, &r.after, &r.source); err == nil {
			pending = append(pending, r)
		}
	}
	rows.Close()

	for _, r := range pending {
		if err := webhook.Post(ctx, p.WebhookURL, Format(r.staff, r.target, r.action, r.reason, r.before, r.after, r.source)); err != nil {
			slog.Warn("audit: posting to #staff-log failed, will retry", "staff_log_id", r.id, "error", err)
			return // keep order; retry from this row next time
		}
		if _, err := p.Pool.Exec(ctx, `UPDATE staff_log SET discord_posted_at = now() WHERE id = $1`, r.id); err != nil {
			slog.Error("audit: marking staff-log row posted failed", "staff_log_id", r.id, "error", err)
			return
		}
	}
}

// Format renders one staff_log row as a #staff-log line. Names and reasons
// are player/staff-supplied, so they're markdown-escaped (mentions are
// already disabled by webhook.Post).
func Format(staff, target, action, reason string, before, after []byte, source string) string {
	who := "**" + webhook.Escape(staff) + "**"
	switch {
	case staff != "":
	case source == SourceGame:
		who = "**In-game**"
	case source == SourceManual:
		who = "⚠️ **Manual database change**"
	default:
		who = "**System**"
	}
	msg := who + " " + webhook.Escape(Describe(action, before, after))
	if target != "" {
		msg += " for **" + webhook.Escape(target) + "**"
	}
	if reason != "" {
		msg += " — " + webhook.Escape(reason)
	}
	return fmt.Sprintf("%s · _via %s_", msg, source)
}
