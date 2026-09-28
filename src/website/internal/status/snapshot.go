package status

import (
	"context"
	"fmt"
	"time"
)

// Days is how many daily bars the page shows per component.
const Days = 60

// staleAfter: a latest check older than this means the checker itself
// isn't running, so the current state is unknown rather than "up".
const staleAfter = 3 * Interval

type DayBar struct {
	Class string // "up", "warn", "down", "none"
	Title string // hover text
}

type ComponentView struct {
	Name        string
	Description string
	State       string // "up", "down", "unknown", "unmonitored"
	StateText   string
	Detail      string
	Uptime      string // over the shown window, "" when no data
	Days        []DayBar
}

type Snapshot struct {
	State      string // "up", "down", "unknown"
	Headline   string
	Uptime     string
	Components []ComponentView
	CheckedAt  string
	Since      string // when monitoring began, "" if never
}

type dayCount struct{ total, ok int }

// Snapshot builds the status page from recorded checks. Days are bucketed
// in loc (the server's region), so "today" matches players' own calendar.
func (m *Monitor) Snapshot(ctx context.Context, loc *time.Location) (Snapshot, error) {
	now := time.Now().In(loc)
	snap := Snapshot{}

	var first *time.Time
	if err := m.Pool.QueryRow(ctx, `SELECT min(checked_at) FROM status_checks`).Scan(&first); err != nil {
		return snap, err
	}
	if first != nil {
		snap.Since = first.In(loc).Format("2 January 2006")
	}

	type latest struct {
		ok        bool
		detail    string
		checkedAt time.Time
	}
	latestBy := map[string]latest{}
	rows, err := m.Pool.Query(ctx, `
		SELECT DISTINCT ON (component) component, ok, COALESCE(detail, ''), checked_at
		FROM status_checks ORDER BY component, checked_at DESC
	`)
	if err != nil {
		return snap, err
	}
	for rows.Next() {
		var key string
		var l latest
		if err := rows.Scan(&key, &l.ok, &l.detail, &l.checkedAt); err == nil {
			latestBy[key] = l
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return snap, err
	}

	windowStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -(Days - 1))
	daily := map[string]map[string]dayCount{}
	rows, err = m.Pool.Query(ctx, `
		SELECT component, to_char(checked_at AT TIME ZONE $1, 'YYYY-MM-DD'), count(*), count(*) FILTER (WHERE ok)
		FROM status_checks WHERE checked_at >= $2
		GROUP BY 1, 2
	`, loc.String(), windowStart)
	if err != nil {
		return snap, err
	}
	for rows.Next() {
		var key, day string
		var c dayCount
		if err := rows.Scan(&key, &day, &c.total, &c.ok); err == nil {
			if daily[key] == nil {
				daily[key] = map[string]dayCount{}
			}
			daily[key][day] = c
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return snap, err
	}

	var newest time.Time
	anyDown, anyUnknown := false, false
	var uptimeSum float64
	uptimeN := 0

	for _, c := range m.Components {
		v := ComponentView{Name: c.Name, Description: c.Description}
		l, has := latestBy[c.Key]
		switch {
		case !has || now.Sub(l.checkedAt) > staleAfter:
			v.State, v.StateText = "unknown", "No recent data"
			anyUnknown = true
		case l.ok:
			v.State, v.StateText = "up", "Operational"
		default:
			v.State, v.StateText = "down", "Down"
			anyDown = true
		}
		if has {
			v.Detail = l.detail
			if l.checkedAt.After(newest) {
				newest = l.checkedAt
			}
		}

		var winOK, winTotal float64
		for i := 0; i < Days; i++ {
			dayStart := windowStart.AddDate(0, 0, i)
			dayEnd := dayStart.AddDate(0, 0, 1)
			key := dayStart.Format("2006-01-02")
			label := dayStart.Format("2 Jan 2006")
			cnt := daily[c.Key][key]

			// Denominator: recorded checks, except for the website's own
			// check, where a missing minute *is* the outage -- expected
			// checks are the minutes monitoring was running that day.
			denom := float64(cnt.total)
			if c.selfCheck && first != nil {
				from, to := dayStart, dayEnd
				if first.After(from) {
					from = first.In(loc)
				}
				if now.Before(to) {
					to = now
				}
				if expected := to.Sub(from) / Interval; expected > 0 && float64(expected) > denom {
					denom = float64(expected)
				}
			}
			if cnt.total == 0 && (first == nil || dayEnd.Before(*first) || !c.selfCheck) {
				v.Days = append(v.Days, DayBar{Class: "none", Title: label + " · No data"})
				continue
			}
			pct := 100.0
			if denom > 0 {
				pct = float64(cnt.ok) / denom * 100
			}
			if pct > 100 {
				pct = 100
			}
			winOK += float64(cnt.ok)
			winTotal += denom
			class := "up"
			switch {
			case pct < 95:
				class = "down"
			case pct < 99.9:
				class = "warn"
			}
			v.Days = append(v.Days, DayBar{Class: class, Title: fmt.Sprintf("%s · %.2f%% uptime", label, pct)})
		}
		if winTotal > 0 {
			u := winOK / winTotal * 100
			v.Uptime = fmt.Sprintf("%.2f%%", u)
			uptimeSum += u
			uptimeN++
		}
		snap.Components = append(snap.Components, v)
	}

	for _, c := range m.Unmonitored {
		snap.Components = append(snap.Components, ComponentView{
			Name: c.Name, Description: c.Description,
			State: "unmonitored", StateText: "Not monitored yet",
		})
	}

	switch {
	case anyDown:
		snap.State, snap.Headline = "down", "Some systems are down"
	case anyUnknown:
		snap.State, snap.Headline = "unknown", "Status unknown"
	default:
		snap.State, snap.Headline = "up", "All systems operational"
	}
	if uptimeN > 0 {
		snap.Uptime = fmt.Sprintf("%.2f%% uptime", uptimeSum/float64(uptimeN))
	}
	if !newest.IsZero() {
		snap.CheckedAt = newest.In(loc).Format("15:04 MST, 2 Jan")
	}
	return snap, nil
}
