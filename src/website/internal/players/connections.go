package players

// Player Lookup's Connections tab: who might be the same person (shared
// IPs, past names) and who they move money and items to and from (gives
// and bank transfers, from the event log) -- for alts and laundering.

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Alias struct {
	Name        string
	First, Last time.Time
}

// SharedIP is another account seen on an IP this player also used. The
// address itself isn't shown (the same as the Sessions tab).
type SharedIP struct {
	PlayerID int64
	Name     string
	IPs      int       // how many addresses they have in common
	Sources  string    // "website", "game" or "website, game"
	Last     time.Time // the latest either of them was seen on a shared one
	Banned   bool
}

// Partner is someone this player traded with: cash and items given each
// way, and bank transfers each way.
type Partner struct {
	PlayerID   int64
	Name       string
	Sent, Recv int64 // dollars: cash given + bank transfers
	ItemsSent  int64
	ItemsRecv  int64
	Times      int
	Last       time.Time
	SharesIP   bool
}

type Connections struct {
	Aliases  []Alias
	Shared   []SharedIP
	Partners []Partner
}

func GetConnections(ctx context.Context, pool *pgxpool.Pool, id int64) (Connections, error) {
	var c Connections
	rows, err := pool.Query(ctx, `SELECT name, first_seen_at, last_seen_at FROM player_aliases WHERE player_id = $1 ORDER BY last_seen_at DESC`, id)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var a Alias
		if err := rows.Scan(&a.Name, &a.First, &a.Last); err != nil {
			rows.Close()
			return c, err
		}
		c.Aliases = append(c.Aliases, a)
	}
	rows.Close()

	rows, err = pool.Query(ctx, `
		SELECT o.player_id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id),
		       count(DISTINCT o.ip_address), string_agg(DISTINCT o.source, ', ' ORDER BY o.source),
		       max(GREATEST(o.last_seen, m.last_seen)),
		       EXISTS (SELECT 1 FROM banlist b WHERE b.uid = p.uid AND (b.expires_at IS NULL OR b.expires_at > now()))
		FROM player_ips m
		JOIN player_ips o ON o.ip_address = m.ip_address AND o.player_id <> m.player_id
		JOIN players p ON p.id = o.player_id
		WHERE m.player_id = $1
		GROUP BY o.player_id, p.id ORDER BY 3 DESC, 5 DESC LIMIT 50`, id)
	if err != nil {
		return c, err
	}
	sharesIP := map[int64]bool{}
	for rows.Next() {
		var s SharedIP
		if err := rows.Scan(&s.PlayerID, &s.Name, &s.IPs, &s.Sources, &s.Last, &s.Banned); err != nil {
			rows.Close()
			return c, err
		}
		sharesIP[s.PlayerID] = true
		c.Shared = append(c.Shared, s)
	}
	rows.Close()

	// Every give / transfer either way: the giver is player_log.player_id,
	// the receiver details->>'to' (a Steam ID).
	rows, err = pool.Query(ctx, `
		WITH me AS (SELECT id, uid FROM players WHERE id = $1),
		moves AS (
		  SELECT o.id AS other, true AS outgoing, l.event_type, l.details, l.created_at
		  FROM player_log l JOIN me ON l.player_id = me.id JOIN players o ON o.uid = l.details ->> 'to'
		  WHERE l.event_type IN ('give.cash', 'give.item', 'bank.transfer')
		  UNION ALL
		  SELECT l.player_id, false, l.event_type, l.details, l.created_at
		  FROM player_log l, me
		  WHERE l.event_type IN ('give.cash', 'give.item', 'bank.transfer') AND l.details ->> 'to' = me.uid AND l.player_id <> me.id
		)
		SELECT m.other, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id),
		       COALESCE(sum(CASE WHEN m.outgoing AND m.event_type <> 'give.item' AND jsonb_typeof(m.details -> 'amount') = 'number' THEN (m.details ->> 'amount')::numeric END), 0)::bigint,
		       COALESCE(sum(CASE WHEN NOT m.outgoing AND m.event_type <> 'give.item' AND jsonb_typeof(m.details -> 'amount') = 'number' THEN (m.details ->> 'amount')::numeric END), 0)::bigint,
		       COALESCE(sum(CASE WHEN m.outgoing AND m.event_type = 'give.item' AND jsonb_typeof(m.details -> 'qty') = 'number' THEN (m.details ->> 'qty')::numeric END), 0)::bigint,
		       COALESCE(sum(CASE WHEN NOT m.outgoing AND m.event_type = 'give.item' AND jsonb_typeof(m.details -> 'qty') = 'number' THEN (m.details ->> 'qty')::numeric END), 0)::bigint,
		       count(*), max(m.created_at)
		FROM moves m JOIN players p ON p.id = m.other
		GROUP BY m.other, p.id`, id)
	if err != nil {
		return c, err
	}
	defer rows.Close()
	for rows.Next() {
		var p Partner
		if err := rows.Scan(&p.PlayerID, &p.Name, &p.Sent, &p.Recv, &p.ItemsSent, &p.ItemsRecv, &p.Times, &p.Last); err != nil {
			return c, err
		}
		p.SharesIP = sharesIP[p.PlayerID]
		c.Partners = append(c.Partners, p)
	}
	// Most money moved first, then most often.
	sort.SliceStable(c.Partners, func(i, j int) bool {
		a, b := c.Partners[i], c.Partners[j]
		if a.Sent+a.Recv != b.Sent+b.Recv {
			return a.Sent+a.Recv > b.Sent+b.Recv
		}
		return a.Times > b.Times
	})
	if len(c.Partners) > 50 {
		c.Partners = c.Partners[:50]
	}
	return c, rows.Err()
}
