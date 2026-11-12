// Package inbox is the staff inbox: one list of everything waiting on
// staff across Support and Moderation -- open tickets, player reports with
// no case yet, ban appeals, high-confidence anti-cheat flags, cases that
// have gone quiet or that you lead, and staff applications -- each only
// for staff who can act on it, most urgent first.
package inbox

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/cases"
)

// Kinds, for the filter chips.
var Kinds = []Kind{{"ticket", "Tickets"}, {"report", "Reports"}, {"appeal", "Ban appeals"}, {"flag", "Flags"}, {"case", "Cases"}, {"application", "Applications"}}

type Kind struct{ Key, Label string }

// Item is one thing waiting.
type Item struct {
	Kind     string
	ID       int64
	Title    string
	Detail   string
	URL      string
	Since    time.Time // waiting since
	Urgency  int       // 3 urgent, 2 high, 1 normal, 0 low
	Owner    string    // who has it ("" = nobody)
	Mine     bool      // assigned to / led by the viewer
	CanClaim bool      // a ticket the viewer can assign to themselves
	Waiting  bool      // a ticket whose last word is the player's (they're waiting on staff)
}

// Viewer is who's looking and what they may see.
type Viewer struct {
	PlayerID int64
	Perms    map[string]bool
	Support  bool // support panel access
}

// QuietAfter is how long an open case can go without an entry before it
// shows up for everyone (cases you lead always do).
const QuietAfter = 3 * 24 * time.Hour

// Collect builds the inbox.
func Collect(ctx context.Context, pool *pgxpool.Pool, v Viewer) ([]Item, error) {
	var out []Item
	name := func(alias string) string {
		return `COALESCE(NULLIF(` + alias + `.name, ''), NULLIF(` + alias + `.steam_name, ''), '')`
	}

	if v.Support {
		rows, err := pool.Query(ctx, `
			SELECT t.id, t.subject, t.priority, t.status, t.created_at, COALESCE(t.assigned_staff_id, 0), `+name("a")+`, `+name("p")+`,
			       COALESCE((SELECT m.author_player_id = t.player_id FROM support_ticket_messages m
			                 WHERE m.ticket_id = t.id AND NOT m.internal ORDER BY m.id DESC LIMIT 1), true),
			       COALESCE(tc.key, '')
			FROM support_tickets t
			JOIN players p ON p.id = t.player_id
			LEFT JOIN players a ON a.id = t.assigned_staff_id
			LEFT JOIN ticket_categories tc ON tc.id = t.subcategory_id
			WHERE t.status <> 'closed' AND t.case_id IS NULL
			ORDER BY t.created_at LIMIT 200`)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var it Item
			var prio, status, owner, requester, sub string
			var assignee int64
			if err := rows.Scan(&it.ID, &it.Title, &prio, &status, &it.Since, &assignee, &owner, &requester, &it.Waiting, &sub); err != nil {
				rows.Close()
				return nil, err
			}
			isReport := false
			for _, k := range cases.ReportCategories {
				isReport = isReport || k == sub
			}
			if isReport && v.Perms["cases.view"] {
				continue // shown as a report instead
			}
			it.Kind, it.URL, it.Owner = "ticket", fmt.Sprintf("/admin/tickets/%d", it.ID), owner
			it.Urgency = map[string]int{"urgent": 3, "high": 2, "normal": 1, "low": 0}[prio]
			// Nobody has picked it up in a day: as pressing as a high one.
			if assignee == 0 && time.Since(it.Since) > 24*time.Hour && it.Urgency < 2 {
				it.Urgency = 2
			}
			it.Mine = assignee == v.PlayerID
			it.CanClaim = assignee != v.PlayerID
			it.Detail = requester + " · " + status
			if it.Waiting {
				it.Detail += " · waiting on staff"
			}
			out = append(out, it)
		}
		rows.Close()
	}

	if v.Perms["cases.view"] {
		reps, err := cases.Reports(ctx, pool, "new", 100)
		if err != nil {
			return nil, err
		}
		for _, r := range reps {
			out = append(out, Item{Kind: "report", ID: r.TicketID, Title: r.Subject, Detail: "reported by " + r.Reporter + " · " + r.Category,
				URL: "/admin/reports", Since: r.Created, Urgency: 2})
		}
		rows, err := pool.Query(ctx, `
			SELECT c.id, c.summary, COALESCE(c.lead_staff_id, 0), `+name("l")+`,
			       GREATEST(c.created_at, COALESCE((SELECT max(e.created_at) FROM staff_case_entries e WHERE e.case_id = c.id), c.created_at))
			FROM staff_cases c LEFT JOIN players l ON l.id = c.lead_staff_id
			WHERE c.status = 'open' ORDER BY c.id`)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var it Item
			var lead int64
			if err := rows.Scan(&it.ID, &it.Title, &lead, &it.Owner, &it.Since); err != nil {
				rows.Close()
				return nil, err
			}
			it.Mine = lead == v.PlayerID
			quiet := time.Since(it.Since) > QuietAfter
			if !it.Mine && !quiet {
				continue
			}
			it.Kind, it.URL = "case", fmt.Sprintf("/admin/cases/%d", it.ID)
			it.Title = fmt.Sprintf("#%d %s", it.ID, it.Title)
			it.Detail = "last entry " + ago(it.Since)
			if quiet {
				it.Detail += " · gone quiet"
				it.Urgency = 1
			}
			out = append(out, it)
		}
		rows.Close()
	}

	if v.Perms["bans.appeal_review"] {
		rows, err := pool.Query(ctx, `
			SELECT a.id, `+name("p")+`, b.reason, a.created_at
			FROM ban_appeals a JOIN players p ON p.id = a.player_id LEFT JOIN banlist b ON b.id = a.ban_id
			WHERE a.status = 'open' ORDER BY a.created_at`)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var it Item
			var who string
			var reason *string
			if err := rows.Scan(&it.ID, &who, &reason, &it.Since); err != nil {
				rows.Close()
				return nil, err
			}
			it.Kind, it.URL, it.Urgency = "appeal", "/admin/bans?tab=appeals", 2
			it.Title = "Ban appeal from " + who
			if reason != nil {
				it.Detail = "banned for: " + *reason
			}
			out = append(out, it)
		}
		rows.Close()
	}

	if v.Perms["anticheat.review"] {
		rows, err := pool.Query(ctx, `
			SELECT f.id, replace(f.flag_type, '_', ' '), `+name("p")+`, f.created_at
			FROM anti_cheat_flags f JOIN players p ON p.id = f.player_id
			WHERE f.resolution IS NULL AND f.confidence = 'high' ORDER BY f.created_at LIMIT 50`)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var it Item
			var typ, who string
			if err := rows.Scan(&it.ID, &typ, &who, &it.Since); err != nil {
				rows.Close()
				return nil, err
			}
			it.Kind, it.URL, it.Urgency = "flag", fmt.Sprintf("/admin/anticheat?flag=%d", it.ID), 2
			it.Title = fmt.Sprintf("Flag #%d: %s", it.ID, typ)
			it.Detail = who + " · high confidence"
			if time.Since(it.Since) > 24*time.Hour {
				it.Urgency = 3
			}
			out = append(out, it)
		}
		rows.Close()
	}

	if v.Perms["applications.view"] {
		rows, err := pool.Query(ctx, `
			SELECT a.id, `+name("p")+`, a.status, a.created_at FROM staff_applications a JOIN players p ON p.id = a.player_id
			WHERE a.status IN ('pending', 'interview') ORDER BY a.created_at`)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var it Item
			var who, status string
			if err := rows.Scan(&it.ID, &who, &status, &it.Since); err != nil {
				rows.Close()
				return nil, err
			}
			it.Kind, it.URL, it.Urgency = "application", fmt.Sprintf("/admin/applications?id=%d", it.ID), 0
			it.Title, it.Detail = "Staff application from "+who, status
			out = append(out, it)
		}
		rows.Close()
	}

	// Most urgent first, then longest waiting.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Urgency != out[j].Urgency {
			return out[i].Urgency > out[j].Urgency
		}
		return out[i].Since.Before(out[j].Since)
	})
	return out, nil
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d days ago", int(d.Hours()/24))
}
