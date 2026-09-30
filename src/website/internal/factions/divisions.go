package factions

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Qual is one qualification.
type Qual struct {
	Key, Name, Body string
	HeadID          int64
	Head            string
	Holders         int
}

// Quals returns a faction's qualifications in display order.
func Quals(ctx context.Context, q dbtx, faction string) ([]Qual, error) {
	rows, err := q.Query(ctx, `
		SELECT fq.key, fq.name, fq.body, COALESCE(fq.head_id, 0),
		       COALESCE(NULLIF(h.name, ''), NULLIF(h.steam_name, ''), ''),
		       (SELECT count(*) FROM faction_member_quals mq JOIN players p ON p.id = mq.player_id
		         WHERE mq.faction = fq.faction AND mq.qual_key = fq.key AND p.`+column[faction]+` > 0)
		FROM faction_quals fq LEFT JOIN players h ON h.id = fq.head_id
		WHERE fq.faction = $1 ORDER BY fq.sort, fq.key`, faction)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Qual
	for rows.Next() {
		var x Qual
		if err := rows.Scan(&x.Key, &x.Name, &x.Body, &x.HeadID, &x.Head, &x.Holders); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func memberQuals(ctx context.Context, q dbtx, faction string, playerID int64) (map[string]bool, error) {
	rows, err := q.Query(ctx, `SELECT qual_key FROM faction_member_quals WHERE faction = $1 AND player_id = $2`, faction, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out[k] = true
	}
	return out, rows.Err()
}

// MemberQuals is who holds what: player ID → qual key → held.
func MemberQuals(ctx context.Context, q dbtx, faction string) (map[int64]map[string]bool, error) {
	rows, err := q.Query(ctx, `SELECT player_id, qual_key FROM faction_member_quals WHERE faction = $1`, faction)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]map[string]bool{}
	for rows.Next() {
		var id int64
		var k string
		if err := rows.Scan(&id, &k); err != nil {
			return nil, err
		}
		if out[id] == nil {
			out[id] = map[string]bool{}
		}
		out[id][k] = true
	}
	return out, rows.Err()
}

// HeldQual is one qualification a member holds.
type HeldQual struct {
	Key, Name string
	By        string
	At        time.Time
}

// QualsOf lists a member's qualifications.
func QualsOf(ctx context.Context, q dbtx, faction string, playerID int64) ([]HeldQual, error) {
	rows, err := q.Query(ctx, `
		SELECT fq.key, fq.name, COALESCE(NULLIF(b.name, ''), NULLIF(b.steam_name, ''), ''), mq.granted_at
		FROM faction_member_quals mq JOIN faction_quals fq ON fq.faction = mq.faction AND fq.key = mq.qual_key
		LEFT JOIN players b ON b.id = mq.granted_by
		WHERE mq.faction = $1 AND mq.player_id = $2 ORDER BY fq.sort`, faction, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HeldQual
	for rows.Next() {
		var h HeldQual
		if err := rows.Scan(&h.Key, &h.Name, &h.By, &h.At); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// Division is one specialist division.
type Division struct {
	Key, Name, Color string
	Roles            []string // most senior first
	RequiredQual     string
	MinLevel         int
}

// EntryRole is the role new members join as.
func (d Division) EntryRole() string { return d.Roles[len(d.Roles)-1] }

func Divisions(ctx context.Context, q dbtx, faction string) ([]Division, error) {
	rows, err := q.Query(ctx, `
		SELECT key, name, color, roles, COALESCE(required_qual, ''), min_level
		FROM faction_divisions WHERE faction = $1 ORDER BY sort, key`, faction)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Division
	for rows.Next() {
		var d Division
		if err := rows.Scan(&d.Key, &d.Name, &d.Color, &d.Roles, &d.RequiredQual, &d.MinLevel); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Posting is a member's specialist division and role.
type Posting struct {
	Key, Role string
	Since     time.Time
}

// MemberDivisions maps player ID → their division posting.
func MemberDivisions(ctx context.Context, q dbtx, faction string) (map[int64]Posting, error) {
	rows, err := q.Query(ctx, `SELECT player_id, division_key, role, since FROM faction_member_divisions WHERE faction = $1`, faction)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]Posting{}
	for rows.Next() {
		var id int64
		var p Posting
		if err := rows.Scan(&id, &p.Key, &p.Role, &p.Since); err != nil {
			return nil, err
		}
		out[id] = p
	}
	return out, rows.Err()
}

// SetDivision posts a member to a division in role, or takes them out of
// their division (divKey ""). Command, for members ranked below them.
func SetDivision(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, targetID int64, divKey, role, reason string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	a, err := commandOver(ctx, tx, actor, faction, targetID)
	if err != nil {
		return err
	}
	if a.TargetLvl == 0 {
		return notAllowed("they aren't in %s", Name(faction))
	}
	if reason, err = cleanText(reason, 300, "the reason", false); err != nil {
		return err
	}
	name := playerName(ctx, tx, targetID)
	var cur Posting
	err = tx.QueryRow(ctx, `SELECT division_key, role FROM faction_member_divisions WHERE faction = $1 AND player_id = $2`, faction, targetID).Scan(&cur.Key, &cur.Role)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	divs, err := Divisions(ctx, tx, faction)
	if err != nil {
		return err
	}
	divName := func(k string) string {
		for _, d := range divs {
			if d.Key == k {
				return d.Name
			}
		}
		return k
	}
	var detail string
	if divKey == "" {
		if cur.Key == "" {
			return notAllowed("they aren't in a division")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM faction_member_divisions WHERE faction = $1 AND player_id = $2`, faction, targetID); err != nil {
			return err
		}
		detail = name + " left " + divName(cur.Key)
	} else {
		var d Division
		for _, x := range divs {
			if x.Key == divKey {
				d = x
			}
		}
		if d.Key == "" {
			return notAllowed("unknown division")
		}
		if role == "" {
			role = d.EntryRole()
		}
		if !slices.Contains(d.Roles, role) {
			return notAllowed("%s has no %q role", d.Name, role)
		}
		if cur.Key == divKey && cur.Role == role {
			return notAllowed("they're already %s in %s", role, d.Name)
		}
		if a.TargetLvl < d.MinLevel {
			return notAllowed("%s needs %s or above", d.Name, RankFor(a.Ranks, d.MinLevel).Label())
		}
		if d.RequiredQual != "" {
			held, err := memberQuals(ctx, tx, faction, targetID)
			if err != nil {
				return err
			}
			if !held[d.RequiredQual] {
				return notAllowed("%s needs the %s qualification", d.Name, d.RequiredQual)
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO faction_member_divisions (faction, player_id, division_key, role) VALUES ($1, $2, $3, $4)
			ON CONFLICT (faction, player_id) DO UPDATE SET division_key = $3, role = $4,
			    since = CASE WHEN faction_member_divisions.division_key = $3 THEN faction_member_divisions.since ELSE now() END`,
			faction, targetID, divKey, role); err != nil {
			return err
		}
		switch {
		case cur.Key == divKey:
			detail = name + ": " + d.Name + " role " + cur.Role + " → " + role
		case cur.Key != "":
			detail = name + " moved from " + divName(cur.Key) + " to " + d.Name + " as " + role
		default:
			detail = name + " joined " + d.Name + " as " + role
		}
	}
	if err := logEvent(ctx, tx, faction, actor, targetID, a.TargetLvl, "division", detail, reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetQual grants or removes a qualification. Command, for members ranked
// below them.
func SetQual(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, targetID int64, key string, grant bool, reason string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	a, err := commandOver(ctx, tx, actor, faction, targetID)
	if err != nil {
		return err
	}
	if a.TargetLvl == 0 {
		return notAllowed("they aren't in %s", Name(faction))
	}
	if reason, err = cleanText(reason, 300, "the note", false); err != nil {
		return err
	}
	var qname string
	if err := tx.QueryRow(ctx, `SELECT name FROM faction_quals WHERE faction = $1 AND key = $2`, faction, key).Scan(&qname); errors.Is(err, pgx.ErrNoRows) {
		return notAllowed("unknown qualification")
	} else if err != nil {
		return err
	}
	name := playerName(ctx, tx, targetID)
	var detail string
	if grant {
		tag, err := tx.Exec(ctx, `INSERT INTO faction_member_quals (faction, player_id, qual_key, granted_by) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
			faction, targetID, key, actor.PlayerID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return notAllowed("they already hold %s", qname)
		}
		detail = name + " passed " + qname + " (" + key + ")"
	} else {
		tag, err := tx.Exec(ctx, `DELETE FROM faction_member_quals WHERE faction = $1 AND player_id = $2 AND qual_key = $3`, faction, targetID, key)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return notAllowed("they don't hold %s", qname)
		}
		if _, err := tx.Exec(ctx, `UPDATE faction_quals SET head_id = NULL WHERE faction = $1 AND key = $2 AND head_id = $3`, faction, key, targetID); err != nil {
			return err
		}
		detail = name + " no longer holds " + qname + " (" + key + ")"
	}
	if err := logEvent(ctx, tx, faction, actor, targetID, a.TargetLvl, "qual", detail, reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetQualHead makes a holder the qualification's head trainer (0 = none).
func SetQualHead(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction, key string, headID int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	a, err := commandOver(ctx, tx, actor, faction, 0)
	if err != nil {
		return err
	}
	var qname string
	if err := tx.QueryRow(ctx, `SELECT name FROM faction_quals WHERE faction = $1 AND key = $2`, faction, key).Scan(&qname); errors.Is(err, pgx.ErrNoRows) {
		return notAllowed("unknown qualification")
	} else if err != nil {
		return err
	}
	detail := qname + ": no head trainer"
	if headID > 0 {
		held, err := memberQuals(ctx, tx, faction, headID)
		if err != nil {
			return err
		}
		if !held[key] {
			return notAllowed("the head trainer must hold %s", qname)
		}
		detail = qname + ": head trainer is now " + playerName(ctx, tx, headID)
	}
	if _, err := tx.Exec(ctx, `UPDATE faction_quals SET head_id = NULLIF($3, 0) WHERE faction = $1 AND key = $2`, faction, key, headID); err != nil {
		return err
	}
	if err := logEvent(ctx, tx, faction, actor, headID, a.Level, "qual", detail, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
