// Package attention is the dashboard's "Needs attention" feed: the things
// waiting on staff right now -- unreviewed anti-cheat flags, cases that
// have gone quiet, unassigned tickets, applications, ban appeals and
// watched players in game -- each only for staff who can act on it.
package attention

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Levels, most urgent first.
const (
	Critical = "critical"
	Warning  = "warning"
	Info     = "info"
)

// Item is one line of the feed.
type Item struct {
	Level  string
	Title  string
	Detail string
	URL    string
}

// StaleCase is how long an open case can go without an entry before it's
// listed as gone quiet.
const StaleCase = 3 * 24 * time.Hour

// Viewer is what the viewer may see.
type Viewer struct {
	Perms   map[string]bool
	Support bool // support panel access (tickets)
}

// Collect builds the feed, most urgent first.
func Collect(ctx context.Context, pool *pgxpool.Pool, v Viewer, now time.Time) ([]Item, error) {
	var out []Item

	if v.Perms["anticheat.review"] {
		var high, medium int
		var oldest *time.Time
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FILTER (WHERE confidence = 'high'), count(*) FILTER (WHERE confidence = 'medium'),
			       min(created_at) FILTER (WHERE confidence = 'high')
			FROM anti_cheat_flags WHERE resolution IS NULL`).Scan(&high, &medium, &oldest); err != nil {
			return nil, err
		}
		if high > 0 {
			level, age := Warning, ""
			if oldest != nil {
				age = "oldest " + Ago(now, *oldest)
				if now.Sub(*oldest) > 24*time.Hour {
					level = Critical
				}
			}
			out = append(out, Item{level, plural(high, "high-confidence anti-cheat flag") + " to review", age, "/admin/anticheat?status=open&confidence=high"})
		}
		if medium > 0 {
			out = append(out, Item{Info, plural(medium, "medium-confidence flag") + " to review", "", "/admin/anticheat?status=open&confidence=medium"})
		}
	}

	if v.Support {
		var n int
		var oldest *time.Time
		if err := pool.QueryRow(ctx, `
			SELECT count(*), min(created_at) FROM support_tickets
			WHERE status <> 'closed' AND assigned_staff_id IS NULL`).Scan(&n, &oldest); err != nil {
			return nil, err
		}
		if n > 0 {
			level, age := Warning, ""
			if oldest != nil {
				age = "oldest " + Ago(now, *oldest)
				if now.Sub(*oldest) > 24*time.Hour {
					level = Critical
				}
			}
			out = append(out, Item{level, plural(n, "support ticket") + " nobody has picked up", age, "/admin/tickets"})
		}
	}

	if v.Perms["cases.view"] {
		rows, err := pool.Query(ctx, `
			SELECT c.id, c.summary, GREATEST(c.created_at, COALESCE(max(e.created_at), c.created_at))
			FROM staff_cases c LEFT JOIN staff_case_entries e ON e.case_id = c.id
			WHERE c.status = 'open' AND c.case_type <> 'ban_appeal'
			GROUP BY c.id
			HAVING GREATEST(c.created_at, COALESCE(max(e.created_at), c.created_at)) < $1
			ORDER BY 3`, now.Add(-StaleCase))
		if err != nil {
			return nil, err
		}
		var ids []string
		var first string
		var firstID int64
		for rows.Next() {
			var id int64
			var summary string
			var last time.Time
			if err := rows.Scan(&id, &summary, &last); err != nil {
				rows.Close()
				return nil, err
			}
			if firstID == 0 {
				firstID, first = id, fmt.Sprintf("#%d %s, quiet for %s", id, summary, Ago(now, last))
			}
			ids = append(ids, fmt.Sprintf("#%d", id))
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		switch len(ids) {
		case 0:
		case 1:
			out = append(out, Item{Warning, "An open case has gone quiet", first, fmt.Sprintf("/admin/cases/%d", firstID)})
		default:
			out = append(out, Item{Warning, plural(len(ids), "open case") + " gone quiet (3+ days without an entry)", list(ids, 6), "/admin/cases?status=open&range=all"})
		}

		var appeals int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM staff_cases WHERE status = 'open' AND case_type = 'ban_appeal'`).Scan(&appeals); err != nil {
			return nil, err
		}
		if appeals > 0 {
			out = append(out, Item{Info, plural(appeals, "ban appeal") + " waiting", "", "/admin/cases?status=open&type=ban_appeal&range=all"})
		}
	}

	if v.Perms["applications.view"] {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM staff_applications WHERE status IN ('pending', 'interview')`).Scan(&n); err != nil {
			return nil, err
		}
		if n > 0 {
			out = append(out, Item{Info, plural(n, "staff application") + " in progress", "", "/admin/applications"})
		}
	}

	if v.Perms["players.view"] {
		rows, err := pool.Query(ctx, `
			SELECT COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id)
			FROM player_watchlist w JOIN players p ON p.id = w.player_id
			WHERE EXISTS (SELECT 1 FROM player_sessions s WHERE s.player_id = p.id AND s.disconnected_at IS NULL)
			ORDER BY 1`)
		if err != nil {
			return nil, err
		}
		var names []string
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				rows.Close()
				return nil, err
			}
			names = append(names, n)
		}
		rows.Close()
		if len(names) > 0 {
			title := "A watched player is in game"
			if len(names) > 1 {
				title = fmt.Sprintf("%d watched players are in game", len(names))
			}
			out = append(out, Item{Info, title, list(names, 5), "/admin/online"})
		}
	}

	// Most urgent first; within a level, the order above.
	rank := map[string]int{Critical: 0, Warning: 1, Info: 2}
	sorted := make([]Item, 0, len(out))
	for _, lvl := range []string{Critical, Warning, Info} {
		for _, it := range out {
			if rank[it.Level] == rank[lvl] {
				sorted = append(sorted, it)
			}
		}
	}
	return sorted, nil
}

func plural(n int, what string) string {
	if n == 1 {
		return "1 " + what
	}
	return fmt.Sprintf("%d %ss", n, what)
}

func list(xs []string, max int) string {
	if len(xs) <= max {
		return strings.Join(xs, ", ")
	}
	return strings.Join(xs[:max], ", ") + fmt.Sprintf(" and %d more", len(xs)-max)
}

// Ago is a rough age: "45 min", "5 h", "3 days".
func Ago(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h", int(d.Hours()))
	}
	return fmt.Sprintf("%d days", int(d.Hours()/24))
}
