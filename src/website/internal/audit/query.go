package audit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Categories group staff_log actions for the Staff Log page's filter pills
// (the layout plan's Staff Log board). Actions that fit none are "Other"
// and only appear under "All".
var Categories = []string{"Moderation", "Economy", "Tickets", "Database", "Permissions"}

// Category maps a raw staff_log action to its category.
func Category(action string) string {
	switch {
	case strings.HasPrefix(action, "rank_change:cop_level"), strings.HasPrefix(action, "rank_change:medic_level"):
		return "Permissions"
	case strings.HasPrefix(action, "rank_change:"), strings.HasPrefix(action, "rank_"),
		strings.HasPrefix(action, "faction_rank_names:"), strings.HasPrefix(action, "staff."),
		strings.HasPrefix(action, "permissions."), strings.HasPrefix(action, "roles."):
		return "Permissions"
	case strings.HasPrefix(action, "player.compensate"), strings.HasPrefix(action, "bank."), strings.HasPrefix(action, "economy."):
		return "Economy"
	case strings.HasPrefix(action, "player."), strings.HasPrefix(action, "ban"), strings.HasPrefix(action, "case"),
		strings.HasPrefix(action, "anticheat."), strings.HasPrefix(action, "kick"),
		action == "discord.kick", action == "discord.ban", action == "discord.unban", strings.HasPrefix(action, "discord.timeout"):
		return "Moderation"
	case strings.HasPrefix(action, "ticket."):
		return "Tickets"
	case strings.HasPrefix(action, "database."):
		return "Database"
	}
	return "Other"
}

// categoryPatterns are the SQL LIKE patterns matching each category, kept
// in step with Category above.
var categoryPatterns = map[string][]string{
	"Permissions": {"rank\\_%", "faction\\_rank\\_names:%", "staff.%", "permissions.%", "roles.%"},
	"Economy":     {"player.compensate%", "bank.%", "economy.%"},
	"Moderation":  {"player.%", "ban%", "case%", "anticheat.%", "kick%", "discord.kick", "discord.ban", "discord.unban", "discord.timeout%"},
	"Tickets":     {"ticket.%"},
	"Database":    {"database.%"},
}

// DisplayAction renders a raw action in the dotted style the layout plan
// uses (e.g. "rank_change:staff_status" -> "staff.status").
func DisplayAction(action string) string {
	switch {
	case action == "rank_change:staff_rank_id":
		return "staff.rank"
	case action == "rank_change:staff_status":
		return "staff.status"
	case action == "rank_change:staff_team":
		return "staff.team"
	case action == "rank_change:cop_level":
		return "player.police_level"
	case action == "rank_change:medic_level":
		return "player.ems_level"
	case strings.HasPrefix(action, "rank_edit:"):
		return "roles.edit"
	case strings.HasPrefix(action, "rank_create:"):
		return "roles.create"
	case strings.HasPrefix(action, "rank_delete:"):
		return "roles.delete"
	case action == "rank_reorder":
		return "roles.reorder"
	case strings.HasPrefix(action, "faction_rank_names:"):
		return "factions.rank_names"
	}
	return action
}

// Filter narrows a staff log query. Zero values mean "no filter".
type Filter struct {
	Category string // one of Categories
	Search   string // matches action, reason, and staff/target names
	StaffID  int64
	Target   string    // name substring
	Action   string    // raw action prefix
	Since    time.Time // zero = all time
	BeforeID int64     // keyset paging: entries older than this id
	AfterID  int64     // keyset paging: entries newer than this id
	Limit    int
}

// LogEntry is one staff_log row, ready to display.
type LogEntry struct {
	ID        int64
	Time      time.Time
	Staff     string
	StaffRank string
	Target    string
	Action    string // raw
	Reason    string
	Before    []byte
	After     []byte
	Source    string
}

func (e LogEntry) Category() string      { return Category(e.Action) }
func (e LogEntry) DisplayAction() string { return DisplayAction(e.Action) }
func (e LogEntry) Summary() string {
	s := Describe(e.Action, e.Before, e.After)
	if e.Reason != "" {
		s += ": " + e.Reason
	}
	return s
}

const entrySelect = `
	SELECT sl.id, sl.created_at,
	       COALESCE(NULLIF(s.name, ''), NULLIF(s.steam_name, ''), 'Player #' || s.id,
	                CASE sl.source WHEN 'game' THEN 'In-game' WHEN 'manual' THEN 'Manual DB change' ELSE 'System' END),
	       COALESCE(sr.display_name, ''),
	       COALESCE(NULLIF(t.name, ''), NULLIF(t.steam_name, ''), 'Player #' || t.id, ''),
	       sl.action, COALESCE(sl.reason, ''), sl.before_value, sl.after_value, sl.source
	FROM staff_log sl
	LEFT JOIN players s ON s.id = sl.staff_player_id
	LEFT JOIN staff_ranks sr ON sr.id = s.staff_rank_id
	LEFT JOIN players t ON t.id = sl.target_player_id`

func (f Filter) where() (string, []any) {
	var conds []string
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if pats, ok := categoryPatterns[f.Category]; ok {
		var ors []string
		for _, p := range pats {
			ors = append(ors, "sl.action LIKE "+arg(p))
		}
		conds = append(conds, "("+strings.Join(ors, " OR ")+")")
		if f.Category == "Moderation" { // player.compensate belongs to Economy
			conds = append(conds, "sl.action NOT LIKE 'player.compensate%'")
		}
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		p := arg("%" + escapeLike(s) + "%")
		conds = append(conds, fmt.Sprintf(`(sl.action ILIKE %[1]s OR sl.reason ILIKE %[1]s OR s.name ILIKE %[1]s OR s.steam_name ILIKE %[1]s OR t.name ILIKE %[1]s OR t.steam_name ILIKE %[1]s)`, p))
	}
	if f.StaffID > 0 {
		conds = append(conds, "sl.staff_player_id = "+arg(f.StaffID))
	}
	if s := strings.TrimSpace(f.Target); s != "" {
		p := arg("%" + escapeLike(s) + "%")
		conds = append(conds, fmt.Sprintf("(t.name ILIKE %[1]s OR t.steam_name ILIKE %[1]s)", p))
	}
	if f.Action != "" {
		conds = append(conds, "sl.action LIKE "+arg(escapeLike(f.Action)+"%"))
	}
	if !f.Since.IsZero() {
		conds = append(conds, "sl.created_at >= "+arg(f.Since))
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// Page is one page of entries plus what's needed for Newer/Older links.
type Page struct {
	Entries  []LogEntry
	Total    int // entries matching the filter (all pages)
	HasOlder bool
	HasNewer bool
}

// Query returns one page of staff_log entries, newest first.
func Query(ctx context.Context, pool *pgxpool.Pool, f Filter) (Page, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	where, args := f.where()
	var p Page
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM staff_log sl
		LEFT JOIN players s ON s.id = sl.staff_player_id
		LEFT JOIN players t ON t.id = sl.target_player_id`+where, args...).Scan(&p.Total); err != nil {
		return p, err
	}

	page := where
	order := " ORDER BY sl.id DESC"
	pageArgs := append([]any{}, args...)
	add := func(cond string, v any) {
		pageArgs = append(pageArgs, v)
		c := fmt.Sprintf(cond, len(pageArgs))
		if page == "" {
			page = " WHERE " + c
		} else {
			page += " AND " + c
		}
	}
	switch {
	case f.AfterID > 0:
		add("sl.id > $%d", f.AfterID)
		order = " ORDER BY sl.id ASC" // take the nearest newer ones, then flip
	case f.BeforeID > 0:
		add("sl.id < $%d", f.BeforeID)
	}
	pageArgs = append(pageArgs, f.Limit+1)
	rows, err := pool.Query(ctx, entrySelect+page+order+fmt.Sprintf(" LIMIT $%d", len(pageArgs)), pageArgs...)
	if err != nil {
		return p, err
	}
	defer rows.Close()
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.Time, &e.Staff, &e.StaffRank, &e.Target, &e.Action, &e.Reason, &e.Before, &e.After, &e.Source); err != nil {
			return p, err
		}
		p.Entries = append(p.Entries, e)
	}
	if err := rows.Err(); err != nil {
		return p, err
	}
	more := len(p.Entries) > f.Limit
	if more {
		p.Entries = p.Entries[:f.Limit]
	}
	if f.AfterID > 0 {
		for i, j := 0, len(p.Entries)-1; i < j; i, j = i+1, j-1 {
			p.Entries[i], p.Entries[j] = p.Entries[j], p.Entries[i]
		}
		p.HasNewer, p.HasOlder = more, true
	} else {
		p.HasOlder, p.HasNewer = more, f.BeforeID > 0
	}
	return p, nil
}

// Get returns one entry by id.
func Get(ctx context.Context, pool *pgxpool.Pool, id int64) (LogEntry, error) {
	var e LogEntry
	err := pool.QueryRow(ctx, entrySelect+" WHERE sl.id = $1", id).
		Scan(&e.ID, &e.Time, &e.Staff, &e.StaffRank, &e.Target, &e.Action, &e.Reason, &e.Before, &e.After, &e.Source)
	return e, err
}

// StaffOption is one choice in the Staff filter.
type StaffOption struct {
	ID   int64
	Name string
}

// StaffWithEntries lists staff who appear in the log, for the Staff filter.
func StaffWithEntries(ctx context.Context, pool *pgxpool.Pool) ([]StaffOption, error) {
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT p.id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id)
		FROM staff_log sl JOIN players p ON p.id = sl.staff_player_id ORDER BY 2`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StaffOption
	for rows.Next() {
		var o StaffOption
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
