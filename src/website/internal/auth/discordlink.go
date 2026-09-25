package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrDiscordAlreadyLinked means the Discord account in question is already
// linked to a DIFFERENT player -- a deliberate conflict, not a bug, so
// callers can show "that Discord account is already connected to another
// player" instead of a generic 500.
var ErrDiscordAlreadyLinked = errors.New("auth: discord account already linked to another player")

// ErrLinkCodeInvalid covers an unknown, expired, or already-used code --
// deliberately not distinguished further in the error a player sees, so a
// guess doesn't leak which case it was.
var ErrLinkCodeInvalid = errors.New("auth: link code invalid or expired")

const linkCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0/O/1/I/L -- typed by hand from Discord

// GenerateLinkCode creates a one-time code a player can redeem with the
// Discord bot's `/link` command (docs/WEBSITE.md §9) -- the mirror-image of
// the member portal's OAuth "Connect Discord" button (discord.go), for a
// player who'd rather start from Discord than from the website.
func GenerateLinkCode(ctx context.Context, pool *pgxpool.Pool, playerID int64) (code string, expiresAt time.Time, err error) {
	buf := make([]byte, 8)
	if _, err = rand.Read(buf); err != nil {
		return "", time.Time{}, err
	}
	b := make([]byte, 8)
	for i, v := range buf {
		b[i] = linkCodeAlphabet[int(v)%len(linkCodeAlphabet)]
	}
	code = string(b)
	expiresAt = time.Now().Add(10 * time.Minute)

	_, err = pool.Exec(ctx, `
		INSERT INTO discord_link_codes (code, player_id, expires_at) VALUES ($1, $2, $3)
	`, code, playerID, expiresAt)
	if err != nil {
		return "", time.Time{}, err
	}
	return code, expiresAt, nil
}

// ConsumeLinkCode redeems a code (as typed into the Discord `/link`
// command) and links the given Discord identity to whichever player
// generated it. Atomic: the UPDATE...RETURNING both claims the code
// (used_at IS NULL AND expires_at > now(), so a second concurrent redemption
// attempt can't also succeed) and identifies the player in one round trip.
func ConsumeLinkCode(ctx context.Context, pool *pgxpool.Pool, code, discordID, discordUsername string) (playerID int64, err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		UPDATE discord_link_codes
		SET used_at = now()
		WHERE code = $1 AND used_at IS NULL AND expires_at > now()
		RETURNING player_id
	`, code).Scan(&playerID)
	if err == pgx.ErrNoRows {
		return 0, ErrLinkCodeInvalid
	}
	if err != nil {
		return 0, err
	}

	if err := linkDiscordTx(ctx, tx, playerID, discordID, discordUsername); err != nil {
		return 0, err
	}

	return playerID, tx.Commit(ctx)
}

// LinkDiscordToPlayer is the OAuth2 "Connect Discord" path's counterpart to
// ConsumeLinkCode above -- same outcome (players.discord_id gets set),
// reached from the opposite direction (an already-authenticated website
// session proving Discord identity via OAuth, instead of a Discord command
// proving website identity via a code).
func LinkDiscordToPlayer(ctx context.Context, pool *pgxpool.Pool, playerID int64, discordID, discordUsername string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := linkDiscordTx(ctx, tx, playerID, discordID, discordUsername); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func linkDiscordTx(ctx context.Context, tx pgx.Tx, playerID int64, discordID, discordUsername string) error {
	_, err := tx.Exec(ctx, `
		UPDATE players SET discord_id = $1, discord_username = $2 WHERE id = $3
	`, discordID, discordUsername, playerID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDiscordAlreadyLinked
		}
		return fmt.Errorf("auth: linking discord account: %w", err)
	}
	return nil
}
