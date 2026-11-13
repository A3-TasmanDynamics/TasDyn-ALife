package cases

// Player context for support tickets: a staff member answering a ticket
// sees the requester's (and, for a report in a case, the reported
// players') moderation history at a glance -- without leaving the ticket.

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ContextPerms says which parts the viewer may see (their own pages'
// permissions); the ban, kicks and other tickets are shown to all staff.
type ContextPerms struct {
	Cases, Flags, Watch bool
}

// PlayerContext is one player's history for the ticket sidebar.
type PlayerContext struct {
	PlayerID     int64
	Name         string
	Role         string // "Requester", "Reported"
	Ban          *Ban
	Kicks30      int
	OtherTickets int
	// Only with the matching permission:
	ShowCases    bool
	ActivePoints int
	OpenCases    []ListRow
	ClosedCases  int
	ShowFlags    bool
	OpenFlags    int
	ShowWatch    bool
	Watched      string // the watchlist reason, "" if not watched
}

// Clean reports whether there's nothing on record to show.
func (c PlayerContext) Clean() bool {
	return c.Ban == nil && c.Kicks30 == 0 && c.ActivePoints == 0 && len(c.OpenCases) == 0 && c.ClosedCases == 0 && c.OpenFlags == 0 && c.Watched == ""
}

// ContextFor builds one player's context; ticketID is left out of their
// other tickets.
func ContextFor(ctx context.Context, pool *pgxpool.Pool, playerID, ticketID int64, role string, p ContextPerms) (PlayerContext, error) {
	c := PlayerContext{PlayerID: playerID, Role: role, ShowCases: p.Cases, ShowFlags: p.Flags, ShowWatch: p.Watch}
	err := pool.QueryRow(ctx, `
		SELECT COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id),
		       (SELECT count(*) FROM kick_log k WHERE k.target_player_id = p.id AND k.created_at > now() - interval '30 days')::int,
		       (SELECT count(*) FROM support_tickets t WHERE t.player_id = p.id AND t.id <> $2)::int,
		       (SELECT count(*) FROM anti_cheat_flags f WHERE f.player_id = p.id AND f.resolution IS NULL)::int,
		       COALESCE((SELECT w.reason FROM player_watchlist w WHERE w.player_id = p.id), ''),
		       (SELECT count(*) FROM staff_case_participants cp JOIN staff_cases sc ON sc.id = cp.case_id
		         WHERE cp.player_id = p.id AND cp.role = 'subject' AND sc.status = 'closed')::int
		FROM players p WHERE p.id = $1`, playerID, ticketID).Scan(&c.Name, &c.Kicks30, &c.OtherTickets, &c.OpenFlags, &c.Watched, &c.ClosedCases)
	if err != nil {
		return c, err
	}
	if c.Ban, _, err = ActiveBan(ctx, pool, playerID); err != nil {
		return c, err
	}
	if p.Cases {
		if _, c.ActivePoints, err = PlayerPoints(ctx, pool, playerID); err != nil {
			return c, err
		}
		// Only cases about them (as the subject), not ones they reported.
		rows, err := pool.Query(ctx, `
			SELECT c.id, c.summary FROM staff_cases c JOIN staff_case_participants cp ON cp.case_id = c.id
			WHERE cp.player_id = $1 AND cp.role = 'subject' AND c.status = 'open' ORDER BY c.id DESC LIMIT 5`, playerID)
		if err != nil {
			return c, err
		}
		for rows.Next() {
			var lr ListRow
			if err := rows.Scan(&lr.ID, &lr.Summary); err != nil {
				rows.Close()
				return c, err
			}
			lr.Status = "open"
			c.OpenCases = append(c.OpenCases, lr)
		}
		rows.Close()
	} else {
		c.ClosedCases = 0
	}
	if !p.Flags {
		c.OpenFlags = 0
	}
	if !p.Watch {
		c.Watched = ""
	}
	return c, nil
}

// TicketContext is the requester's context and, when the ticket is a
// report linked to a case, each reported player's (the case's subjects).
func TicketContext(ctx context.Context, pool *pgxpool.Pool, ticketID, requesterID int64, p ContextPerms) ([]PlayerContext, error) {
	req, err := ContextFor(ctx, pool, requesterID, ticketID, "Requester", p)
	if err != nil {
		return nil, err
	}
	out := []PlayerContext{req}
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT cp.player_id FROM support_tickets t
		JOIN staff_case_participants cp ON cp.case_id = t.case_id AND cp.role = 'subject'
		WHERE t.id = $1 AND cp.player_id <> $2 ORDER BY 1 LIMIT 4`, ticketID, requesterID)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		c, err := ContextFor(ctx, pool, id, ticketID, "Reported", p)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}
