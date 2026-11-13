package factions

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// activityRegion buckets days in the server's region, like the player
// dashboard, so "today" matches command's own calendar.
const activityRegion = "Australia/Sydney"

// QuietAfter is how long a member can go without playing as the faction
// before the overview lists them.
const QuietAfter = 14 * 24 * time.Hour

// sessionFaction maps a command faction to player_sessions.faction.
var sessionFaction = map[string]string{"police": "cop", "ems": "medic"}

// DutyDay is one day on the on-duty chart.
type DutyDay struct {
	Day     time.Time
	Minutes int
	Pct     int // bar height relative to the busiest day shown
}

func (d DutyDay) Label() string { return d.Day.Format("Mon 2") }

func (d DutyDay) Hours() string {
	if d.Minutes < 60 {
		return fmt.Sprintf("%d min", d.Minutes)
	}
	return fmt.Sprintf("%.1f h", float64(d.Minutes)/60)
}

// QuietMember is a member who hasn't played as the faction for a while.
type QuietMember struct {
	ID       int64
	Name     string
	Rank     string
	LastDuty *time.Time // nil = never on record as this faction
}

// Since is how long ago they were last on duty, for the list.
func (q QuietMember) Since() string {
	if q.LastDuty == nil {
		return "never on record"
	}
	return fmt.Sprintf("%d days ago", int(time.Since(*q.LastDuty).Hours()/24))
}

// Activity is the overview's activity panel.
type Activity struct {
	Days        []DutyDay // 14 days ending today, oldest first
	TotalMins   int
	ActiveWeek  int // members on duty at least once in the last 7 days
	WeekMins    int
	Recruits    int // last 30 days, from the command log
	Promotions  int
	Demotions   int
	Removals    int // removals and discharges
	Quiet       []QuietMember
	QuietTotal  int
	MemberCount int
}

func (a Activity) WeekHours() int { return a.WeekMins / 60 }

// LoadActivity builds the activity panel for a faction ("police" / "ems").
// members is the current roster, highest rank first.
func LoadActivity(ctx context.Context, pool *pgxpool.Pool, faction string, members []Member) (Activity, error) {
	a := Activity{MemberCount: len(members)}
	sf := sessionFaction[faction]
	if sf == "" {
		return a, notAllowed("unknown faction")
	}
	loc, err := time.LoadLocation(activityRegion)
	if err != nil {
		loc = time.UTC
	}
	ids := make([]int64, len(members))
	for i, m := range members {
		ids[i] = m.ID
	}

	// On-duty minutes per day: current members' sessions as this faction,
	// clipped to each day. The FILTER keeps empty days at zero (the LEFT
	// JOIN still yields a row, and GREATEST would ignore its NULL).
	rows, err := pool.Query(ctx, `
		WITH days AS (
			SELECT d::date AS d FROM generate_series((now() AT TIME ZONE $3)::date - 13, (now() AT TIME ZONE $3)::date, interval '1 day') d
		)
		SELECT days.d::text, COALESCE(sum(EXTRACT(EPOCH FROM
		         LEAST(COALESCE(s.disconnected_at, now()), (days.d + 1)::timestamp AT TIME ZONE $3)
		       - GREATEST(s.connected_at, days.d::timestamp AT TIME ZONE $3))) FILTER (WHERE s.id IS NOT NULL), 0)::bigint
		FROM days
		LEFT JOIN player_sessions s ON s.faction = $1 AND s.player_id = ANY($2)
		     AND s.connected_at < (days.d + 1)::timestamp AT TIME ZONE $3
		     AND COALESCE(s.disconnected_at, now()) > days.d::timestamp AT TIME ZONE $3
		GROUP BY days.d ORDER BY days.d`, sf, ids, activityRegion)
	if err != nil {
		return a, err
	}
	max := 0
	for rows.Next() {
		var day string
		var secs int64
		if err := rows.Scan(&day, &secs); err != nil {
			rows.Close()
			return a, err
		}
		t, _ := time.ParseInLocation("2006-01-02", day, loc)
		dd := DutyDay{Day: t, Minutes: int(secs / 60)}
		a.TotalMins += dd.Minutes
		if dd.Minutes > max {
			max = dd.Minutes
		}
		a.Days = append(a.Days, dd)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return a, err
	}
	for i := range a.Days {
		if max > 0 && a.Days[i].Minutes > 0 {
			a.Days[i].Pct = a.Days[i].Minutes * 100 / max
			if a.Days[i].Pct < 4 {
				a.Days[i].Pct = 4
			}
		}
	}

	if err := pool.QueryRow(ctx, `
		SELECT count(DISTINCT player_id),
		       COALESCE(sum(EXTRACT(EPOCH FROM COALESCE(disconnected_at, now()) - GREATEST(connected_at, now() - interval '7 days'))), 0)::bigint / 60
		FROM player_sessions
		WHERE faction = $1 AND player_id = ANY($2) AND COALESCE(disconnected_at, now()) > now() - interval '7 days'`,
		sf, ids).Scan(&a.ActiveWeek, &a.WeekMins); err != nil {
		return a, err
	}

	if err := pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE kind = 'recruit'), count(*) FILTER (WHERE kind = 'promote'),
		       count(*) FILTER (WHERE kind = 'demote'), count(*) FILTER (WHERE kind IN ('remove', 'discharge'))
		FROM faction_log WHERE faction = $1 AND created_at > now() - interval '30 days'`,
		faction).Scan(&a.Recruits, &a.Promotions, &a.Demotions, &a.Removals); err != nil {
		return a, err
	}

	// Last time on duty per member.
	last := map[int64]time.Time{}
	rows, err = pool.Query(ctx, `
		SELECT player_id, max(COALESCE(disconnected_at, now())) FROM player_sessions
		WHERE faction = $1 AND player_id = ANY($2) GROUP BY player_id`, sf, ids)
	if err != nil {
		return a, err
	}
	for rows.Next() {
		var id int64
		var t time.Time
		if err := rows.Scan(&id, &t); err != nil {
			rows.Close()
			return a, err
		}
		last[id] = t
	}
	rows.Close()
	for _, m := range members {
		t, ok := last[m.ID]
		if ok && time.Since(t) < QuietAfter {
			continue
		}
		a.QuietTotal++
		q := QuietMember{ID: m.ID, Name: m.Name, Rank: m.Rank.Label()}
		if ok {
			tt := t
			q.LastDuty = &tt
		}
		a.Quiet = append(a.Quiet, q)
	}
	// Longest quiet first (never-on-record before anyone with a date).
	for i := 1; i < len(a.Quiet); i++ {
		for j := i; j > 0 && quieter(a.Quiet[j], a.Quiet[j-1]); j-- {
			a.Quiet[j], a.Quiet[j-1] = a.Quiet[j-1], a.Quiet[j]
		}
	}
	if len(a.Quiet) > 8 {
		a.Quiet = a.Quiet[:8]
	}
	return a, rows.Err()
}

func quieter(x, y QuietMember) bool {
	if x.LastDuty == nil || y.LastDuty == nil {
		return x.LastDuty == nil && y.LastDuty != nil
	}
	return x.LastDuty.Before(*y.LastDuty)
}
