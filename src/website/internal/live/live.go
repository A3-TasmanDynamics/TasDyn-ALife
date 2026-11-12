// Package live pushes admin panel updates to open pages over Server-Sent
// Events (/admin/live). One poller per website process looks at the
// database every few seconds -- a single cheap query -- and when something
// changed it tells every connected page: the sidebar badge counts, how many
// are online, and new anti-cheat flags, tickets, cases and watched players
// joining. Each message names the permission needed to see it, and the
// handler only forwards what the viewer may see.
package live

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Every is how often the poller looks.
const Every = 4 * time.Second

// Msg is one update for the browser.
type Msg struct {
	Topic string `json:"topic"` // counts / flag / ticket / case / watch / online
	// Perm the viewer needs: a permission key, "-support" for support panel
	// access, or "" for anyone with the admin panel.
	Perm string `json:"-"`
	Data any    `json:"data"`
}

// Counts are the sidebar badges and the online count.
type Counts struct {
	OpenCases         int `json:"cases"`
	OpenFlags         int `json:"anticheat"`
	ActiveBans        int `json:"bans"`
	OpenApps          int `json:"applications"`
	OpenTickets       int `json:"tickets_open"`
	UnassignedTickets int `json:"tickets_unassigned"`
	Online            int `json:"online"`
}

// Toast is a new thing worth a pop-up.
type Toast struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"`
	URL   string `json:"url"`
	Level string `json:"level"` // critical / warning / info
}

type cursor struct {
	flag, ticket, kase, session int64
	gameDone                    float64 // the latest game_commands.done_at (epoch)
}

// Hub runs the poller and fans messages out to subscribers.
type Hub struct {
	pool *pgxpool.Pool

	mu     sync.Mutex
	subs   map[chan Msg]struct{}
	counts Counts
	cur    cursor
	primed bool
}

func NewHub(pool *pgxpool.Pool) *Hub {
	return &Hub{pool: pool, subs: map[chan Msg]struct{}{}}
}

// Subscribe returns a channel of messages (the current counts first) and a
// function to stop.
func (h *Hub) Subscribe() (<-chan Msg, func()) {
	ch := make(chan Msg, 32)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	if h.primed {
		ch <- Msg{Topic: "counts", Data: h.counts}
	}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
}

// Subscribers is how many pages are listening.
func (h *Hub) Subscribers() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

func (h *Hub) send(msgs []Msg) {
	if len(msgs) == 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		for _, m := range msgs {
			select {
			case ch <- m:
			default: // a stuck page: drop rather than block everyone
			}
		}
	}
}

// Run polls until ctx ends. It only queries while someone is listening.
func (h *Hub) Run(ctx context.Context) {
	t := time.NewTicker(Every)
	defer t.Stop()
	for {
		if h.Subscribers() > 0 || !h.primed {
			msgs, err := h.Poll(ctx)
			if err != nil && ctx.Err() == nil {
				slog.Warn("live: poll failed", "error", err)
			}
			h.send(msgs)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Poll reads the database once and returns what changed since last time.
// The first poll only records where things are (no toasts for old rows).
func (h *Hub) Poll(ctx context.Context) ([]Msg, error) {
	var c Counts
	var cur cursor
	err := h.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM staff_cases WHERE status = 'open'),
		       (SELECT count(*) FROM anti_cheat_flags WHERE resolution IS NULL),
		       (SELECT count(*) FROM banlist WHERE expires_at IS NULL OR expires_at > now()),
		       (SELECT count(*) FROM staff_applications WHERE status IN ('pending', 'interview')),
		       (SELECT count(*) FROM support_tickets WHERE status <> 'closed'),
		       (SELECT count(*) FROM support_tickets WHERE status <> 'closed' AND assigned_staff_id IS NULL),
		       (SELECT count(DISTINCT player_id) FROM player_sessions WHERE disconnected_at IS NULL),
		       COALESCE((SELECT max(id) FROM anti_cheat_flags), 0),
		       COALESCE((SELECT max(id) FROM support_tickets), 0),
		       COALESCE((SELECT max(id) FROM staff_cases), 0),
		       COALESCE((SELECT max(id) FROM player_sessions), 0),
		       COALESCE((SELECT EXTRACT(EPOCH FROM max(done_at)) FROM game_commands), 0)::float8`).Scan(
		&c.OpenCases, &c.OpenFlags, &c.ActiveBans, &c.OpenApps, &c.OpenTickets, &c.UnassignedTickets, &c.Online,
		&cur.flag, &cur.ticket, &cur.kase, &cur.session, &cur.gameDone)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	prev, prevCounts, primed := h.cur, h.counts, h.primed
	h.mu.Unlock()

	var msgs []Msg
	if !primed || c != prevCounts {
		msgs = append(msgs, Msg{Topic: "counts", Data: c})
	}
	if primed {
		if c.Online != prevCounts.Online || cur.session != prev.session {
			msgs = append(msgs, Msg{Topic: "online", Perm: "players.view", Data: map[string]int{"online": c.Online}})
		}
		if cur.gameDone != prev.gameDone {
			msgs = append(msgs, Msg{Topic: "gamecmd", Perm: "players.view", Data: map[string]float64{"at": cur.gameDone}})
		}
		more, err := h.news(ctx, prev, cur)
		if err != nil {
			return msgs, err
		}
		msgs = append(msgs, more...)
	}
	h.mu.Lock()
	h.counts, h.cur, h.primed = c, cur, true
	h.mu.Unlock()
	return msgs, nil
}

const maxToasts = 5 // per kind per poll; a burst says "and N more"

// news turns new rows (between the old and new cursors) into toasts.
func (h *Hub) news(ctx context.Context, prev, cur cursor) ([]Msg, error) {
	var out []Msg
	if cur.flag > prev.flag {
		rows, err := h.pool.Query(ctx, `
			SELECT f.id, replace(f.flag_type, '_', ' '), f.confidence, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id)
			FROM anti_cheat_flags f JOIN players p ON p.id = f.player_id
			WHERE f.id > $1 AND f.id <= $2 ORDER BY f.id LIMIT $3`, prev.flag, cur.flag, maxToasts)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			var typ, conf, name string
			if err := rows.Scan(&id, &typ, &conf, &name); err != nil {
				rows.Close()
				return nil, err
			}
			level := "info"
			if conf == "high" {
				level = "warning"
			}
			out = append(out, Msg{Topic: "flag", Perm: "anticheat.review", Data: Toast{id, "New anti-cheat flag", fmt.Sprintf("%s · %s (%s)", name, typ, conf), fmt.Sprintf("/admin/anticheat?flag=%d", id), level}})
		}
		rows.Close()
	}
	if cur.ticket > prev.ticket {
		rows, err := h.pool.Query(ctx, `
			SELECT t.id, t.subject, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id)
			FROM support_tickets t JOIN players p ON p.id = t.player_id
			WHERE t.id > $1 AND t.id <= $2 ORDER BY t.id LIMIT $3`, prev.ticket, cur.ticket, maxToasts)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			var subject, name string
			if err := rows.Scan(&id, &subject, &name); err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, Msg{Topic: "ticket", Perm: "-support", Data: Toast{id, "New support ticket", name + " · " + subject, fmt.Sprintf("/admin/tickets/%d", id), "info"}})
		}
		rows.Close()
	}
	if cur.kase > prev.kase {
		rows, err := h.pool.Query(ctx, `
			SELECT c.id, c.summary, COALESCE(NULLIF(l.name, ''), NULLIF(l.steam_name, ''), 'Someone')
			FROM staff_cases c LEFT JOIN players l ON l.id = c.lead_staff_id
			WHERE c.id > $1 AND c.id <= $2 ORDER BY c.id LIMIT $3`, prev.kase, cur.kase, maxToasts)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			var summary, lead string
			if err := rows.Scan(&id, &summary, &lead); err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, Msg{Topic: "case", Perm: "cases.view", Data: Toast{id, fmt.Sprintf("Case #%d opened", id), lead + " · " + summary, fmt.Sprintf("/admin/cases/%d", id), "info"}})
		}
		rows.Close()
	}
	if cur.session > prev.session {
		rows, err := h.pool.Query(ctx, `
			SELECT s.id, p.id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id), w.reason
			FROM player_sessions s
			JOIN player_watchlist w ON w.player_id = s.player_id
			JOIN players p ON p.id = s.player_id
			WHERE s.id > $1 AND s.id <= $2 ORDER BY s.id LIMIT $3`, prev.session, cur.session, maxToasts)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, pid int64
			var name, reason string
			if err := rows.Scan(&id, &pid, &name, &reason); err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, Msg{Topic: "watch", Perm: "players.view", Data: Toast{id, "Watched player joined", name + " · " + reason, fmt.Sprintf("/admin/players?id=%d", pid), "warning"}})
		}
		rows.Close()
	}
	return out, nil
}

// Allowed reports whether a viewer with perms (and support access) may
// see m.
func Allowed(m Msg, perms map[string]bool, admin, support bool) bool {
	switch m.Perm {
	case "":
		return admin || support
	case "-support":
		return support
	}
	return admin && perms[m.Perm]
}

// Encode is m as an SSE frame.
func Encode(m Msg) []byte {
	b, _ := json.Marshal(m.Data)
	return []byte("event: " + m.Topic + "\ndata: " + string(b) + "\n\n")
}
