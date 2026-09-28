// Package roles edits staff ranks and their permissions (the layout plan's
// Roles & Permissions board, GAMEPANEL_PARITY §2.1) and the police/EMS
// rank display names (§6.3).
//
// Rules, enforced here so the website and any future bot command share
// them:
//   - you can only edit, move, create or delete ranks BELOW your own level;
//   - you can only grant a permission you currently hold yourself
//     (removing any permission from a rank you may edit is fine);
//   - only catalogue keys can be granted (auth.Known);
//   - the last level-100 rank can't be removed, and a rank with members
//     can't be deleted.
//
// Every change writes a staff_log row with before/after values.
package roles

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
	"website/internal/auth"
)

var ErrNotAllowed = errors.New("not allowed")

func notAllowed(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrNotAllowed, fmt.Sprintf(format, args...))
}

type Rank struct {
	ID           int
	Key          string
	Name         string
	Level        int
	AdminPanel   bool
	SupportPanel bool
	Members      int
	Perms        map[string]bool
}

// List returns every rank, highest level first, with members and perms.
func List(ctx context.Context, pool *pgxpool.Pool) ([]Rank, error) {
	rows, err := pool.Query(ctx, `
		SELECT sr.id, sr.key, sr.display_name, sr.level, sr.default_admin_panel, sr.default_support_panel,
		       (SELECT count(*) FROM players p WHERE p.staff_rank_id = sr.id),
		       COALESCE((SELECT array_agg(command_key) FROM rank_permissions rp WHERE rp.rank_id = sr.id), '{}')
		FROM staff_ranks sr ORDER BY sr.level DESC, sr.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Rank
	for rows.Next() {
		var r Rank
		var keys []string
		if err := rows.Scan(&r.ID, &r.Key, &r.Name, &r.Level, &r.AdminPanel, &r.SupportPanel, &r.Members, &keys); err != nil {
			return nil, err
		}
		r.Perms = map[string]bool{}
		for _, k := range keys {
			r.Perms[k] = true
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ActorLevel is the level of the actor's own rank (0 if not staff).
func ActorLevel(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, actorID int64) (int, error) {
	var level int
	err := q.QueryRow(ctx, `
		SELECT COALESCE(sr.level, 0) FROM players p LEFT JOIN staff_ranks sr ON sr.id = p.staff_rank_id
		WHERE p.id = $1`, actorID).Scan(&level)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return level, err
}

func lockRank(ctx context.Context, tx pgx.Tx, rankID int) (Rank, error) {
	var r Rank
	err := tx.QueryRow(ctx, `
		SELECT id, key, display_name, level, default_admin_panel, default_support_panel
		FROM staff_ranks WHERE id = $1 FOR UPDATE`, rankID).
		Scan(&r.ID, &r.Key, &r.Name, &r.Level, &r.AdminPanel, &r.SupportPanel)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, notAllowed("that rank doesn't exist")
	}
	return r, err
}

func rankPerms(ctx context.Context, tx pgx.Tx, rankID int) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT command_key FROM rank_permissions WHERE rank_id = $1 ORDER BY 1`, rankID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// Update saves a rank's name, default panels and permission set.
func Update(ctx context.Context, pool *pgxpool.Pool, actorID int64, rankID int, name string, adminPanel, supportPanel bool, keys []string) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 50 {
		return notAllowed("the display name must be 1-50 characters")
	}
	want := map[string]bool{}
	for _, k := range keys {
		if !auth.Known(k) {
			return notAllowed("unknown permission %q", k)
		}
		want[k] = true
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	actorLevel, err := ActorLevel(ctx, tx, actorID)
	if err != nil {
		return err
	}
	r, err := lockRank(ctx, tx, rankID)
	if err != nil {
		return err
	}
	if r.Level >= actorLevel {
		return notAllowed("%s is at or above your level, so it's read-only for you", r.Name)
	}
	before, err := rankPerms(ctx, tx, rankID)
	if err != nil {
		return err
	}
	had := map[string]bool{}
	for _, k := range before {
		had[k] = true
	}
	var added, removed []string
	for k := range want {
		if !had[k] {
			added = append(added, k)
		}
	}
	for k := range had {
		if !want[k] {
			removed = append(removed, k)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	for _, k := range added {
		denial, err := auth.Can(ctx, pool, actorID, k)
		if err != nil {
			return err
		}
		if denial != auth.Allowed {
			return notAllowed("you can't grant %s because you don't have it yourself", k)
		}
	}
	if len(added) == 0 && len(removed) == 0 && name == r.Name && adminPanel == r.AdminPanel && supportPanel == r.SupportPanel {
		return notAllowed("nothing changed")
	}

	if _, err := tx.Exec(ctx, `UPDATE staff_ranks SET display_name = $2, default_admin_panel = $3, default_support_panel = $4 WHERE id = $1`,
		rankID, name, adminPanel, supportPanel); err != nil {
		return err
	}
	if len(removed) > 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM rank_permissions WHERE rank_id = $1 AND command_key = ANY($2)`, rankID, removed); err != nil {
			return err
		}
	}
	for _, k := range added {
		if _, err := tx.Exec(ctx, `INSERT INTO rank_permissions (rank_id, command_key) VALUES ($1, $2)`, rankID, k); err != nil {
			return err
		}
	}

	reason := describeChanges(r, name, adminPanel, supportPanel, added, removed)
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actorID, Action: "rank_edit:" + r.Name, Reason: reason,
		Before: map[string]any{"name": r.Name, "admin_panel": r.AdminPanel, "support_panel": r.SupportPanel, "permissions": before},
		After:  map[string]any{"name": name, "admin_panel": adminPanel, "support_panel": supportPanel, "permissions": sortedKeys(want)},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func describeChanges(r Rank, name string, admin, support bool, added, removed []string) string {
	var parts []string
	if name != r.Name {
		parts = append(parts, "renamed to "+name)
	}
	if admin != r.AdminPanel {
		parts = append(parts, fmt.Sprintf("admin panel %v", onOff(admin)))
	}
	if support != r.SupportPanel {
		parts = append(parts, fmt.Sprintf("support panel %v", onOff(support)))
	}
	if len(added) > 0 {
		parts = append(parts, "+"+strings.Join(added, " +"))
	}
	if len(removed) > 0 {
		parts = append(parts, "-"+strings.Join(removed, " -"))
	}
	return strings.Join(parts, "; ")
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// Move swaps a rank's level with its neighbour above (up) or below.
func Move(ctx context.Context, pool *pgxpool.Pool, actorID int64, rankID int, up bool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	actorLevel, err := ActorLevel(ctx, tx, actorID)
	if err != nil {
		return err
	}
	r, err := lockRank(ctx, tx, rankID)
	if err != nil {
		return err
	}
	var other Rank
	q := `SELECT id, display_name, level FROM staff_ranks WHERE level < $1 ORDER BY level DESC LIMIT 1 FOR UPDATE`
	if up {
		q = `SELECT id, display_name, level FROM staff_ranks WHERE level > $1 ORDER BY level ASC LIMIT 1 FOR UPDATE`
	}
	if err := tx.QueryRow(ctx, q, r.Level).Scan(&other.ID, &other.Name, &other.Level); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notAllowed("%s is already at the %s", r.Name, map[bool]string{true: "top", false: "bottom"}[up])
		}
		return err
	}
	if r.Level >= actorLevel || other.Level >= actorLevel {
		return notAllowed("you can only reorder ranks below your own level")
	}
	if _, err := tx.Exec(ctx, `UPDATE staff_ranks SET level = CASE id WHEN $1 THEN $3::integer WHEN $2 THEN $4::integer END WHERE id IN ($1, $2)`,
		r.ID, other.ID, other.Level, r.Level); err != nil {
		return err
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actorID, Action: "rank_reorder", Reason: fmt.Sprintf("%s moved %s %s", r.Name, map[bool]string{true: "above", false: "below"}[up], other.Name),
		Before: map[string]int{r.Name: r.Level, other.Name: other.Level},
		After:  map[string]int{r.Name: other.Level, other.Name: r.Level},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Create adds a rank with no permissions at the given level.
func Create(ctx context.Context, pool *pgxpool.Pool, actorID int64, name string, level int) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 50 {
		return 0, notAllowed("the display name must be 1-50 characters")
	}
	key := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "_"), "_")
	if key == "" {
		return 0, notAllowed("the name needs at least one letter or number")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	actorLevel, err := ActorLevel(ctx, tx, actorID)
	if err != nil {
		return 0, err
	}
	if level < 1 || level >= actorLevel {
		return 0, notAllowed("the level must be between 1 and %d (below your own)", actorLevel-1)
	}
	var clash string
	err = tx.QueryRow(ctx, `SELECT display_name FROM staff_ranks WHERE level = $1 OR key = $2 LIMIT 1`, level, key).Scan(&clash)
	if err == nil {
		return 0, notAllowed("%s already uses that level or name", clash)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	var id int
	if err := tx.QueryRow(ctx, `INSERT INTO staff_ranks (key, display_name, level) VALUES ($1, $2, $3) RETURNING id`,
		key, name, level).Scan(&id); err != nil {
		return 0, err
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actorID, Action: "rank_create:" + name, After: map[string]any{"name": name, "level": level},
	}); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

// Delete removes an empty rank below the actor's level.
func Delete(ctx context.Context, pool *pgxpool.Pool, actorID int64, rankID int) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	actorLevel, err := ActorLevel(ctx, tx, actorID)
	if err != nil {
		return err
	}
	r, err := lockRank(ctx, tx, rankID)
	if err != nil {
		return err
	}
	if r.Level >= actorLevel {
		return notAllowed("%s is at or above your level", r.Name)
	}
	var members, top int
	if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM players WHERE staff_rank_id = $1), (SELECT count(*) FROM staff_ranks WHERE level >= 100)`,
		rankID).Scan(&members, &top); err != nil {
		return err
	}
	if members > 0 {
		return notAllowed("%s still has %d member(s); move them to another rank first", r.Name, members)
	}
	if r.Level >= 100 && top <= 1 {
		return notAllowed("the last level-100 rank can't be removed")
	}
	perms, err := rankPerms(ctx, tx, rankID)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM staff_ranks WHERE id = $1`, rankID); err != nil {
		return err
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actorID, Action: "rank_delete:" + r.Name,
		Before: map[string]any{"name": r.Name, "level": r.Level, "permissions": perms},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FactionNames returns police and EMS level names, ordered by level.
func FactionNames(ctx context.Context, pool *pgxpool.Pool) (map[string][]string, error) {
	out := map[string][]string{"police": nil, "ems": nil}
	rows, err := pool.Query(ctx, `SELECT faction, name FROM faction_rank_names ORDER BY faction, level`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var f, n string
		if err := rows.Scan(&f, &n); err != nil {
			return nil, err
		}
		out[f] = append(out[f], n)
	}
	return out, rows.Err()
}

// SaveFactionNames replaces one faction's level names (level 1 = first).
// Blank trailing entries are dropped; a blank in the middle is refused so
// levels never shift unexpectedly. Caller checks factions.configure.
func SaveFactionNames(ctx context.Context, pool *pgxpool.Pool, actorID int64, faction string, names []string) error {
	if faction != "police" && faction != "ems" {
		return notAllowed("unknown faction")
	}
	for len(names) > 0 && strings.TrimSpace(names[len(names)-1]) == "" {
		names = names[:len(names)-1]
	}
	for i := range names {
		names[i] = strings.TrimSpace(names[i])
		if names[i] == "" {
			return notAllowed("level %d has no name; remove levels from the end instead", i+1)
		}
		if len(names[i]) > 50 {
			return notAllowed("level %d's name is too long (50 characters max)", i+1)
		}
	}
	if len(names) > 30 {
		return notAllowed("at most 30 levels")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var before []string
	rows, err := tx.Query(ctx, `SELECT name FROM faction_rank_names WHERE faction = $1 ORDER BY level`, faction)
	if err != nil {
		return err
	}
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil {
			before = append(before, n)
		}
	}
	rows.Close()
	if strings.Join(before, "\x00") == strings.Join(names, "\x00") {
		return notAllowed("nothing changed")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM faction_rank_names WHERE faction = $1`, faction); err != nil {
		return err
	}
	for i, n := range names {
		if _, err := tx.Exec(ctx, `INSERT INTO faction_rank_names (faction, level, name) VALUES ($1, $2, $3)`, faction, i+1, n); err != nil {
			return err
		}
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actorID, Action: "faction_rank_names:" + faction,
		Before: map[string]any{"names": before}, After: map[string]any{"names": names},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
