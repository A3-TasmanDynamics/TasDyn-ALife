package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The outbox (docs/DISCORD_BOT.md §8) delivers messages a person must
// actually receive -- a ban notice, an application decision -- reliably.
// Enqueue writes a row inside the caller's transaction, so if the action
// commits the message is guaranteed to be queued; the Outbox worker sends
// it, retrying with backoff, even if the bot was offline at the time.

// Message kinds.
const (
	KindDM          = "dm"           // Target = Discord user ID
	KindChannelPost = "channel_post" // Target = channel ID
)

// OutboxMessage is one queued message.
type OutboxMessage struct {
	Kind    string
	Target  string
	Content string
	// DedupeKey, if set, makes Enqueue a no-op when the same key was already
	// queued -- e.g. "ban-notice:9123" -- so a retried action can't double-send.
	DedupeKey string
}

type outboxPayload struct {
	Content string `json:"content"`
}

// Enqueue queues m inside tx.
func Enqueue(ctx context.Context, tx pgx.Tx, m OutboxMessage) error {
	if m.Kind != KindDM && m.Kind != KindChannelPost {
		return fmt.Errorf("discord outbox: unknown kind %q", m.Kind)
	}
	if m.Target == "" {
		return errors.New("discord outbox: empty target")
	}
	payload, err := json.Marshal(outboxPayload{Content: m.Content})
	if err != nil {
		return err
	}
	var dedupe *string
	if m.DedupeKey != "" {
		dedupe = &m.DedupeKey
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO discord_outbox (kind, target, payload, dedupe_key) VALUES ($1, $2, $3, $4)
		ON CONFLICT (dedupe_key) DO NOTHING
	`, m.Kind, m.Target, payload, dedupe)
	return err
}

// Sender is the part of a discordgo session the worker needs -- an
// interface so the worker can be tested without Discord.
type Sender interface {
	UserChannelCreate(recipientID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Message, error)
}

type Outbox struct {
	Pool   *pgxpool.Pool
	Sender Sender
}

const (
	outboxBatch    = 20
	outboxGiveUp   = 24 * time.Hour
	discordDMClosed = 50007 // "Cannot send messages to this user"
)

// Backlog counts messages not yet delivered or given up on.
func Backlog(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `SELECT count(*) FROM discord_outbox WHERE sent_at IS NULL AND gave_up_at IS NULL`).Scan(&n)
	return n, err
}

// Run delivers due messages every few seconds until ctx is cancelled.
func (o *Outbox) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		o.deliverDue(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (o *Outbox) deliverDue(ctx context.Context) {
	rows, err := o.Pool.Query(ctx, `
		SELECT id, kind, target, payload, attempts, created_at FROM discord_outbox
		WHERE sent_at IS NULL AND gave_up_at IS NULL AND next_attempt <= now()
		ORDER BY id LIMIT $1
	`, outboxBatch)
	if err != nil {
		slog.Error("discord outbox: query failed", "error", err)
		return
	}
	type due struct {
		id        int64
		kind      string
		target    string
		payload   []byte
		attempts  int
		createdAt time.Time
	}
	var batch []due
	for rows.Next() {
		var d due
		if rows.Scan(&d.id, &d.kind, &d.target, &d.payload, &d.attempts, &d.createdAt) == nil {
			batch = append(batch, d)
		}
	}
	rows.Close()

	for _, d := range batch {
		err := o.send(d.kind, d.target, d.payload)
		switch {
		case err == nil:
			_, err = o.Pool.Exec(ctx, `UPDATE discord_outbox SET sent_at = now(), attempts = attempts + 1, last_error = NULL WHERE id = $1`, d.id)
		case isPermanent(err):
			_, err = o.Pool.Exec(ctx, `UPDATE discord_outbox SET gave_up_at = now(), attempts = attempts + 1, last_error = $2 WHERE id = $1`, d.id, err.Error())
		case time.Since(d.createdAt) > outboxGiveUp:
			slog.Warn("discord outbox: giving up after 24h", "id", d.id, "error", err)
			_, err = o.Pool.Exec(ctx, `UPDATE discord_outbox SET gave_up_at = now(), attempts = attempts + 1, last_error = $2 WHERE id = $1`, d.id, err.Error())
		default:
			delay := time.Duration(math.Min(float64(time.Hour), float64(time.Duration(1<<min(d.attempts, 12))*time.Second)))
			_, err = o.Pool.Exec(ctx, `
				UPDATE discord_outbox SET attempts = attempts + 1, last_error = $2, next_attempt = now() + make_interval(secs => $3)
				WHERE id = $1`, d.id, err.Error(), delay.Seconds())
		}
		if err != nil {
			slog.Error("discord outbox: updating row failed", "id", d.id, "error", err)
		}
	}
}

func (o *Outbox) send(kind, target string, rawPayload []byte) error {
	var p outboxPayload
	if err := json.Unmarshal(rawPayload, &p); err != nil {
		return permanentError{err}
	}
	msg := &discordgo.MessageSend{
		Content:         p.Content,
		AllowedMentions: &discordgo.MessageAllowedMentions{}, // never ping from queued content
	}
	channelID := target
	if kind == KindDM {
		ch, err := o.Sender.UserChannelCreate(target)
		if err != nil {
			return classify(err)
		}
		channelID = ch.ID
	}
	_, err := o.Sender.ChannelMessageSendComplex(channelID, msg)
	return classify(err)
}

type permanentError struct{ error }

func isPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}

// classify marks errors that retrying can't fix as permanent: DMs closed,
// unknown channel/user, missing access.
func classify(err error) error {
	if err == nil {
		return nil
	}
	var rest *discordgo.RESTError
	if errors.As(err, &rest) {
		if rest.Message != nil && rest.Message.Code == discordDMClosed {
			return permanentError{fmt.Errorf("DMs closed")}
		}
		if rest.Response != nil {
			switch rest.Response.StatusCode {
			case http.StatusForbidden, http.StatusNotFound:
				return permanentError{fmt.Errorf("discord HTTP %d", rest.Response.StatusCode)}
			}
		}
	}
	return err
}
