package factions

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
)

// Adding and deleting ranks from Ranks & gear. Levels are positions
// (1 = lowest), so a rank added or deleted in the middle moves every rank
// above it, and with it everything that stores a level: members, division
// minimums, blacklist / discharge records, the Command log and the Discord /
// TeamSpeak role map -- everyone keeps the rank they had, it just has a new
// number. Only cabinet (ranks below their own) and Management (any) can.

// CanManageRanks says whether the viewer may add and delete ranks.
func (c Command) CanManageRanks() bool { return c.Cabinet }

// AddRank inserts a rank named name at level at (1 = the lowest; one past
// the top adds a new top rank).
func AddRank(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, at int, name, short string) error {
	var err error
	if name, err = cleanText(name, 60, "the rank name", true); err != nil {
		return err
	}
	if short, err = cleanText(short, 12, "the short name", false); err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	a, err := maintainOver(ctx, tx, actor, faction)
	if err != nil {
		return err
	}
	if !a.Management && !a.Cabinet {
		return notAllowed("only cabinet or Management can add ranks")
	}
	n := len(a.Ranks)
	switch {
	case n >= 30:
		return notAllowed("at most 30 ranks")
	case at < 1 || at > n+1:
		return notAllowed("pick where the new rank goes")
	case !a.Management && at > a.Level:
		return notAllowed("you can only add ranks below your own; Management can add higher")
	}
	for _, r := range a.Ranks {
		if r.Name == name {
			return notAllowed("there's already a rank called %s", name)
		}
	}
	moved := n - at + 1
	reason := fmt.Sprintf("Rank added: %s at level %d", name, at)
	if err := renumber(ctx, tx, actor, faction, at, 1, reason); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO faction_rank_names (faction, level, name, short_name) VALUES ($1, $2, $3, NULLIF($4, ''))`,
		faction, at, name, short); err != nil {
		return err
	}
	detail := fmt.Sprintf("Added rank %s at level %d", name, at)
	if moved > 0 {
		detail += fmt.Sprintf("; the %d rank(s) above it moved up one level", moved)
	}
	if err := logEvent(ctx, tx, faction, actor, 0, a.Level, "rank_rules", detail, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteRank removes the rank at level; nobody may hold it.
func DeleteRank(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, level int) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	a, err := maintainOver(ctx, tx, actor, faction)
	if err != nil {
		return err
	}
	if !a.Management && !a.Cabinet {
		return notAllowed("only cabinet or Management can delete ranks")
	}
	if !a.Management && level >= a.Level {
		return notAllowed("you can only delete ranks below your own; Management can delete any")
	}
	var old Rank
	for _, r := range a.Ranks {
		if r.Level == level {
			old = r
		}
	}
	if old.Level == 0 {
		return notAllowed("that rank doesn't exist")
	}
	var held int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM players WHERE `+column[faction]+` = $1`, level).Scan(&held); err != nil {
		return err
	}
	if held > 0 {
		return notAllowed("%d member(s) hold %s; move them to another rank first", held, old.Label())
	}
	if _, err := tx.Exec(ctx, `DELETE FROM faction_rank_names WHERE faction = $1 AND level = $2`, faction, level); err != nil {
		return err
	}
	// Its Discord / TeamSpeak roles go with it.
	if _, err := tx.Exec(ctx, `DELETE FROM platform_group_map WHERE entitlement = $1`, fmt.Sprintf("faction_rank:%s:%d", faction, level)); err != nil {
		return err
	}
	if err := renumber(ctx, tx, actor, faction, level, -1, "Rank deleted: "+old.Name); err != nil {
		return err
	}
	detail := "Deleted rank " + old.Label()
	if moved := len(a.Ranks) - level; moved > 0 {
		detail += fmt.Sprintf("; the %d rank(s) above it moved down one level", moved)
	}
	if err := logEvent(ctx, tx, faction, actor, 0, a.Level, "rank_rules", detail, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// renumber moves every stored level of faction at or above from up one
// (delta 1, making room at from), or above from down one (delta -1, closing
// the gap left by deleting from; anything that pointed at from itself
// drops to the rank below).
func renumber(ctx context.Context, tx pgx.Tx, actor Actor, faction string, from, delta int, reason string) error {
	col := column[faction]
	// Levels that move: >= from going up, > from going down.
	first := from
	if delta < 0 {
		first = from + 1
	}
	// The ranks themselves, through 1000+ so no two share a level on the way
	// (the primary key); "promotes up to" follows.
	type step struct {
		q    string
		args []any
	}
	parked := step{`UPDATE faction_rank_names SET level = level - 1000 WHERE faction = $1 AND level > 1000`, []any{faction}}
	steps := []step{
		{`UPDATE faction_rank_names SET level = level + 1001 WHERE faction = $1 AND level >= $2`, []any{faction, first}},
		parked,
		{`UPDATE faction_rank_names SET promote_up_to = promote_up_to + 1 WHERE faction = $1 AND promote_up_to >= $2`, []any{faction, first}},
	}
	if delta < 0 {
		steps = []step{
			// "Up to" the deleted rank or above drops one.
			{`UPDATE faction_rank_names SET promote_up_to = promote_up_to - 1 WHERE faction = $1 AND promote_up_to > 0 AND promote_up_to >= $2`, []any{faction, from}},
			{`UPDATE faction_rank_names SET level = level + 999 WHERE faction = $1 AND level >= $2`, []any{faction, first}},
			parked,
		}
	}
	for _, s := range steps {
		if _, err := tx.Exec(ctx, s.q, s.args...); err != nil {
			return err
		}
	}
	// Members: logged as rank changes by the players trigger, with the reason.
	if err := audit.SetActor(ctx, tx, actor.PlayerID, actor.Source, reason); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE players SET `+col+` = `+col+` + $2 WHERE `+col+` >= $1`, first, delta); err != nil {
		return err
	}
	// Records that keep a level: anything at the deleted level goes to the rank below.
	low := first
	if delta < 0 {
		low = from
	}
	for _, q := range []string{
		`UPDATE faction_divisions SET min_level = GREATEST(min_level + $3, 0) WHERE faction = $1 AND min_level > 0 AND min_level >= $2`,
		`UPDATE faction_blacklist SET last_level = GREATEST(last_level + $3, 0) WHERE faction = $1 AND last_level > 0 AND last_level >= $2`,
		`UPDATE faction_discharges SET last_level = GREATEST(last_level + $3, 0) WHERE faction = $1 AND last_level > 0 AND last_level >= $2`,
		`UPDATE faction_log SET from_level = GREATEST(from_level + $3, 0) WHERE faction = $1 AND from_level > 0 AND from_level >= $2`,
		`UPDATE faction_log SET to_level = GREATEST(to_level + $3, 0) WHERE faction = $1 AND to_level > 0 AND to_level >= $2`,
	} {
		if _, err := tx.Exec(ctx, q, faction, low, delta); err != nil {
			return err
		}
	}
	// Discord / TeamSpeak roles follow their rank to its new level (via a
	// temporary name, so no two mappings collide on the way).
	prefix := "faction_rank:" + faction + ":"
	if _, err := tx.Exec(ctx, `
		UPDATE platform_group_map SET entitlement = 'renumber:' || $1 || (split_part(entitlement, ':', 3)::int + $3)
		WHERE entitlement LIKE $1 || '%' AND split_part(entitlement, ':', 3) ~ '^[0-9]+$' AND split_part(entitlement, ':', 3)::int >= $2`,
		prefix, first, delta); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE platform_group_map SET entitlement = substr(entitlement, 10) WHERE entitlement LIKE 'renumber:%'`)
	return err
}
