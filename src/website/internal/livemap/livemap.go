// Package livemap is the admin panel's Live Map: where the players in game
// are on Altis, from the positions the game server already saves for each
// online player about once a minute (players.<faction>_position, written
// by ALife_fnc_savePlayer from the autosave loop). Places on the map come
// from the mission's own markers (mission.sqm): the town spawn points and
// every labelled marker (mines, processors, shops, ...).
package livemap

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WorldSize is Altis's size in metres (square, origin bottom-left).
const WorldSize = 30720

// Point is a player in game.
type Point struct {
	PlayerID int64   `json:"id"`
	Name     string  `json:"name"`
	Faction  string  `json:"faction"` // civ / cop / medic / ""
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Watched  bool    `json:"watched"`
	Wanted   bool    `json:"wanted"`
	Flags    int     `json:"flags"` // open anti-cheat flags
	Gang     string  `json:"gang,omitempty"`
}

// Points is every online player with a saved position for the faction
// they're playing (a player who just joined may not have one yet).
func Points(ctx context.Context, pool *pgxpool.Pool) ([]Point, int, error) {
	rows, err := pool.Query(ctx, `
		WITH s AS (
		  SELECT DISTINCT ON (player_id) player_id, faction FROM player_sessions
		  WHERE disconnected_at IS NULL ORDER BY player_id, connected_at DESC)
		SELECT p.id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id), COALESCE(s.faction, ''),
		       (CASE s.faction WHEN 'cop' THEN p.cop_position WHEN 'medic' THEN p.medic_position ELSE p.civ_position END)::text,
		       EXISTS (SELECT 1 FROM player_watchlist w WHERE w.player_id = p.id),
		       EXISTS (SELECT 1 FROM wanted_crimes c WHERE c.player_id = p.id AND c.cleared_at IS NULL),
		       (SELECT count(*) FROM anti_cheat_flags f WHERE f.player_id = p.id AND f.resolution IS NULL)::int,
		       COALESCE((SELECT g.name FROM gang_members gm JOIN gangs g ON g.id = gm.gang_id WHERE gm.player_id = p.id), '')
		FROM s JOIN players p ON p.id = s.player_id`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Point
	online := 0
	for rows.Next() {
		var pt Point
		var pos string
		if err := rows.Scan(&pt.PlayerID, &pt.Name, &pt.Faction, &pos, &pt.Watched, &pt.Wanted, &pt.Flags, &pt.Gang); err != nil {
			return nil, 0, err
		}
		online++
		x, y, ok := ParsePosition(pos)
		if !ok {
			continue
		}
		pt.X, pt.Y = x, y
		out = append(out, pt)
	}
	return out, online, rows.Err()
}

// ParsePosition reads a saved position: [["x",n],["y",n],["z",n]] (pairs,
// as ALife_fnc_savePlayer writes them), or a plain [x, y, z].
func ParsePosition(s string) (float64, float64, bool) {
	var raw []any
	if json.Unmarshal([]byte(s), &raw) != nil || len(raw) < 2 {
		return 0, 0, false
	}
	num := func(v any) (float64, bool) { f, ok := v.(float64); return f, ok }
	var x, y float64
	var hx, hy bool
	if _, plain := raw[0].(float64); plain {
		x, hx = num(raw[0])
		y, hy = num(raw[1])
	} else {
		for _, e := range raw {
			pair, ok := e.([]any)
			if !ok || len(pair) != 2 {
				continue
			}
			k, _ := pair[0].(string)
			switch strings.ToLower(k) {
			case "x":
				x, hx = num(pair[1])
			case "y":
				y, hy = num(pair[1])
			}
		}
	}
	if !hx || !hy || x < 0 || y < 0 || x > WorldSize || y > WorldSize || (x == 0 && y == 0) {
		return 0, 0, false
	}
	return x, y, true
}

// Place is a labelled spot from the mission.
type Place struct {
	Name string  `json:"name"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Town bool    `json:"town"`
}

var (
	placesOnce sync.Once
	places     []Place
)

// Places reads the mission's markers once (nil if the mission isn't there).
func Places(missionSQM string) []Place {
	placesOnce.Do(func() {
		b, err := os.ReadFile(missionSQM)
		if err == nil {
			places = ParseMarkers(string(b))
		}
	})
	return places
}

var (
	reItem = regexp.MustCompile(`(?s)dataType="Marker";(.*?)\n\t\t\};`)
	rePos  = regexp.MustCompile(`position\[\]=\{([-0-9.e]+),([-0-9.e]+),([-0-9.e]+)\}`)
	reName = regexp.MustCompile(`\bname="([^"]*)"`)
	reText = regexp.MustCompile(`\btext="([^"]*)"`)
)

// ParseMarkers picks the towns (spawn_civ_<town> markers) and every marker
// with a label out of mission.sqm. Arma's marker position is {x, height, y}.
func ParseMarkers(sqm string) []Place {
	seen := map[string]bool{}
	var out []Place
	for _, m := range reItem.FindAllStringSubmatch(sqm, -1) {
		body := m[1]
		pm := rePos.FindStringSubmatch(body)
		if pm == nil {
			continue
		}
		x, _ := strconv.ParseFloat(pm[1], 64)
		y, _ := strconv.ParseFloat(pm[3], 64)
		name := ""
		if nm := reName.FindStringSubmatch(body); nm != nil {
			name = nm[1]
		}
		label, town := "", false
		if t := reText.FindStringSubmatch(body); t != nil && strings.TrimSpace(t[1]) != "" {
			label = strings.TrimSpace(t[1])
		} else if rest, ok := strings.CutPrefix(name, "spawn_civ_"); ok && rest != "" {
			label, town = strings.ToUpper(rest[:1])+strings.ReplaceAll(rest[1:], "_", " "), true
		}
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, Place{Name: label, X: x, Y: y, Town: town})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
