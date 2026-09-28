package rolesync

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
)

// Option is one entitlement offered on the mapping page.
type Option struct {
	Key   string
	Label string
	Group string
}

// Options lists every entitlement that can currently occur, grouped for
// the mapping page. Gang roles are left out on purpose (INTEGRATIONS §7,
// decision 5: off by default).
func Options(ctx context.Context, pool *pgxpool.Pool) ([]Option, error) {
	out := []Option{
		{"linked", "Linked website account (e.g. Verified)", "Everyone"},
		{"staff", "Any active staff member", "Staff"},
		{"staff_loa", "Staff on leave (LOA)", "Staff"},
	}
	rows, err := pool.Query(ctx, `SELECT key, display_name FROM staff_ranks ORDER BY level DESC`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var k, n string
		if rows.Scan(&k, &n) == nil {
			out = append(out, Option{"staff_rank:" + k, "Rank: " + n, "Staff"})
		}
	}
	rows.Close()
	rows, err = pool.Query(ctx, `SELECT DISTINCT staff_team FROM players WHERE staff_team IS NOT NULL AND staff_team <> '' ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil {
			out = append(out, Option{"staff_team:" + Slug(t), "Team: " + t, "Staff"})
		}
	}
	rows.Close()

	out = append(out,
		Option{"faction:police", "Police (any level)", "Factions"},
		Option{"faction:ems", "EMS (any level)", "Factions"},
	)
	rows, err = pool.Query(ctx, `
		SELECT f, lvl, COALESCE((SELECT name FROM faction_rank_names n WHERE n.faction = f AND n.level = lvl), '')
		FROM (
			SELECT faction AS f, level AS lvl FROM faction_rank_names
			UNION SELECT 'police', cop_level FROM players WHERE cop_level > 0
			UNION SELECT 'ems', medic_level FROM players WHERE medic_level > 0
		) x ORDER BY f DESC, lvl`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var f, name string
		var lvl int
		if rows.Scan(&f, &lvl, &name) == nil {
			label := map[string]string{"police": "Police", "ems": "EMS"}[f] + " level " + strconv.Itoa(lvl)
			if name != "" {
				label += " · " + name
			}
			out = append(out, Option{"faction_rank:" + f + ":" + strconv.Itoa(lvl), label, "Factions"})
		}
	}
	rows.Close()
	return out, rows.Err()
}

// SaveMappings replaces a platform's mappings with m (entitlement -> group
// IDs), logging the change. Entitlements not in m lose their mappings.
func SaveMappings(ctx context.Context, pool *pgxpool.Pool, actorID int64, platform string, m map[string][]string) error {
	if platform != "discord" && platform != "teamspeak" {
		return fmt.Errorf("unknown platform %q", platform)
	}
	before, err := Mappings(ctx, pool, platform)
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM platform_group_map WHERE platform = $1`, platform); err != nil {
		return err
	}
	var after []string
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, ent := range keys {
		for _, g := range m[ent] {
			g = strings.TrimSpace(g)
			if g == "" {
				continue
			}
			if _, err := tx.Exec(ctx, `INSERT INTO platform_group_map (platform, entitlement, group_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
				platform, ent, g); err != nil {
				return err
			}
			after = append(after, ent+" -> "+g)
		}
	}
	var beforeList []string
	for _, b := range before {
		beforeList = append(beforeList, b.Entitlement+" -> "+b.GroupID)
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actorID, Action: "roles.sync_mapping", Reason: platform + " role mapping updated",
		Before: map[string]any{"platform": platform, "mappings": beforeList},
		After:  map[string]any{"platform": platform, "mappings": after},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// LogEntry is one recent sync_log row for the settings page.
type LogEntry struct {
	When     time.Time
	Player   string
	PlayerID int64
	Platform string
	Action   string
	Group    string
	OK       bool
	Error    string
}

// Recent returns the latest sync_log rows and the number of failures in
// the last 24 hours.
func Recent(ctx context.Context, pool *pgxpool.Pool, limit int) ([]LogEntry, int, error) {
	var failures int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sync_log WHERE NOT ok AND created_at > now() - interval '24 hours'`).Scan(&failures); err != nil {
		return nil, 0, err
	}
	rows, err := pool.Query(ctx, `
		SELECT l.created_at, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id, 'Deleted player'),
		       COALESCE(p.id, 0), l.platform, l.action, COALESCE(l.group_id, ''), l.ok, COALESCE(l.error, '')
		FROM sync_log l LEFT JOIN players p ON p.id = l.player_id
		ORDER BY l.id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, failures, err
	}
	defer rows.Close()
	var out []LogEntry
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.When, &e.Player, &e.PlayerID, &e.Platform, &e.Action, &e.Group, &e.OK, &e.Error); err != nil {
			return nil, failures, err
		}
		out = append(out, e)
	}
	return out, failures, rows.Err()
}
