package dashboard

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Region is the timezone days are bucketed in (the server's region), so
// "today" on the playtime chart matches the player's own calendar.
const Region = "Australia/Sydney"

// PlayDay is one day on the playtime chart.
type PlayDay struct {
	Day     time.Time
	Minutes int
	Pct     int // bar height, 0-100, relative to the busiest day shown
}

// Label is the day's short name for the chart ("Mon 3").
func (d PlayDay) Label() string { return d.Day.Format("Mon 2") }

// Hours is the day's playtime for the tooltip ("2 h 15 min").
func (d PlayDay) Hours() string {
	if d.Minutes < 60 {
		return fmt.Sprintf("%d min", d.Minutes)
	}
	if d.Minutes%60 == 0 {
		return fmt.Sprintf("%d h", d.Minutes/60)
	}
	return fmt.Sprintf("%d h %d min", d.Minutes/60, d.Minutes%60)
}

// Event is one line in the recent-activity feed: a play session or a bank
// transaction on one of the player's own accounts.
type Event struct {
	At     time.Time
	Kind   string // "session", "money-in", "money-out"
	Title  string
	Detail string
	Amount int64 // signed dollars, bank events only
}

// Activity is the dashboard's activity panel.
type Activity struct {
	Days       []PlayDay // oldest first, 14 days ending today
	TotalMins  int       // across Days
	ActiveDays int       // days in Days with any play
	Recent     []Event   // newest first
}

// TotalHours is the fortnight's playtime, rounded down.
func (a Activity) TotalHours() int { return a.TotalMins / 60 }

var factionName = map[string]string{"civ": "Civilian", "cop": "Police", "medic": "Medic", "civilian": "Civilian", "police": "Police"}

// LoadActivity builds the activity panel for a player.
func LoadActivity(ctx context.Context, pool *pgxpool.Pool, playerID int64) (Activity, error) {
	var a Activity
	loc, err := time.LoadLocation(Region)
	if err != nil {
		loc = time.UTC
	}
	// Each session is clipped to each day it overlaps, in the region's
	// days. An open session counts up to now. The FILTER matters: a day
	// with no session still has its LEFT JOIN row, and GREATEST ignores
	// the NULL, which would count the whole day.
	rows, err := pool.Query(ctx, `
		WITH days AS (
			SELECT d::date AS d FROM generate_series((now() AT TIME ZONE $2)::date - 13, (now() AT TIME ZONE $2)::date, interval '1 day') d
		)
		SELECT days.d::text, COALESCE(sum(EXTRACT(EPOCH FROM
		         LEAST(COALESCE(s.disconnected_at, now()), (days.d + 1)::timestamp AT TIME ZONE $2)
		       - GREATEST(s.connected_at, days.d::timestamp AT TIME ZONE $2))) FILTER (WHERE s.id IS NOT NULL), 0)::bigint
		FROM days
		LEFT JOIN player_sessions s ON s.player_id = $1
		     AND s.connected_at < (days.d + 1)::timestamp AT TIME ZONE $2
		     AND COALESCE(s.disconnected_at, now()) > days.d::timestamp AT TIME ZONE $2
		GROUP BY days.d ORDER BY days.d`, playerID, Region)
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
		pd := PlayDay{Day: t, Minutes: int(secs / 60)}
		a.TotalMins += pd.Minutes
		if pd.Minutes > 0 {
			a.ActiveDays++
		}
		if pd.Minutes > max {
			max = pd.Minutes
		}
		a.Days = append(a.Days, pd)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return a, err
	}
	for i := range a.Days {
		if max > 0 && a.Days[i].Minutes > 0 {
			a.Days[i].Pct = a.Days[i].Minutes * 100 / max
			if a.Days[i].Pct < 4 {
				a.Days[i].Pct = 4 // a few minutes still shows as a bar
			}
		}
	}

	const keep = 8
	rows, err = pool.Query(ctx, `
		SELECT connected_at, disconnected_at, COALESCE(faction, '') FROM player_sessions
		WHERE player_id = $1 ORDER BY connected_at DESC LIMIT $2`, playerID, keep)
	if err != nil {
		return a, err
	}
	for rows.Next() {
		var from time.Time
		var to *time.Time
		var faction string
		if err := rows.Scan(&from, &to, &faction); err != nil {
			rows.Close()
			return a, err
		}
		e := Event{At: from, Kind: "session", Title: "Played"}
		if n := factionName[faction]; n != "" {
			e.Title = "Played " + n
		}
		if to == nil {
			e.Title, e.Detail = "Online now", "connected "+from.In(loc).Format("15:04")
			if n := factionName[faction]; n != "" {
				e.Detail = "as " + n + " · " + e.Detail
			}
		} else {
			e.Detail = span(to.Sub(from))
		}
		a.Recent = append(a.Recent, e)
	}
	rows.Close()

	// Bank transactions on the player's own accounts. Staff adjustments
	// show without their memo (that's a staff note, not a receipt).
	rows, err = pool.Query(ctx, `
		SELECT t.created_at, t.type, t.amount, a.faction, CASE WHEN t.type = 'admin_adjustment' THEN '' ELSE COALESCE(t.memo, '') END
		FROM bank_transactions t JOIN bank_accounts a ON a.id = t.account_id
		WHERE a.player_id = $1 ORDER BY t.id DESC LIMIT $2`, playerID, keep)
	if err != nil {
		return a, err
	}
	for rows.Next() {
		var e Event
		var typ, faction, memo string
		if err := rows.Scan(&e.At, &typ, &e.Amount, &faction, &memo); err != nil {
			rows.Close()
			return a, err
		}
		// Withdrawals, outgoing transfers and purchases are stored as
		// positive or negative depending on the writer; the type decides.
		out := typ == "withdrawal" || typ == "transfer_out" || typ == "purchase" || (typ == "admin_adjustment" && e.Amount < 0)
		if e.Amount < 0 {
			e.Amount = -e.Amount
		}
		if out {
			e.Kind, e.Amount = "money-out", -e.Amount
		} else {
			e.Kind = "money-in"
		}
		e.Title = map[string]string{"deposit": "Deposit", "withdrawal": "Withdrawal", "transfer_in": "Money received", "transfer_out": "Money sent",
			"payout": "Paid", "purchase": "Purchase", "admin_adjustment": "Staff adjustment"}[typ]
		e.Detail = factionName[faction] + " bank"
		if memo != "" {
			e.Detail += " · " + memo
		}
		a.Recent = append(a.Recent, e)
	}
	rows.Close()

	sort.SliceStable(a.Recent, func(i, j int) bool { return a.Recent[i].At.After(a.Recent[j].At) })
	if len(a.Recent) > keep {
		a.Recent = a.Recent[:keep]
	}
	return a, rows.Err()
}

// Ago is the event's time for the feed ("5 min ago", "yesterday", "3 Nov").
func (e Event) Ago() string {
	d := time.Since(e.At)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	case d < 48*time.Hour:
		return "yesterday"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
	return e.At.Format("2 Jan")
}

// AbsAmount is the transaction's size without its sign.
func (e Event) AbsAmount() int64 {
	if e.Amount < 0 {
		return -e.Amount
	}
	return e.Amount
}

func span(d time.Duration) string {
	m := int(d.Minutes())
	if m < 60 {
		return fmt.Sprintf("%d min", m)
	}
	if m%60 == 0 {
		return fmt.Sprintf("%d h", m/60)
	}
	return fmt.Sprintf("%d h %d min", m/60, m%60)
}
