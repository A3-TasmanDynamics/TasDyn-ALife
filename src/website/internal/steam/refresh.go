package steam

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Refresher keeps players' cached Steam profile and ban data current: on
// every login (RefreshOne) and via a background sweep of anyone not
// refreshed in the last day (Run). With no API key configured both are
// no-ops -- cached values simply stay as they are.
type Refresher struct {
	Pool   *pgxpool.Pool
	Client *Client
}

const staleAfter = 24 * time.Hour

// RefreshOne refreshes a single player. Meant to be called on login, in a
// goroutine -- it must never delay or fail the login itself.
func (r *Refresher) RefreshOne(ctx context.Context, steam64 string) {
	if err := r.refresh(ctx, []string{steam64}); err != nil && !errors.Is(err, ErrNoAPIKey) {
		slog.Warn("steam: refresh on login failed", "error", err)
	}
}

// Refresh refreshes a single player now and reports the error -- the
// profile page's "re-sync Steam" button. ErrNoAPIKey means it's off.
func (r *Refresher) Refresh(ctx context.Context, steam64 string) error {
	if r == nil || r.Client == nil {
		return ErrNoAPIKey
	}
	return r.refresh(ctx, []string{steam64})
}

// Run sweeps stale players in batches until ctx is cancelled. BattlEye
// GUIDs are backfilled every pass whether or not an API key is set -- they
// need no Steam call, and players first created by the game server (the
// C++ extension) arrive without one.
func (r *Refresher) Run(ctx context.Context) {
	hasKey := r.Client != nil && r.Client.Key != ""
	if !hasKey {
		slog.Info("steam: STEAM_WEB_API_KEY not set, profile/ban refresh disabled (BattlEye GUIDs still backfilled)")
	}
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		r.backfillGUIDs(ctx)
		if hasKey {
			r.sweep(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Refresher) sweep(ctx context.Context) {
	for {
		rows, err := r.Pool.Query(ctx, `
			SELECT uid FROM players
			WHERE steam_refreshed_at IS NULL OR steam_refreshed_at < now() - make_interval(secs => $1)
			ORDER BY steam_refreshed_at NULLS FIRST
			LIMIT $2
		`, staleAfter.Seconds(), maxIDsPerCall)
		if err != nil {
			slog.Error("steam: sweep query failed", "error", err)
			return
		}
		var ids []string
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				ids = append(ids, id)
			}
		}
		rows.Close()
		if len(ids) == 0 || ctx.Err() != nil {
			return
		}
		if err := r.refresh(ctx, ids); err != nil {
			slog.Warn("steam: sweep batch failed, will retry next pass", "error", err)
			return
		}
		if len(ids) < maxIDsPerCall {
			return
		}
	}
}

// refresh fetches both endpoints for ids and writes the results. A player
// whose Steam ban counts went UP since the last refresh gets an
// anti_cheat_flags row, so a new VAC/game ban lands in the existing review
// queue instead of sitting silently in a column (INTEGRATIONS.md §2.1).
func (r *Refresher) refresh(ctx context.Context, ids []string) error {
	summaries, err := r.Client.Summaries(ctx, ids)
	if err != nil {
		return err
	}
	bans, err := r.Client.PlayerBans(ctx, ids)
	if err != nil {
		return err
	}

	for _, id := range ids {
		s, hasSummary := summaries[id]
		b, hasBans := bans[id]
		var createdAt *time.Time
		if hasSummary && !s.CreatedAt.IsZero() {
			createdAt = &s.CreatedAt
		}

		var playerID int64
		var prevVAC, prevGame *int
		err := r.Pool.QueryRow(ctx, `
			SELECT id, steam_vac_bans, steam_game_bans FROM players WHERE uid = $1
		`, id).Scan(&playerID, &prevVAC, &prevGame)
		if err != nil {
			continue
		}

		_, err = r.Pool.Exec(ctx, `
			UPDATE players SET
				steam_name             = CASE WHEN $2 THEN NULLIF($3, '') ELSE steam_name END,
				steam_avatar_url       = CASE WHEN $2 THEN NULLIF($4, '') ELSE steam_avatar_url END,
				steam_created_at       = CASE WHEN $2 THEN COALESCE($5, steam_created_at) ELSE steam_created_at END,
				steam_vac_bans         = CASE WHEN $6 THEN $7 ELSE steam_vac_bans END,
				steam_game_bans        = CASE WHEN $6 THEN $8 ELSE steam_game_bans END,
				steam_days_since_ban   = CASE WHEN $6 THEN $9 ELSE steam_days_since_ban END,
				steam_community_banned = CASE WHEN $6 THEN $10 ELSE steam_community_banned END,
				steam_refreshed_at     = now()
			WHERE id = $1
		`, playerID, hasSummary, s.Name, s.AvatarURL, createdAt,
			hasBans, b.VACBans, b.GameBans, b.DaysSinceLastBan, b.CommunityBanned)
		if err != nil {
			slog.Error("steam: updating player failed", "player_id", playerID, "error", err)
			continue
		}

		// Only flag an increase against a previously-known count -- the
		// very first refresh just records the baseline, since an old ban
		// from years ago isn't news the review queue needs.
		if hasBans && prevVAC != nil && prevGame != nil && (b.VACBans > *prevVAC || b.GameBans > *prevGame) {
			details, _ := json.Marshal(map[string]int{
				"vac_bans_before": *prevVAC, "vac_bans_now": b.VACBans,
				"game_bans_before": *prevGame, "game_bans_now": b.GameBans,
				"days_since_last_ban": b.DaysSinceLastBan,
			})
			if _, err := r.Pool.Exec(ctx, `
				INSERT INTO anti_cheat_flags (player_id, flag_type, confidence, details)
				VALUES ($1, 'steam_ban', 'high', $2)
			`, playerID, details); err != nil {
				slog.Error("steam: recording new-ban flag failed", "player_id", playerID, "error", err)
			}
		}
	}
	return nil
}

// backfillGUIDs sets players.be_guid for any row that doesn't have one yet.
func (r *Refresher) backfillGUIDs(ctx context.Context) {
	rows, err := r.Pool.Query(ctx, `SELECT id, uid FROM players WHERE be_guid IS NULL LIMIT 1000`)
	if err != nil {
		slog.Error("steam: GUID backfill query failed", "error", err)
		return
	}
	type row struct {
		id  int64
		uid string
	}
	var todo []row
	for rows.Next() {
		var x row
		if rows.Scan(&x.id, &x.uid) == nil {
			todo = append(todo, x)
		}
	}
	rows.Close()
	for _, x := range todo {
		guid, err := BEGUID(x.uid)
		if err != nil {
			continue // not a numeric Steam64 -- leave NULL rather than store garbage
		}
		if _, err := r.Pool.Exec(ctx, `UPDATE players SET be_guid = $2 WHERE id = $1 AND be_guid IS NULL`, x.id, guid); err != nil {
			slog.Error("steam: GUID backfill update failed", "player_id", x.id, "error", err)
		}
	}
}