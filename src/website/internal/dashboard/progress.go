package dashboard

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BankDay is one day's closing balance across all of a player's bank
// accounts.
type BankDay struct {
	Day     time.Time
	Balance int64
}

// BankHistory is the player's total bank balance at the end of each of the
// last `days` days (in Region days), oldest first, and whether anything
// moved in that time (a flat line isn't worth a chart).
//
// Balances are walked back from today's: money only ever moves through
// bank_transactions, and each row's amount is the signed change it made
// (the balance trigger applies it as such), so the balance at the end of a
// day is the current balance minus every change made after it.
func BankHistory(ctx context.Context, pool *pgxpool.Pool, playerID int64, days int) ([]BankDay, bool, error) {
	loc, err := time.LoadLocation(Region)
	if err != nil {
		loc = time.UTC
	}
	rows, err := pool.Query(ctx, `
		WITH days AS (
			SELECT d::date AS d FROM generate_series((now() AT TIME ZONE $2)::date - ($3::int - 1), (now() AT TIME ZONE $2)::date, interval '1 day') d
		), acct AS (
			SELECT id, balance FROM bank_accounts WHERE player_id = $1
		)
		SELECT days.d::text,
		       COALESCE((SELECT sum(balance) FROM acct), 0)
		       - COALESCE((SELECT sum(t.amount) FROM bank_transactions t
		                   WHERE t.account_id IN (SELECT id FROM acct)
		                     AND t.created_at >= (days.d + 1)::timestamp AT TIME ZONE $2), 0),
		       EXISTS (SELECT 1 FROM bank_transactions t
		               WHERE t.account_id IN (SELECT id FROM acct)
		                 AND t.created_at >= days.d::timestamp AT TIME ZONE $2
		                 AND t.created_at < (days.d + 1)::timestamp AT TIME ZONE $2)
		FROM days ORDER BY days.d`, playerID, Region, days)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var out []BankDay
	moved := false
	for rows.Next() {
		var day string
		var bd BankDay
		var any bool
		if err := rows.Scan(&day, &bd.Balance, &any); err != nil {
			return nil, false, err
		}
		bd.Day, _ = time.ParseInLocation("2006-01-02", day, loc)
		moved = moved || any
		out = append(out, bd)
	}
	return out, moved, rows.Err()
}

// Milestone is one entry on the dashboard's milestones card.
type Milestone struct {
	Title    string
	Detail   string // what it takes, or how far along
	Done     bool
	Progress int // 0-100 for the ones with a number to reach
}

// Milestones are computed from what the dashboard already knows -- nothing
// stored, so they can't drift from the player's real state.
func Milestones(o Overview, inGang bool) []Milestone {
	pct := func(have, want int64) int {
		if have >= want {
			return 100
		}
		if have <= 0 {
			return 0
		}
		return int(have * 100 / want)
	}
	count := func(have, want int64, unit string) string {
		if have >= want {
			return fmt.Sprintf("%d %s", have, unit)
		}
		return fmt.Sprintf("%d of %d %s", have, want, unit)
	}
	var police, ems bool
	for _, s := range o.Sides {
		if s.Key == "police" && s.Member {
			police = true
		}
		if s.Key == "ems" && s.Member {
			ems = true
		}
	}
	worth := o.NetWorth()
	ms := []Milestone{
		{Title: "First steps", Detail: "Play your first session", Done: o.PlaytimeHours > 0 || o.WeekHours > 0},
		{Title: "Regular", Detail: count(o.PlaytimeHours, 10, "hours"), Done: o.PlaytimeHours >= 10, Progress: pct(o.PlaytimeHours, 10)},
		{Title: "Veteran", Detail: count(o.PlaytimeHours, 100, "hours"), Done: o.PlaytimeHours >= 100, Progress: pct(o.PlaytimeHours, 100)},
		{Title: "Wheels", Detail: "Own a vehicle", Done: len(o.Vehicles) > 0},
		{Title: "Homeowner", Detail: "Buy a house", Done: len(o.Houses) > 0},
		{Title: "Licensed", Detail: count(int64(len(o.Licences)), 3, "licences"), Done: len(o.Licences) >= 3, Progress: pct(int64(len(o.Licences)), 3)},
		{Title: "Six figures", Detail: "Net worth of $100,000", Done: worth >= 100_000, Progress: pct(worth, 100_000)},
		{Title: "Millionaire", Detail: "Net worth of $1,000,000", Done: worth >= 1_000_000, Progress: pct(worth, 1_000_000)},
		{Title: "Crew", Detail: "Join a gang", Done: inGang},
		{Title: "Sworn in", Detail: "Join the Police or EMS", Done: police || ems},
	}
	for i := range ms {
		if ms[i].Done {
			ms[i].Progress = 100
		}
	}
	return ms
}

// MilestonesDone counts the finished ones.
func MilestonesDone(ms []Milestone) int {
	n := 0
	for _, m := range ms {
		if m.Done {
			n++
		}
	}
	return n
}
