// Package playerlookup resolves a player by exact (case-insensitive) name
// -- what a player can actually see and ask another player for, unlike a
// numeric ID. Shared by anything that needs "find the player this other
// player is talking about" (bank transfers, gang invites): ambiguous or
// unknown names are rejected rather than guessed at, since routing a real
// action (money, a gang invite) to the wrong account on a bad guess is
// exactly the class of mistake worth an extra rejection to prevent.
package playerlookup

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound  = errors.New("playerlookup: no player found with that name")
	ErrAmbiguous = errors.New("playerlookup: more than one player has that name -- ask them for their exact name")
)

func ByExactName(ctx context.Context, pool *pgxpool.Pool, name string) (playerID int64, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, ErrNotFound
	}

	rows, err := pool.Query(ctx, `SELECT id FROM players WHERE lower(name) = lower($1)`, name)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	found := 0
	for rows.Next() {
		found++
		if err := rows.Scan(&playerID); err != nil {
			return 0, err
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	switch {
	case found == 0:
		return 0, ErrNotFound
	case found > 1:
		return 0, ErrAmbiguous
	default:
		return playerID, nil
	}
}
