// Package gang implements the member portal's gang-management writes --
// invite, remove, change rank -- against the existing gang_members/gang_log
// tables, gated to whoever gangs.leader_player_id names for that gang. No
// separate "officer" permission tier exists in the schema (gang_members.rank
// is free text with no CHECK constraint), so only the leader can manage
// membership here -- a rank value like "officer" is a label a leader can
// set for their own roster's organization, not a grant of these actions.
package gang

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/playerlookup"
)

var (
	ErrNotLeader        = errors.New("gang: you're not the leader of a gang")
	ErrMemberNotFound   = errors.New("gang: no player found with that name")
	ErrMemberAmbiguous  = errors.New("gang: more than one player has that name -- ask them for their exact name")
	ErrAlreadyInAGang   = errors.New("gang: that player is already in a gang")
	ErrCannotManageSelf = errors.New("gang: you can't remove or re-rank yourself as leader")
	ErrMemberNotInGang  = errors.New("gang: that player isn't a member of your gang")
	ErrInvalidRank      = errors.New("gang: rank must be \"member\" or \"officer\"")
)

func validRank(r string) bool { return r == "member" || r == "officer" }

// leaderGangID returns the gang a player leads, or ErrNotLeader if they
// don't lead one.
func leaderGangID(ctx context.Context, pool *pgxpool.Pool, playerID int64) (int64, error) {
	var gangID int64
	err := pool.QueryRow(ctx, `SELECT id FROM gangs WHERE leader_player_id = $1`, playerID).Scan(&gangID)
	if err == pgx.ErrNoRows {
		return 0, ErrNotLeader
	}
	return gangID, err
}

// Invite adds memberName (by exact, case-insensitive name) to the leader's
// gang as a 'member'. gang_members.player_id is UNIQUE (a player belongs to
// at most one gang at a time, per schema.sql) -- caught here and turned
// into ErrAlreadyInAGang instead of a raw constraint-violation error.
func Invite(ctx context.Context, pool *pgxpool.Pool, leaderPlayerID int64, memberName string) error {
	gangID, err := leaderGangID(ctx, pool, leaderPlayerID)
	if err != nil {
		return err
	}

	memberID, err := playerlookup.ByExactName(ctx, pool, memberName)
	switch {
	case err == playerlookup.ErrNotFound:
		return ErrMemberNotFound
	case err == playerlookup.ErrAmbiguous:
		return ErrMemberAmbiguous
	case err != nil:
		return err
	}
	if memberID == leaderPlayerID {
		return ErrCannotManageSelf
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `INSERT INTO gang_members (gang_id, player_id, rank) VALUES ($1, $2, 'member')`, gangID, memberID)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrAlreadyInAGang
		}
		return fmt.Errorf("gang: inviting member: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO gang_log (gang_id, actor_player_id, action, details)
		VALUES ($1, $2, 'member_added', jsonb_build_object('player_id', $3::bigint, 'source', 'website'))
	`, gangID, leaderPlayerID, memberID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Remove drops a member from the leader's gang. The leader can't remove
// themselves this way -- leaving/disbanding a gang is a different action,
// not built here.
func Remove(ctx context.Context, pool *pgxpool.Pool, leaderPlayerID, memberPlayerID int64) error {
	gangID, err := leaderGangID(ctx, pool, leaderPlayerID)
	if err != nil {
		return err
	}
	if memberPlayerID == leaderPlayerID {
		return ErrCannotManageSelf
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `DELETE FROM gang_members WHERE gang_id = $1 AND player_id = $2`, gangID, memberPlayerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrMemberNotInGang
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO gang_log (gang_id, actor_player_id, action, details)
		VALUES ($1, $2, 'member_removed', jsonb_build_object('player_id', $3::bigint, 'source', 'website'))
	`, gangID, leaderPlayerID, memberPlayerID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// SetRank updates a member's rank label ("member"/"officer" -- see the
// package doc for why this is organizational only, not a permission grant).
func SetRank(ctx context.Context, pool *pgxpool.Pool, leaderPlayerID, memberPlayerID int64, newRank string) error {
	if !validRank(newRank) {
		return ErrInvalidRank
	}
	gangID, err := leaderGangID(ctx, pool, leaderPlayerID)
	if err != nil {
		return err
	}
	if memberPlayerID == leaderPlayerID {
		return ErrCannotManageSelf
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `UPDATE gang_members SET rank = $1 WHERE gang_id = $2 AND player_id = $3`, newRank, gangID, memberPlayerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrMemberNotInGang
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO gang_log (gang_id, actor_player_id, action, details)
		VALUES ($1, $2, 'rank_changed', jsonb_build_object('player_id', $3::bigint, 'new_rank', $4::text, 'source', 'website'))
	`, gangID, leaderPlayerID, memberPlayerID, newRank)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
