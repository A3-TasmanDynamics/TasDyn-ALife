// Package roles edits staff ranks and their permissions (the layout plan's
// Roles & Permissions board, GAMEPANEL_PARITY §2.1) and the police/EMS
// rank display names (§6.3).
//
// Rules, enforced here so the website and any future bot command share
// them:
//   - you can only edit, move, create or delete ranks BELOW your own level,
//     except the top rank (level 100, Management), which can also edit
//     ranks at its own level -- there's nobody above it to do so -- but
//     never remove its own access to Roles (roles.manage) or the admin
//     panel;
//   - a rank's level can be set directly, to an unused level below yours;
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
	"website/internal/factions"
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

// TopLevel is the highest staff level (Management).
const TopLevel = 100

// CanEdit reports whether an actor at actorLevel may edit a rank at
// rankLevel: ranks below them, or, for the top level, ranks at it too.
func CanEdit(rankLevel, actorLevel int) bool {
	return rankLevel < actorLevel || (actorLevel >= TopLevel && rankLevel <= actorLevel)
}

// actorRank is the actor's own staff rank id (0 if not staff).
func actorRank(ctx context.Context, tx pgx.Tx, actorID int64) int {
	var id *int
	_ = tx.QueryRow(ctx, `SELECT staff_rank_id FROM players WHERE id = $1`, actorID).Scan(&id)
	if id == nil {
		return 0
	}
	return *id
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
// Update saves a rank's name, default panels, permissions and (when level
// is non-zero and different) its level.
func Update(ctx context.Context, pool *pgxpool.Pool, actorID int64, rankID int, name string, adminPanel, supportPanel bool, keys []string, level int) error {
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
	if !CanEdit(r.Level, actorLevel) {
		return notAllowed("%s is at or above your level, so it's read-only for you", r.Name)
	}
	if level == 0 {
		level = r.Level
	}
	if level != r.Level {
		if r.Level >= actorLevel {
			return notAllowed("%s's level can't change: it's at your own level", r.Name)
		}
		if level < 1 || level >= actorLevel {
			return notAllowed("the level must be between 1 and %d (below your own)", actorLevel-1)
		}
		var clash string
		if err := tx.QueryRow(ctx, `SELECT display_name FROM staff_ranks WHERE level = $1 AND id <> $2`, level, rankID).Scan(&clash); err == nil {
			return notAllowed("%s already uses level %d", clash, level)
		}
	}
	if actorRank(ctx, tx, actorID) == rankID {
		// Editing your own (top) rank: don't lock yourself out.
		if !adminPanel {
			return notAllowed("you can't take the admin panel away from your own rank")
		}
		if !want["roles.manage"] {
			return notAllowed("you can't remove roles.manage from your own rank, or you'd lose this page")
		}
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
	if len(added) == 0 && len(removed) == 0 && name == r.Name && adminPanel == r.AdminPanel && supportPanel == r.SupportPanel && level == r.Level {
		return notAllowed("nothing changed")
	}

	if _, err := tx.Exec(ctx, `UPDATE staff_ranks SET display_name = $2, default_admin_panel = $3, default_support_panel = $4, level = $5 WHERE id = $1`,
		rankID, name, adminPanel, supportPanel, level); err != nil {
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
	if level != r.Level {
		if reason != "" {
			reason += "; "
		}
		reason += fmt.Sprintf("level %d → %d", r.Level, level)
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actorID, Action: "rank_edit:" + r.Name, Reason: reason,
		Before: map[string]any{"name": r.Name, "level": r.Level, "admin_panel": r.AdminPanel, "support_panel": r.SupportPanel, "permissions": before},
		After:  map[string]any{"name": name, "level": level, "admin_panel": adminPanel, "support_panel": supportPanel, "permissions": sortedKeys(want)},
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
	if !CanEdit(r.Level, actorLevel) {
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

// FactionRanks returns each faction's configured ranks, lowest first.
func FactionRanks(ctx context.Context, pool *pgxpool.Pool) (map[string][]factions.Rank, error) {
	out := map[string][]factions.Rank{}
	for _, f := range factions.Factions {
		r, err := factions.Ranks(ctx, pool, f)
		if err != nil {
			return nil, err
		}
		out[f] = r
	}
	return out, nil
}

// SaveFactionNames replaces one faction's level names, keeping each
// level's other settings (short name, slots, command authority).
func SaveFactionNames(ctx context.Context, pool *pgxpool.Pool, actorID int64, faction string, names []string) error {
	cur, err := factions.Ranks(ctx, pool, faction)
	if err != nil {
		return err
	}
	ranks := make([]factions.Rank, len(names))
	for i, n := range names {
		ranks[i] = factions.RankFor(cur, i+1)
		ranks[i].Level, ranks[i].Name = i+1, n
	}
	return SaveFactionRanks(ctx, pool, actorID, faction, ranks)
}

// SaveFactionRanks replaces one faction's ranks (level 1 = first): name,
// short name, slots (0 = no limit) and command authority (the highest
// level this rank may set others to; 0 = not command). Blank trailing
// ranks are dropped; a blank in the middle is refused so levels never
// shift unexpectedly. Caller checks factions.configure.
func SaveFactionRanks(ctx context.Context, pool *pgxpool.Pool, actorID int64, faction string, ranks []factions.Rank) error {
	if !factions.Valid(faction) {
		return notAllowed("unknown faction")
	}
	for len(ranks) > 0 && strings.TrimSpace(ranks[len(ranks)-1].Name) == "" {
		ranks = ranks[:len(ranks)-1]
	}
	if len(ranks) > 30 {
		return notAllowed("at most 30 levels")
	}
	for i := range ranks {
		r := &ranks[i]
		r.Level = i + 1
		r.Name, r.Short = strings.TrimSpace(r.Name), strings.TrimSpace(r.Short)
		switch {
		case r.Name == "":
			return notAllowed("level %d has no name; remove levels from the end instead", r.Level)
		case len(r.Name) > 50:
			return notAllowed("level %d's name is too long (50 characters max)", r.Level)
		case len(r.Short) > 12:
			return notAllowed("level %d's short name is too long (12 characters max)", r.Level)
		case r.Slots < 0 || r.Slots > 500:
			return notAllowed("level %d's slots must be between 1 and 500, or blank for no limit", r.Level)
		case r.PromoteUpTo < 0 || r.PromoteUpTo >= r.Level:
			return notAllowed("%s can only be given authority over ranks below it", r.Name)
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	before, err := factions.Ranks(ctx, tx, faction)
	if err != nil {
		return err
	}
	// Only the fields edited here are compared and written; the Ranks &
	// gear rules (min days, quals, description, command/cabinet ticks) stay.
	edited := func(rs []factions.Rank) string {
		var b strings.Builder
		for _, r := range rs {
			fmt.Fprintf(&b, "%d|%s|%s|%d|%d;", r.Level, r.Name, r.Short, r.Slots, r.PromoteUpTo)
		}
		return b.String()
	}
	if edited(before) == edited(ranks) {
		return notAllowed("nothing changed")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM faction_rank_names WHERE faction = $1 AND level > $2`, faction, len(ranks)); err != nil {
		return err
	}
	for _, r := range ranks {
		var short *string
		var slots *int
		if r.Short != "" {
			short = &r.Short
		}
		if r.Slots > 0 {
			slots = &r.Slots
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO faction_rank_names (faction, level, name, short_name, slots, promote_up_to)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (faction, level) DO UPDATE SET name = $3, short_name = $4, slots = $5, promote_up_to = $6`,
			faction, r.Level, r.Name, short, slots, r.PromoteUpTo); err != nil {
			return err
		}
	}
	summary := func(rs []factions.Rank) []string {
		out := make([]string, len(rs))
		for i, r := range rs {
			out[i] = r.Name
			if r.PromoteUpTo > 0 {
				out[i] += fmt.Sprintf(" (promotes up to level %d)", r.PromoteUpTo)
			}
			if r.Slots > 0 {
				out[i] += fmt.Sprintf(" [%d slots]", r.Slots)
			}
		}
		return out
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actorID, Action: "faction_rank_names:" + faction,
		Before: map[string]any{"names": summary(before)}, After: map[string]any{"names": summary(ranks)},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
