package dashboard

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PublicBoard is one board on the public /leaderboards page. Players
// appear only when they've opted in (players.leaderboard_opt_in); wealth
// is shown as a band, never an exact balance.
type PublicBoard struct {
	Key   string // anchor and icon: wealth, playtime, houses, vehicles, police, ems, wanted, gang_wealth, gang_size
	Title string
	Sub   string
	Rows  []BoardRow
	Count int    // entries on the board
	You   string // the viewer's position, "" when not on it
}

// Leaderboard is the whole public page.
type Leaderboard struct {
	Players  []PublicBoard
	Gangs    []PublicBoard
	OptedIn  int // players who opted in
	YouIn    bool
	SignedIn bool
}

type lbPlayer struct {
	id                         int64
	name, copRank, medRank     string
	gangID                     int64
	wealth, houses, cars       int64
	playSecs, copSecs, medSecs int64
	cop, medic                 bool
	bounty                     int64
}

// PublicLeaderboard builds every board, top `limit` rows each. youID is
// the signed-in viewer (0 = signed out), highlighted where they appear.
func PublicLeaderboard(ctx context.Context, pool *pgxpool.Pool, youID int64, limit int) (Leaderboard, error) {
	lb := Leaderboard{SignedIn: youID != 0}
	rows, err := pool.Query(ctx, `
		SELECT p.id, COALESCE(NULLIF(p.display_name, ''), NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id),
		       COALESCE(p.civ_cash, 0) + COALESCE(p.cop_cash, 0) + COALESCE(p.medic_cash, 0)
		         + COALESCE((SELECT sum(balance) FROM bank_accounts b WHERE b.player_id = p.id), 0),
		       (SELECT count(*) FROM houses h WHERE h.owner_player_id = p.id),
		       (SELECT count(*) FROM vehicles v WHERE v.owner_player_id = p.id AND v.status <> 'destroyed'
		          AND COALESCE(v.faction_scope, 'civilian') = 'civilian'),
		       COALESCE(p.civ_playtime_seconds, 0) + COALESCE(p.cop_playtime_seconds, 0) + COALESCE(p.medic_playtime_seconds, 0),
		       COALESCE(p.cop_playtime_seconds, 0), COALESCE(p.medic_playtime_seconds, 0),
		       COALESCE(p.cop_level, 0) > 0, COALESCE(p.medic_level, 0) > 0,
		       COALESCE((SELECT name FROM faction_rank_names WHERE faction = 'police' AND level = p.cop_level), ''),
		       COALESCE((SELECT name FROM faction_rank_names WHERE faction = 'ems' AND level = p.medic_level), ''),
		       COALESCE(p.civ_bounty, 0),
		       COALESCE((SELECT gang_id FROM gang_members gm WHERE gm.player_id = p.id), 0)
		FROM players p WHERE p.leaderboard_opt_in`)
	if err != nil {
		return lb, err
	}
	var ps []lbPlayer
	for rows.Next() {
		var p lbPlayer
		if err := rows.Scan(&p.id, &p.name, &p.wealth, &p.houses, &p.cars, &p.playSecs, &p.copSecs, &p.medSecs,
			&p.cop, &p.medic, &p.copRank, &p.medRank, &p.bounty, &p.gangID); err != nil {
			rows.Close()
			return lb, err
		}
		ps = append(ps, p)
		if p.id == youID {
			lb.YouIn = true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return lb, err
	}
	lb.OptedIn = len(ps)

	var youGang int64
	if youID != 0 {
		_ = pool.QueryRow(ctx, `SELECT COALESCE((SELECT gang_id FROM gang_members WHERE player_id = $1), 0)`, youID).Scan(&youGang)
	}

	hours := func(secs int64) string { return fmt.Sprintf("%s h", commas(secs/3600)) }
	type board struct {
		key, title, sub string
		bands           bool // score is a band index: lower is better
		pick            func(p lbPlayer) (score int64, label, note string, ok bool)
	}
	boards := []board{
		{"wealth", "Richest players", "Cash and bank together, in bands. Same band = alphabetical.", true, func(p lbPlayer) (int64, string, string, bool) {
			i, l := Band(p.wealth)
			return int64(i), l, "", true
		}},
		{"playtime", "Most playtime", "Hours on the server across civilian, police and EMS.", false, func(p lbPlayer) (int64, string, string, bool) {
			return p.playSecs / 3600, hours(p.playSecs), "", p.playSecs >= 3600
		}},
		{"houses", "Most houses", "Houses owned right now.", false, func(p lbPlayer) (int64, string, string, bool) {
			return p.houses, plural(p.houses, "house", "houses"), "", p.houses > 0
		}},
		{"vehicles", "Most vehicles", "Personal vehicles. Faction vehicles don't count.", false, func(p lbPlayer) (int64, string, string, bool) {
			return p.cars, plural(p.cars, "vehicle", "vehicles"), "", p.cars > 0
		}},
		{"police", "Police service", "Hours on duty as police.", false, func(p lbPlayer) (int64, string, string, bool) {
			return p.copSecs / 3600, hours(p.copSecs), p.copRank, p.cop && p.copSecs >= 3600
		}},
		{"ems", "EMS service", "Hours on duty as EMS.", false, func(p lbPlayer) (int64, string, string, bool) {
			return p.medSecs / 3600, hours(p.medSecs), p.medRank, p.medic && p.medSecs >= 3600
		}},
		{"wanted", "Most wanted", "Outstanding bounties right now.", false, func(p lbPlayer) (int64, string, string, bool) {
			return p.bounty, "$" + commas(p.bounty), "", p.bounty > 0
		}},
	}
	for _, b := range boards {
		var es []entry
		notes := map[int64]string{}
		for _, p := range ps {
			score, label, note, ok := b.pick(p)
			if !ok {
				continue
			}
			es = append(es, entry{p.id, p.name, score, label})
			notes[p.id] = note
		}
		lb.Players = append(lb.Players, finish(b.key, b.title, b.sub, es, b.bands, youID, limit, notes))
	}

	// Gangs: every gang (they're public in game), bank as a band.
	grows, err := pool.Query(ctx, `
		SELECT g.id, g.name, g.tag, COALESCE(ga.balance, 0), (SELECT count(*) FROM gang_members m WHERE m.gang_id = g.id)
		FROM gangs g LEFT JOIN gang_accounts ga ON ga.gang_id = g.id`)
	if err != nil {
		return lb, err
	}
	var rich, big []entry
	tags := map[int64]string{}
	for grows.Next() {
		var id, bal, members int64
		var name, tag string
		if err := grows.Scan(&id, &name, &tag, &bal, &members); err != nil {
			grows.Close()
			return lb, err
		}
		i, l := Band(bal)
		rich = append(rich, entry{id, name, int64(i), l})
		big = append(big, entry{id, name, members, plural(members, "member", "members")})
		tags[id] = "[" + tag + "]"
	}
	grows.Close()
	if err := grows.Err(); err != nil {
		return lb, err
	}
	lb.Gangs = []PublicBoard{
		finish("gang_wealth", "Richest gangs", "Gang bank, in bands.", rich, true, youGang, limit, tags),
		finish("gang_size", "Largest gangs", "Members right now.", big, false, youGang, limit, tags),
	}
	return lb, nil
}

// finish sorts a board best-first (ties alphabetical), ranks it and adds
// the viewer's position.
func finish(key, title, sub string, es []entry, bands bool, youID int64, limit int, notes map[int64]string) PublicBoard {
	sort.Slice(es, func(i, j int) bool {
		if es[i].score != es[j].score {
			if bands {
				return es[i].score < es[j].score
			}
			return es[i].score > es[j].score
		}
		return strings.ToLower(es[i].name) < strings.ToLower(es[j].name)
	})
	rows, yours := rankRows(es, func(a, b entry) bool { return a.score == b.score }, youID, limit)
	for i := range rows {
		rows[i].Note = notes[es[i].id]
	}
	b := PublicBoard{Key: key, Title: title, Sub: sub, Rows: rows, Count: len(es)}
	if yours != "" {
		b.You = fmt.Sprintf("#%s of %d", strings.TrimSuffix(yours, "="), len(es))
	}
	return b
}

// commas formats 1234567 as "1,234,567".
func commas(n int64) string {
	s := fmt.Sprint(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		return "-" + s
	}
	return s
}
