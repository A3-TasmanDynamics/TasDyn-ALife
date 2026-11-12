// Package quicksearch is the admin panel's Ctrl+K search: one box that
// finds pages, players (name, past names, Steam ID, BattlEye GUID, #id),
// vehicles (plate, #id), gangs (name, tag), cases (#id, summary) and
// anti-cheat flags (#id), each only for staff who may see it.
package quicksearch

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/gamenames"
	"website/internal/players"
)

// Hit is one result.
type Hit struct {
	Title string `json:"title"`
	Sub   string `json:"sub,omitempty"`
	URL   string `json:"url"`
}

// Group is one kind of result.
type Group struct {
	Name string `json:"name"`
	Hits []Hit  `json:"hits"`
}

// perGroup is how many hits each group shows.
const perGroup = 5

// Page is an admin page the search can jump to.
type Page struct {
	Title, URL, Perm string // Perm "" = anyone with the admin panel
	Words            string // extra words it's found by
}

// Pages are the admin pages, in sidebar order.
var Pages = []Page{
	{"Dashboard", "/admin", "", "home overview"},
	{"Online Now", "/admin/online", "players.view", "who's playing in game players count"},
	{"Watchlist", "/admin/watchlist", "players.view", "watched suspects"},
	{"Player Lookup", "/admin/players", "players.view", "search find"},
	{"Event Log", "/admin/events", "players.view", "history activity log"},
	{"Economy & Crime", "/admin/economy", "players.view", "money supply market prices robberies"},
	{"Gangs", "/admin/gangs", "players.view", "groups crews"},
	{"Wanted & Jail", "/admin/wanted", "players.view", "bounties prison"},
	{"Vehicle Lookup", "/admin/vehicles", "players.vehicles", "cars plates garage impound"},
	{"Houses & Garages", "/admin/houses", "players.view", "property"},
	{"Cases", "/admin/cases", "cases.view", "moderation reports"},
	{"Bans", "/admin/bans", "cases.view", "banned"},
	{"Anti-Cheat Flags", "/admin/anticheat", "anticheat.review", "cheating detections"},
	{"Staff Directory", "/admin/staff", "staff.view", "team members"},
	{"Roles & Permissions", "/admin/roles", "", "ranks access"},
	{"Staff Log", "/admin/staff-log", "", "audit actions"},
	{"Applications", "/admin/applications", "applications.view", "staff apply"},
	{"Server Control", "/admin/server", "server.logs", "restart rpt logs status"},
	{"Database", "/admin/database", "database.query", "sql query"},
	{"Item Prices", "/admin/prices", "players.compensate", "shop costs"},
	{"Essential Notices", "/admin/notices", "announce.post", "announcement"},
	{"Server Rules", "/admin/rules", "rules.edit", "rulebook"},
	{"Discord Bot", "/admin/discord", "bot.admin", "channels alerts settings"},
}

// Search runs q against everything the viewer may see (perms: their
// permission keys).
func Search(ctx context.Context, pool *pgxpool.Pool, q string, perms map[string]bool) ([]Group, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	var out []Group
	add := func(name string, hits []Hit) {
		if len(hits) > 0 {
			out = append(out, Group{name, hits})
		}
	}
	// Pages whose title matches come first; ones found only by their extra
	// words ("plates", "market") go last, under the records.
	titled, worded := pages(q, perms)
	add("Pages", titled)

	// "#12", "12": an id for players, vehicles, cases and flags.
	id, _ := strconv.ParseInt(strings.TrimPrefix(q, "#"), 10, 64)
	like := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.TrimPrefix(q, "#")) + "%"

	if perms["players.view"] {
		res, err := players.Search(ctx, pool, strings.TrimPrefix(q, "#"))
		if err != nil {
			return nil, err
		}
		var hits []Hit
		for _, r := range res {
			if len(hits) == perGroup {
				break
			}
			sub := r.Hint
			if r.Status != "" {
				sub = r.Status + " · " + sub
			}
			hits = append(hits, Hit{r.Name, sub, fmt.Sprintf("/admin/players?id=%d", r.ID)})
		}
		add("Players", hits)

		gangs, err := query(ctx, pool, `
			SELECT g.id, g.name || ' [' || g.tag || ']', (SELECT count(*) FROM gang_members m WHERE m.gang_id = g.id)::text || ' members'
			FROM gangs g WHERE g.name ILIKE $1 OR g.tag ILIKE $1 ORDER BY g.name LIMIT $2`, like, perGroup)
		if err != nil {
			return nil, err
		}
		add("Gangs", hits3(gangs, "/admin/gangs?id=%d"))
	}

	if perms["players.vehicles"] {
		rows, err := pool.Query(ctx, `
			SELECT v.id, v.classname, COALESCE(v.plate, ''), v.status,
			       COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), (SELECT g.name FROM gangs g WHERE g.id = v.owner_gang_id), '')
			FROM vehicles v LEFT JOIN players p ON p.id = v.owner_player_id
			WHERE v.plate ILIKE $1 OR v.id = $2
			ORDER BY (v.id = $2) DESC, v.id DESC LIMIT $3`, like, id, perGroup)
		if err != nil {
			return nil, err
		}
		var hits []Hit
		for rows.Next() {
			var vid int64
			var class, plate, status, owner string
			if err := rows.Scan(&vid, &class, &plate, &status, &owner); err != nil {
				rows.Close()
				return nil, err
			}
			name, ok := gamenames.Any(class)
			if !ok {
				name = class
			}
			title := name
			if plate != "" {
				title = plate + " · " + name
			}
			sub := status
			if owner != "" {
				sub = owner + " · " + status
			}
			hits = append(hits, Hit{title, sub, fmt.Sprintf("/admin/vehicles?id=%d", vid)})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		add("Vehicles", hits)
	}

	if perms["cases.view"] {
		cases, err := query(ctx, pool, `
			SELECT c.id, '#' || c.id || ' ' || c.summary, initcap(replace(c.case_type, '_', ' ')) || ' · ' || c.status
			FROM staff_cases c
			WHERE c.id = $2 OR c.summary ILIKE $1
			   OR EXISTS (SELECT 1 FROM staff_case_participants cp JOIN players p ON p.id = cp.player_id
			              WHERE cp.case_id = c.id AND (p.name ILIKE $1 OR p.uid = $3))
			ORDER BY (c.id = $2) DESC, (c.status = 'open') DESC, c.id DESC LIMIT $4`, like, id, q, perGroup)
		if err != nil {
			return nil, err
		}
		add("Cases", hits3(cases, "/admin/cases/%d"))
	}

	if perms["anticheat.review"] && id > 0 {
		flags, err := query(ctx, pool, `
			SELECT f.id, 'Flag #' || f.id || ' · ' || replace(f.flag_type, '_', ' '),
			       COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id) || ' · ' || f.confidence || ' · ' || COALESCE(f.resolution, 'open')
			FROM anti_cheat_flags f JOIN players p ON p.id = f.player_id
			WHERE f.id = $1 LIMIT 1`, id)
		if err != nil {
			return nil, err
		}
		add("Anti-cheat flags", hits3(flags, "/admin/anticheat?flag=%d"))
	}
	if len(titled) == 0 {
		add("Pages", worded)
	}
	return out, nil
}

// pages are the admin pages the viewer may open whose title (titled) or
// title and extra words (worded) contain every word of q, three at most.
func pages(q string, perms map[string]bool) (titled, worded []Hit) {
	words := strings.Fields(strings.ToLower(q))
	has := func(hay string) bool {
		for _, w := range words {
			if !strings.Contains(hay, w) {
				return false
			}
		}
		return true
	}
	for _, p := range Pages {
		if p.Perm != "" && !perms[p.Perm] {
			continue
		}
		hit := Hit{Title: p.Title, Sub: "Page", URL: p.URL}
		switch {
		case has(strings.ToLower(p.Title)):
			if len(titled) < 3 {
				titled = append(titled, hit)
			}
		case has(strings.ToLower(p.Title + " " + p.Words)):
			if len(worded) < 3 {
				worded = append(worded, hit)
			}
		}
	}
	return titled, worded
}

type row3 struct {
	id         int64
	title, sub string
}

func query(ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) ([]row3, error) {
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []row3
	for rows.Next() {
		var r row3
		if err := rows.Scan(&r.id, &r.title, &r.sub); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func hits3(rows []row3, url string) []Hit {
	var out []Hit
	for _, r := range rows {
		out = append(out, Hit{r.title, r.sub, fmt.Sprintf(url, r.id)})
	}
	return out
}
