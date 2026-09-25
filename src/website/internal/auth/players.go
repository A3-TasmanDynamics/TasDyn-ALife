package auth

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FindOrCreatePlayerBySteamUID resolves a players.id for a Steam64 ID,
// creating the row if this is the account's first-ever login. uid is the
// same Steam64 ID Arma's getPlayerUID returns in-game -- see
// docs/DATA_CONTRACT.md -- so a player who has played before already has a
// row here, and the website simply attaches to it rather than creating a
// second, disconnected identity.
//
// A brand-new row also gets one bank_accounts row per faction, seeded to
// 0 -- the same thing src/cpp_extension/src/db.cpp's EnsureBlankPlayer does
// on a player's first in-game load. Without this, a player who signs up on
// the website BEFORE ever connecting in-game would have no bank_accounts
// rows at all, and internal/bank's transfers (which require one) would
// fail for them -- website-first signup is a real path now, not
// hypothetical, so it has to seed the same rows the game-first path does.
func FindOrCreatePlayerBySteamUID(ctx context.Context, pool *pgxpool.Pool, uid string) (playerID int64, err error) {
	err = pool.QueryRow(ctx, `SELECT id FROM players WHERE uid = $1`, uid).Scan(&playerID)
	if err == nil {
		return playerID, nil
	}
	if err != pgx.ErrNoRows {
		return 0, err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		INSERT INTO players (uid, status) VALUES ($1, 'active') RETURNING id
	`, uid).Scan(&playerID)
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO bank_accounts (player_id, faction, balance)
		SELECT $1, f, 0 FROM unnest(ARRAY['civilian', 'police', 'medic']) AS f
	`, playerID)
	if err != nil {
		return 0, err
	}

	return playerID, tx.Commit(ctx)
}

// FindPlayerByDiscordID looks up a player who has already linked the given
// Discord account -- used by the "Login with Discord" path, which can only
// ever be a second login method for an account that already exists via
// Steam (see docs/WEBSITE.md §3: Steam identity is what creates a player
// row at all).
func FindPlayerByDiscordID(ctx context.Context, pool *pgxpool.Pool, discordID string) (playerID int64, found bool, err error) {
	err = pool.QueryRow(ctx, `SELECT id FROM players WHERE discord_id = $1`, discordID).Scan(&playerID)
	if err == pgx.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return playerID, true, nil
}
