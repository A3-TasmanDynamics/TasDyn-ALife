// Package timeline is Player Lookup's Timeline tab: one chronological view
// of everything the panel knows about a player -- sessions, game events,
// anti-cheat flags, cases, bans, kicks, wanted crimes, staff notes, staff
// actions and in-game commands -- newest first, paged by time.
package timeline

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
	"website/internal/events"
)

// Kinds, in the order the filter chips show them.
var Kinds = []Kind{
	{"session", "Sessions"}, {"event", "Game events"}, {"wanted", "Crimes"}, {"flag", "Flags"},
	{"case", "Cases"}, {"ban", "Bans & kicks"}, {"note", "Notes"}, {"staff", "Staff actions"}, {"command", "From the panel"},
}

type Kind struct{ Key, Label string }

// Item is one line.
type Item struct {
	When   time.Time
	Kind   string
	Title  string
	Detail string
	URL    string
}

// Opts says whose timeline, which page, and what the viewer may see.
type Opts struct {
	PlayerID int64
	Before   time.Time       // newest first, strictly older than this (zero = now)
	Limit    int             // items per page
	Only     map[string]bool // kinds to show (empty = all the viewer may see)
	// What the viewer may see (the matching tabs / pages' permissions).
	Notes, Flags, Cases bool
}

func (o Opts) wants(kind string) bool {
	switch kind {
	case "note":
		if !o.Notes {
			return false
		}
	case "flag":
		if !o.Flags {
			return false
		}
	case "case":
		if !o.Cases {
			return false
		}
	}
	return len(o.Only) == 0 || o.Only[kind]
}

// Build returns one page and whether there's more before it.
func Build(ctx context.Context, pool *pgxpool.Pool, o Opts) ([]Item, bool, error) {
	if o.Limit <= 0 || o.Limit > 200 {
		o.Limit = 60
	}
	if o.Before.IsZero() {
		o.Before = time.Now().Add(time.Minute)
	}
	n := o.Limit + 1 // one more than a page from each source tells us if there's more
	var all []Item
	add := func(it Item) { all = append(all, it) }

	type src struct {
		kind string
		sql  string
		row  func(scan func(...any) error) (Item, error)
	}
	name := func(alias string) string {
		return `COALESCE(NULLIF(` + alias + `.name, ''), NULLIF(` + alias + `.steam_name, ''), 'Staff')`
	}
	sources := []src{
		{"session", `SELECT connected_at, disconnected_at, COALESCE(faction, '') FROM player_sessions
			WHERE player_id = $1 AND connected_at < $2 ORDER BY connected_at DESC LIMIT $3`,
			func(scan func(...any) error) (Item, error) {
				var at time.Time
				var end *time.Time
				var fac string
				if err := scan(&at, &end, &fac); err != nil {
					return Item{}, err
				}
				as := map[string]string{"civ": " as a civilian", "cop": " as police", "medic": " as EMS"}[fac]
				detail := "still in game"
				if end != nil {
					detail = "played " + span(end.Sub(at))
				}
				return Item{at, "session", "Joined the server" + as, detail, "&tab=sessions"}, nil
			}},
		{"flag", `SELECT id, created_at, replace(flag_type, '_', ' '), confidence, COALESCE(resolution, '') FROM anti_cheat_flags
			WHERE player_id = $1 AND created_at < $2 ORDER BY created_at DESC LIMIT $3`,
			func(scan func(...any) error) (Item, error) {
				var id int64
				var at time.Time
				var typ, conf, res string
				if err := scan(&id, &at, &typ, &conf, &res); err != nil {
					return Item{}, err
				}
				state := map[string]string{"": "open", "dismiss": "dismissed", "watch": "watching", "case": "in a case", "kick": "kicked", "ban": "banned"}[res]
				if state == "" {
					state = res
				}
				return Item{at, "flag", fmt.Sprintf("Anti-cheat flag #%d: %s", id, typ), conf + " confidence · " + state, fmt.Sprintf("/admin/anticheat?flag=%d", id)}, nil
			}},
		{"case", `SELECT c.id, c.created_at, c.summary, cp.role, c.status FROM staff_case_participants cp JOIN staff_cases c ON c.id = cp.case_id
			WHERE cp.player_id = $1 AND c.created_at < $2 ORDER BY c.created_at DESC LIMIT $3`,
			func(scan func(...any) error) (Item, error) {
				var id int64
				var at time.Time
				var summary, role, status string
				if err := scan(&id, &at, &summary, &role, &status); err != nil {
					return Item{}, err
				}
				return Item{at, "case", fmt.Sprintf("Case #%d: %s", id, summary), "as " + role + " · " + status, fmt.Sprintf("/admin/cases/%d", id)}, nil
			}},
		{"ban", `SELECT b.created_at, b.reason, b.expires_at, ` + name("s") + ` FROM banlist b LEFT JOIN players s ON s.id = b.banned_by
			WHERE b.uid = (SELECT uid FROM players WHERE id = $1) AND b.created_at < $2 ORDER BY b.created_at DESC LIMIT $3`,
			func(scan func(...any) error) (Item, error) {
				var at time.Time
				var reason, by string
				var until *time.Time
				if err := scan(&at, &reason, &until, &by); err != nil {
					return Item{}, err
				}
				length := "permanent"
				if until != nil {
					length = "until " + until.Local().Format("2 Jan 2006 15:04")
				}
				return Item{at, "ban", "Banned: " + reason, length + " · by " + by, "/admin/bans"}, nil
			}},
		{"ban", `SELECT k.created_at, COALESCE(k.reason, ''), k.kick_type, COALESCE(` + name("s") + `, 'System') FROM kick_log k LEFT JOIN players s ON s.id = k.kicked_by
			WHERE k.target_player_id = $1 AND k.created_at < $2 ORDER BY k.created_at DESC LIMIT $3`,
			func(scan func(...any) error) (Item, error) {
				var at time.Time
				var reason, typ, by string
				if err := scan(&at, &reason, &typ, &by); err != nil {
					return Item{}, err
				}
				t := "Kicked"
				if reason != "" {
					t += ": " + reason
				}
				return Item{at, "ban", t, typ + " · by " + by, ""}, nil
			}},
		{"wanted", `SELECT created_at, replace(crime_key, '_', ' '), bounty_amount, cleared_at IS NOT NULL FROM wanted_crimes
			WHERE player_id = $1 AND created_at < $2 ORDER BY created_at DESC LIMIT $3`,
			func(scan func(...any) error) (Item, error) {
				var at time.Time
				var crime string
				var bounty int64
				var cleared bool
				if err := scan(&at, &crime, &bounty, &cleared); err != nil {
					return Item{}, err
				}
				d := fmt.Sprintf("bounty $%s", money(bounty))
				if cleared {
					d += " · cleared"
				}
				return Item{at, "wanted", "Wanted for " + crime, d, "/admin/wanted"}, nil
			}},
		{"note", `SELECT n.created_at, n.body, ` + name("s") + ` FROM staff_notes n LEFT JOIN players s ON s.id = n.author_id
			WHERE n.player_id = $1 AND n.kind = 'player' AND n.created_at < $2 ORDER BY n.created_at DESC LIMIT $3`,
			func(scan func(...any) error) (Item, error) {
				var at time.Time
				var body, by string
				if err := scan(&at, &body, &by); err != nil {
					return Item{}, err
				}
				return Item{at, "note", "Staff note by " + by, clip(body, 240), "&tab=notes"}, nil
			}},
		{"staff", `SELECT l.created_at, l.action, l.before_value, l.after_value, COALESCE(l.reason, ''),
			       COALESCE(` + name("s") + `, CASE l.source WHEN 'game' THEN 'In game' ELSE 'System' END)
			FROM staff_log l LEFT JOIN players s ON s.id = l.staff_player_id
			WHERE l.target_player_id = $1 AND l.created_at < $2 AND l.action NOT IN ('player.kick', 'ban.issue', 'game.message', 'game.warn')
			ORDER BY l.created_at DESC LIMIT $3`,
			func(scan func(...any) error) (Item, error) {
				var at time.Time
				var action, reason, by string
				var before, after []byte
				if err := scan(&at, &action, &before, &after, &reason, &by); err != nil {
					return Item{}, err
				}
				return Item{at, "staff", by + " " + audit.Describe(action, before, after), clip(reason, 200), "&tab=audit"}, nil
			}},
		{"command", `SELECT g.created_at, g.action, g.text, ` + name("s") + `, g.status FROM game_commands g LEFT JOIN players s ON s.id = g.issued_by
			WHERE g.target_player_id = $1 AND g.created_at < $2 ORDER BY g.created_at DESC LIMIT $3`,
			func(scan func(...any) error) (Item, error) {
				var at time.Time
				var action, text, by, status string
				if err := scan(&at, &action, &text, &by, &status); err != nil {
					return Item{}, err
				}
				verb := map[string]string{"message": "messaged them", "warn": "warned them", "kick": "kicked them"}[action]
				return Item{at, "command", by + " " + verb + " from the panel", clip(text, 200) + " · " + status, "/admin/online"}, nil
			}},
	}
	for _, s := range sources {
		if !o.wants(s.kind) {
			continue
		}
		rows, err := pool.Query(ctx, s.sql, o.PlayerID, o.Before, n)
		if err != nil {
			return nil, false, fmt.Errorf("timeline %s: %w", s.kind, err)
		}
		for rows.Next() {
			it, err := s.row(rows.Scan)
			if err != nil {
				rows.Close()
				return nil, false, fmt.Errorf("timeline %s: %w", s.kind, err)
			}
			add(it)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, false, err
		}
	}
	if o.wants("event") {
		evs, _, err := events.List(ctx, pool, events.Filter{PlayerID: o.PlayerID, To: o.Before.Add(-time.Microsecond), Limit: n})
		if err != nil {
			return nil, false, err
		}
		for _, e := range evs {
			add(Item{e.When, "event", e.Summary, e.Category, "&tab=events"})
		}
	}
	for i := range all {
		if len(all[i].URL) > 0 && all[i].URL[0] == '&' { // a Player Lookup tab
			all[i].URL = fmt.Sprintf("/admin/players?id=%d%s", o.PlayerID, all[i].URL)
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].When.After(all[j].When) })
	more := len(all) > o.Limit
	if more {
		all = all[:o.Limit]
	}
	return all, more, nil
}

func span(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

func money(n int64) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
