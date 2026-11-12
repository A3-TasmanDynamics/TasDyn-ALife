// Package online is the admin panel's "Online now" page: who's in game
// right now (open player_sessions), as which faction, for how long, with
// their money, bounty and open anti-cheat flags, plus the player count over
// the last day worked out from the session rows.
package online

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const playerName = `COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id)`

// Player is one player in game.
type Player struct {
	PlayerID  int64
	Name, UID string
	Faction   string // civ / cop / medic; "" when the session didn't record it
	Since     time.Time
	Level     int   // cop / medic level (0 for civilians)
	Cash      int64 // the faction they're playing (all three when unknown)
	Bank      int64
	Bounty    int64 // outstanding
	JailSecs  int
	OpenFlags int
	GangID    int64
	Gang      string
	Alive     bool
}

// For is how long they've been on.
func (p Player) For(now time.Time) time.Duration { return now.Sub(p.Since) }

// Left is a session that ended recently.
type Left struct {
	PlayerID int64
	Name     string
	Faction  string
	At       time.Time
	Length   time.Duration
}

// Summary is the numbers at the top of the page.
type Summary struct {
	Online, Civ, Cop, Medic, Unknown int
	Peak24                           int           // most at once in the last 24 h
	PeakAt                           time.Time     // when (the first time it was reached)
	Unique24                         int           // different players in the last 24 h
	AvgSession                       time.Duration // finished sessions in the last 24 h
}

// Now lists everyone with an open session, longest on first.
func Now(ctx context.Context, pool *pgxpool.Pool) ([]Player, error) {
	rows, err := pool.Query(ctx, `
		WITH s AS (
		  SELECT DISTINCT ON (player_id) player_id, faction, connected_at
		  FROM player_sessions WHERE disconnected_at IS NULL
		  ORDER BY player_id, connected_at DESC)
		SELECT p.id, `+playerName+`, p.uid, COALESCE(s.faction, ''), s.connected_at,
		       CASE s.faction WHEN 'cop' THEN p.cop_level WHEN 'medic' THEN p.medic_level ELSE 0 END,
		       CASE s.faction WHEN 'civ' THEN p.civ_cash WHEN 'cop' THEN p.cop_cash WHEN 'medic' THEN p.medic_cash
		                      ELSE p.civ_cash + p.cop_cash + p.medic_cash END,
		       COALESCE((SELECT sum(b.balance) FROM bank_accounts b WHERE b.player_id = p.id
		                   AND (s.faction IS NULL OR b.faction = CASE s.faction WHEN 'civ' THEN 'civilian' WHEN 'cop' THEN 'police' ELSE 'medic' END)), 0)::bigint,
		       COALESCE((SELECT sum(w.bounty_amount) FROM wanted_crimes w WHERE w.player_id = p.id AND w.cleared_at IS NULL), 0)::bigint,
		       p.civ_jail_seconds,
		       (SELECT count(*) FROM anti_cheat_flags f WHERE f.player_id = p.id AND f.resolution IS NULL)::int,
		       COALESCE(g.id, 0), COALESCE(g.name, ''),
		       CASE s.faction WHEN 'cop' THEN p.cop_alive WHEN 'medic' THEN p.medic_alive ELSE p.civ_alive END
		FROM s JOIN players p ON p.id = s.player_id
		LEFT JOIN gang_members gm ON gm.player_id = p.id
		LEFT JOIN gangs g ON g.id = gm.gang_id
		ORDER BY s.connected_at, p.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Player
	for rows.Next() {
		var p Player
		if err := rows.Scan(&p.PlayerID, &p.Name, &p.UID, &p.Faction, &p.Since, &p.Level, &p.Cash, &p.Bank,
			&p.Bounty, &p.JailSecs, &p.OpenFlags, &p.GangID, &p.Gang, &p.Alive); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// RecentlyLeft is the last n players whose session ended in the past
// `within` (and who haven't come back), their latest session each.
func RecentlyLeft(ctx context.Context, pool *pgxpool.Pool, within time.Duration, n int) ([]Left, error) {
	rows, err := pool.Query(ctx, `
		SELECT p.id, `+playerName+`, COALESCE(s.faction, ''), s.disconnected_at, EXTRACT(EPOCH FROM s.disconnected_at - s.connected_at)::float8
		FROM (SELECT DISTINCT ON (player_id) * FROM player_sessions
		      WHERE disconnected_at > now() - make_interval(secs => $1)
		      ORDER BY player_id, disconnected_at DESC) s
		JOIN players p ON p.id = s.player_id
		WHERE NOT EXISTS (SELECT 1 FROM player_sessions o WHERE o.player_id = s.player_id AND o.disconnected_at IS NULL)
		ORDER BY s.disconnected_at DESC LIMIT $2`, within.Seconds(), n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Left
	for rows.Next() {
		var l Left
		var secs float64
		if err := rows.Scan(&l.PlayerID, &l.Name, &l.Faction, &l.At, &secs); err != nil {
			return nil, err
		}
		l.Length = time.Duration(secs) * time.Second
		out = append(out, l)
	}
	return out, rows.Err()
}

// History is how many players were on at each step over the last `span`,
// oldest first, ending now.
func History(ctx context.Context, pool *pgxpool.Pool, span, step time.Duration) ([]time.Time, []float64, error) {
	// Only the sessions that overlap the window (idx_player_sessions_disconnected),
	// read once, then counted at each step.
	rows, err := pool.Query(ctx, `
		WITH s AS MATERIALIZED (
		  SELECT player_id, connected_at, disconnected_at FROM player_sessions
		  WHERE disconnected_at IS NULL OR disconnected_at > now() - make_interval(secs => $1))
		SELECT t, (SELECT count(DISTINCT s.player_id) FROM s
		           WHERE s.connected_at <= t AND (s.disconnected_at IS NULL OR s.disconnected_at > t))
		FROM generate_series(now() - make_interval(secs => $1), now(), make_interval(secs => $2)) t
		ORDER BY t`, span.Seconds(), step.Seconds())
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var times []time.Time
	var counts []float64
	for rows.Next() {
		var t time.Time
		var n int64
		if err := rows.Scan(&t, &n); err != nil {
			return nil, nil, err
		}
		times = append(times, t)
		counts = append(counts, float64(n))
	}
	return times, counts, rows.Err()
}

// Summarize counts the players by faction and adds the last day's numbers
// (peak from the history points).
func Summarize(ctx context.Context, pool *pgxpool.Pool, players []Player, times []time.Time, counts []float64) (Summary, error) {
	s := Summary{Online: len(players)}
	for _, p := range players {
		switch p.Faction {
		case "civ":
			s.Civ++
		case "cop":
			s.Cop++
		case "medic":
			s.Medic++
		default:
			s.Unknown++
		}
	}
	for i, c := range counts {
		if int(c) > s.Peak24 {
			s.Peak24, s.PeakAt = int(c), times[i]
		}
	}
	var avgSecs float64
	err := pool.QueryRow(ctx, `
		SELECT (SELECT count(DISTINCT player_id) FROM player_sessions
		        WHERE disconnected_at IS NULL OR disconnected_at > now() - interval '24 hours'),
		       COALESCE((SELECT avg(EXTRACT(EPOCH FROM disconnected_at - connected_at)) FROM player_sessions
		        WHERE disconnected_at > now() - interval '24 hours'), 0)::float8`).Scan(&s.Unique24, &avgSecs)
	s.AvgSession = time.Duration(avgSecs) * time.Second
	return s, err
}
