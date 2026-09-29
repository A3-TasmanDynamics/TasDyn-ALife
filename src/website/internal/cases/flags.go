package cases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
)

// Flag is one anti-cheat flag (layout plan "Anti-Cheat Flags"). Flags never
// punish on their own: a staff member dismisses one, watches the player, or
// opens a case from it.
type Flag struct {
	ID         int64
	PlayerID   int64
	Player     string
	Type       string
	Title      string
	Confidence string // high / medium
	Details    []KV
	CreatedAt  time.Time
	Resolution string // "" = open; dismiss / watch / case / kick / ban
	Note       string
	ReviewedBy string
	ReviewedAt *time.Time
	CaseID     int64
}

// KV is one detail field, flattened for display.
type KV struct{ Key, Value string }

var flagTitles = map[string]string{
	"honeypot":           "Honeypot triggered",
	"movement":           "Impossible movement",
	"idempotency_reject": "Replayed transaction rejected",
	"rate_limit":         "Request rate limit exceeded",
	"steam_ban":          "New Steam ban on account",
}

// Open reports whether nobody has reviewed the flag yet.
func (f Flag) Open() bool { return f.Resolution == "" }

// Status is the flag's review state for display.
func (f Flag) Status() string {
	switch f.Resolution {
	case "":
		return "Open"
	case "dismiss":
		return "Dismissed"
	case "watch":
		return "Watching"
	case "case":
		return "In a case"
	}
	return strings.ToUpper(f.Resolution[:1]) + f.Resolution[1:]
}

// FlagFilter narrows the flag list.
type FlagFilter struct {
	Confidence string // "" / high / medium
	OpenOnly   bool
}

const flagSelect = `
	SELECT f.id, f.player_id, ` + `COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id)` + `, f.flag_type, f.confidence,
	       COALESCE(f.details::text, ''), f.created_at, COALESCE(f.resolution, ''), COALESCE(f.resolution_note, ''),
	       COALESCE(NULLIF(r.name, ''), NULLIF(r.steam_name, ''), ''), f.reviewed_at,
	       COALESCE((SELECT id FROM staff_cases c WHERE c.flag_id = f.id ORDER BY id LIMIT 1), 0)
	FROM anti_cheat_flags f JOIN players p ON p.id = f.player_id LEFT JOIN players r ON r.id = f.reviewed_by`

func scanFlag(row pgx.Row) (Flag, error) {
	var f Flag
	var details string
	err := row.Scan(&f.ID, &f.PlayerID, &f.Player, &f.Type, &f.Confidence, &details, &f.CreatedAt, &f.Resolution,
		&f.Note, &f.ReviewedBy, &f.ReviewedAt, &f.CaseID)
	if err != nil {
		return f, err
	}
	f.Title = flagTitles[f.Type]
	if f.Title == "" {
		f.Title = f.Type
	}
	f.Details = flattenDetails(details)
	return f, nil
}

// flattenDetails turns the details JSON object into sorted key/value rows.
func flattenDetails(raw string) []KV {
	if raw == "" {
		return nil
	}
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return []KV{{"details", raw}}
	}
	out := make([]KV, 0, len(m))
	for k, v := range m {
		var s string
		switch x := v.(type) {
		case string:
			s = x
		default:
			b, _ := json.Marshal(x)
			s = string(b)
		}
		out = append(out, KV{strings.ReplaceAll(k, "_", " "), s})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// Flags lists flags, newest first.
func Flags(ctx context.Context, pool *pgxpool.Pool, f FlagFilter, limit int) ([]Flag, error) {
	rows, err := pool.Query(ctx, flagSelect+`
		WHERE ($1 = '' OR f.confidence = $1) AND (NOT $2 OR f.resolution IS NULL)
		ORDER BY f.id DESC LIMIT $3`, f.Confidence, f.OpenOnly, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Flag
	for rows.Next() {
		fl, err := scanFlag(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, fl)
	}
	return out, rows.Err()
}

// GetFlag loads one flag.
func GetFlag(ctx context.Context, pool *pgxpool.Pool, id int64) (Flag, error) {
	f, err := scanFlag(pool.QueryRow(ctx, flagSelect+` WHERE f.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return f, notAllowed("that flag doesn't exist")
	}
	return f, err
}

// ReviewFlag records a review: "dismiss" (false positive), "watch" (keep an
// eye on the player) or "" to reopen. Acting on the player happens through a
// case. The caller checks anticheat.review.
func ReviewFlag(ctx context.Context, pool *pgxpool.Pool, actor Actor, id int64, resolution, note string) error {
	note = strings.TrimSpace(note)
	switch resolution {
	case "dismiss", "watch":
		if note == "" {
			return notAllowed("add a note saying why")
		}
	case "":
	default:
		return notAllowed("unknown review outcome")
	}
	if len(note) > 1000 {
		return notAllowed("the note is too long (1000 characters max)")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var cur *string
	var playerID int64
	err = tx.QueryRow(ctx, `SELECT resolution, player_id FROM anti_cheat_flags WHERE id = $1 FOR UPDATE`, id).Scan(&cur, &playerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return notAllowed("that flag doesn't exist")
	}
	if err != nil {
		return err
	}
	if resolution == "" {
		if cur == nil {
			return notAllowed("the flag is already open")
		}
		if *cur == "case" {
			return notAllowed("this flag is handled in a case; work from the case instead")
		}
		_, err = tx.Exec(ctx, `UPDATE anti_cheat_flags SET resolution = NULL, reviewed_by = NULL, reviewed_at = NULL, resolution_note = NULL WHERE id = $1`, id)
	} else {
		if cur != nil && *cur == "case" {
			return notAllowed("this flag is handled in a case; work from the case instead")
		}
		_, err = tx.Exec(ctx, `UPDATE anti_cheat_flags SET resolution = $2, reviewed_by = $3, reviewed_at = now(), resolution_note = $4 WHERE id = $1`,
			id, resolution, actor.PlayerID, note)
	}
	if err != nil {
		return err
	}
	action := map[string]string{"dismiss": "anticheat.dismiss", "watch": "anticheat.watch", "": "anticheat.reopen"}[resolution]
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actor.PlayerID, TargetID: playerID, Action: action, Source: actor.Source,
		Reason: fmt.Sprintf("flag #%d: %s", id, note), After: map[string]any{"flag": id},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// OpenFlagCount is the sidebar badge.
func OpenFlagCount(ctx context.Context, pool *pgxpool.Pool) int {
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM anti_cheat_flags WHERE resolution IS NULL`).Scan(&n)
	return n
}
