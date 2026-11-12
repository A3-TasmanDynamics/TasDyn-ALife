// Package staffreport is the admin panel's Staff Activity page: who on the
// staff team is doing what (Staff Log actions, flag reviews, case work,
// ticket replies, bans), how quickly flags, tickets and cases get handled,
// and how that's trending week by week. Everything comes from timestamps
// the panel already records; nothing new is tracked.
package staffreport

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Member is one staff member's work in the window.
type Member struct {
	PlayerID      int64
	Name, Rank    string
	Level         int
	Actions       int // Staff Log entries (any source)
	FlagsReviewed int
	CasesOpened   int // as lead
	CasesClosed   int // as lead
	CaseEntries   int // notes / corrections / points ... written
	TicketReplies int // to players
	TicketNotes   int // internal notes
	Bans          int
	LastActive    *time.Time
}

// Total is a rough "work done" count for sorting.
func (m Member) Total() int {
	return m.Actions + m.FlagsReviewed + m.CaseEntries + m.TicketReplies + m.TicketNotes
}

// Team is how quickly things get handled.
type Team struct {
	FlagsReviewed     int
	FlagReviewMedian  time.Duration // raised -> reviewed
	TicketsOpened     int
	TicketsAnswered   int
	TicketFirstMedian time.Duration // opened -> first staff reply
	CasesClosed       int
	CaseCloseMedian   time.Duration // opened -> closed
	OpenFlags         int
	OldestOpenFlag    *time.Time
	UnansweredTickets int // open, no staff reply yet
	OldestUnanswered  *time.Time
}

// Week is one week's totals, oldest first.
type Week struct {
	Start         time.Time
	FlagsReviewed int
	TicketReplies int
	CaseEntries   int
	Actions       int
}

// Members is every current staff member's work since `since`, busiest
// first; staff who did nothing are listed too (last).
func Members(ctx context.Context, pool *pgxpool.Pool, since time.Time) ([]Member, error) {
	rows, err := pool.Query(ctx, `
		WITH staff AS (
		  SELECT p.id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id) AS name, sr.display_name AS rank, sr.level
		  FROM players p JOIN staff_ranks sr ON sr.id = p.staff_rank_id),
		a AS (SELECT staff_player_id AS id, count(*) AS n, max(created_at) AS last FROM staff_log WHERE created_at > $1 GROUP BY 1),
		f AS (SELECT reviewed_by AS id, count(*) AS n, max(reviewed_at) AS last FROM anti_cheat_flags WHERE reviewed_at > $1 GROUP BY 1),
		co AS (SELECT lead_staff_id AS id, count(*) FILTER (WHERE created_at > $1) AS opened, count(*) FILTER (WHERE closed_at > $1) AS closed
		       FROM staff_cases WHERE created_at > $1 OR closed_at > $1 GROUP BY 1),
		ce AS (SELECT author_id AS id, count(*) AS n, max(created_at) AS last FROM staff_case_entries WHERE created_at > $1 GROUP BY 1),
		tm AS (SELECT m.author_player_id AS id, count(*) FILTER (WHERE NOT m.internal) AS replies, count(*) FILTER (WHERE m.internal) AS notes, max(m.created_at) AS last
		       FROM support_ticket_messages m JOIN support_tickets t ON t.id = m.ticket_id
		       WHERE m.created_at > $1 AND m.author_player_id IS DISTINCT FROM t.player_id GROUP BY 1),
		b AS (SELECT banned_by AS id, count(*) AS n FROM banlist WHERE created_at > $1 GROUP BY 1)
		SELECT s.id, s.name, s.rank, s.level,
		       COALESCE(a.n, 0)::int, COALESCE(f.n, 0)::int, COALESCE(co.opened, 0)::int, COALESCE(co.closed, 0)::int,
		       COALESCE(ce.n, 0)::int, COALESCE(tm.replies, 0)::int, COALESCE(tm.notes, 0)::int, COALESCE(b.n, 0)::int,
		       GREATEST(a.last, f.last, ce.last, tm.last)
		FROM staff s
		LEFT JOIN a ON a.id = s.id LEFT JOIN f ON f.id = s.id LEFT JOIN co ON co.id = s.id
		LEFT JOIN ce ON ce.id = s.id LEFT JOIN tm ON tm.id = s.id LEFT JOIN b ON b.id = s.id`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.PlayerID, &m.Name, &m.Rank, &m.Level, &m.Actions, &m.FlagsReviewed, &m.CasesOpened, &m.CasesClosed,
			&m.CaseEntries, &m.TicketReplies, &m.TicketNotes, &m.Bans, &m.LastActive); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sortMembers(out)
	return out, nil
}

func sortMembers(ms []Member) {
	// Busiest first; ties by rank level (higher first), then name.
	for i := 1; i < len(ms); i++ {
		for j := i; j > 0 && less(ms[j], ms[j-1]); j-- {
			ms[j], ms[j-1] = ms[j-1], ms[j]
		}
	}
}

func less(a, b Member) bool {
	if a.Total() != b.Total() {
		return a.Total() > b.Total()
	}
	if a.Level != b.Level {
		return a.Level > b.Level
	}
	return a.Name < b.Name
}

// TeamStats is how quickly things got handled since `since`, and what's
// waiting now.
func TeamStats(ctx context.Context, pool *pgxpool.Pool, since time.Time) (Team, error) {
	var t Team
	var flagSecs, ticketSecs, caseSecs *float64
	err := pool.QueryRow(ctx, `
		WITH tk AS (
		  SELECT t.id, t.created_at,
		         (SELECT min(m.created_at) FROM support_ticket_messages m
		          WHERE m.ticket_id = t.id AND NOT m.internal AND m.author_player_id IS DISTINCT FROM t.player_id AND m.author_player_id IS NOT NULL) AS first_reply
		  FROM support_tickets t WHERE t.created_at > $1)
		SELECT (SELECT count(*) FROM anti_cheat_flags WHERE reviewed_at > $1),
		       (SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM reviewed_at - created_at)) FROM anti_cheat_flags WHERE reviewed_at > $1),
		       (SELECT count(*) FROM tk), (SELECT count(*) FROM tk WHERE first_reply IS NOT NULL),
		       (SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM first_reply - created_at)) FROM tk WHERE first_reply IS NOT NULL),
		       (SELECT count(*) FROM staff_cases WHERE closed_at > $1),
		       (SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM closed_at - created_at)) FROM staff_cases WHERE closed_at > $1),
		       (SELECT count(*) FROM anti_cheat_flags WHERE resolution IS NULL),
		       (SELECT min(created_at) FROM anti_cheat_flags WHERE resolution IS NULL)`, since).Scan(
		&t.FlagsReviewed, &flagSecs, &t.TicketsOpened, &t.TicketsAnswered, &ticketSecs, &t.CasesClosed, &caseSecs, &t.OpenFlags, &t.OldestOpenFlag)
	if err != nil {
		return t, err
	}
	dur := func(p *float64) time.Duration {
		if p == nil {
			return 0
		}
		return time.Duration(*p) * time.Second
	}
	t.FlagReviewMedian, t.TicketFirstMedian, t.CaseCloseMedian = dur(flagSecs), dur(ticketSecs), dur(caseSecs)
	err = pool.QueryRow(ctx, `
		SELECT count(*), min(t.created_at) FROM support_tickets t
		WHERE t.status <> 'closed' AND NOT EXISTS (
		  SELECT 1 FROM support_ticket_messages m WHERE m.ticket_id = t.id AND NOT m.internal
		    AND m.author_player_id IS NOT NULL AND m.author_player_id IS DISTINCT FROM t.player_id)`).Scan(&t.UnansweredTickets, &t.OldestUnanswered)
	return t, err
}

// Weekly is the last n weeks (Monday-start, local time), oldest first.
func Weekly(ctx context.Context, pool *pgxpool.Pool, n int) ([]Week, error) {
	rows, err := pool.Query(ctx, `
		WITH w AS (SELECT generate_series(date_trunc('week', now()) - make_interval(weeks => $1 - 1), date_trunc('week', now()), interval '1 week') AS start)
		SELECT w.start,
		       (SELECT count(*) FROM anti_cheat_flags f WHERE f.reviewed_at >= w.start AND f.reviewed_at < w.start + interval '1 week')::int,
		       (SELECT count(*) FROM support_ticket_messages m JOIN support_tickets t ON t.id = m.ticket_id
		         WHERE m.created_at >= w.start AND m.created_at < w.start + interval '1 week' AND NOT m.internal
		           AND m.author_player_id IS NOT NULL AND m.author_player_id IS DISTINCT FROM t.player_id)::int,
		       (SELECT count(*) FROM staff_case_entries e WHERE e.created_at >= w.start AND e.created_at < w.start + interval '1 week')::int,
		       (SELECT count(*) FROM staff_log l WHERE l.created_at >= w.start AND l.created_at < w.start + interval '1 week' AND l.staff_player_id IS NOT NULL)::int
		FROM w ORDER BY w.start`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Week
	for rows.Next() {
		var w Week
		if err := rows.Scan(&w.Start, &w.FlagsReviewed, &w.TicketReplies, &w.CaseEntries, &w.Actions); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
