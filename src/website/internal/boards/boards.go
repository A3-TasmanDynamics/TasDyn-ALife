// Package boards is the admin panel's Gangs and Wanted boards: read-only
// overviews of every gang (members, bank, wealth, wanted members, crime)
// and of who's wanted and who's in jail.
package boards

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/gamenames"
)

const playerName = `COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id)`

// online is true while the player has an open game session.
const online = `EXISTS (SELECT 1 FROM player_sessions s WHERE s.player_id = p.id AND s.disconnected_at IS NULL)`

// ---- Gangs ----------------------------------------------------------------

type Gang struct {
	ID        int64
	Name, Tag string
	LeaderID  int64
	Leader    string
	Members   int
	Online    int
	Bank      int64 // the gang's own account
	Wealth    int64 // members' cash + bank
	Wanted    int   // members with an outstanding bounty
	Bounties  int64
	Crimes    int // robberies / vault break-ins by members, last 7 days
	Vehicles  int
	Created   time.Time
}

func Gangs(ctx context.Context, pool *pgxpool.Pool, search string) ([]Gang, error) {
	rows, err := pool.Query(ctx, `
		WITH m AS (
		  SELECT gm.gang_id, p.id, p.uid, `+online+` AS is_on,
		         COALESCE(p.civ_cash, 0) + COALESCE(p.cop_cash, 0) + COALESCE(p.medic_cash, 0)
		           + COALESCE((SELECT sum(balance) FROM bank_accounts b WHERE b.player_id = p.id), 0) AS wealth,
		         COALESCE((SELECT sum(bounty_amount) FROM wanted_crimes w WHERE w.player_id = p.id AND w.cleared_at IS NULL), 0) AS bounty,
		         (SELECT count(*) FROM player_log l WHERE l.player_id = p.id AND l.created_at > now() - interval '7 days'
		            AND l.event_type IN ('robbery.gas', 'evidence.robbed', 'evidence.breakin')) AS crimes
		  FROM gang_members gm JOIN players p ON p.id = gm.player_id)
		SELECT g.id, g.name, g.tag, g.leader_player_id, COALESCE(NULLIF(l.name, ''), NULLIF(l.steam_name, ''), 'Player #' || l.id),
		       count(m.id), count(m.id) FILTER (WHERE m.is_on), COALESCE(a.balance, 0),
		       COALESCE(sum(m.wealth), 0)::bigint, count(m.id) FILTER (WHERE m.bounty > 0), COALESCE(sum(m.bounty), 0)::bigint,
		       COALESCE(sum(m.crimes), 0)::int,
		       (SELECT count(*) FROM vehicles v WHERE v.owner_gang_id = g.id AND v.status NOT IN ('destroyed', 'sold')),
		       g.created_at
		FROM gangs g JOIN players l ON l.id = g.leader_player_id
		LEFT JOIN gang_accounts a ON a.gang_id = g.id
		LEFT JOIN m ON m.gang_id = g.id
		WHERE $1 = '' OR g.name ILIKE '%' || $1 || '%' OR g.tag ILIKE '%' || $1 || '%'
		GROUP BY g.id, l.id, a.balance
		ORDER BY count(m.id) FILTER (WHERE m.is_on) DESC, count(m.id) DESC, g.name`, strings.TrimSpace(search))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Gang
	for rows.Next() {
		var g Gang
		if err := rows.Scan(&g.ID, &g.Name, &g.Tag, &g.LeaderID, &g.Leader, &g.Members, &g.Online, &g.Bank,
			&g.Wealth, &g.Wanted, &g.Bounties, &g.Crimes, &g.Vehicles, &g.Created); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

type Member struct {
	PlayerID int64
	Name     string
	Rank     string // leader / officer / member
	Joined   time.Time
	Online   bool
	Wealth   int64
	Bounty   int64
}

type GangLogRow struct {
	When   time.Time
	Actor  string
	Action string
	Detail string
}

type GangTx struct {
	When   time.Time
	Type   string
	Amount int64
	By     string
	Memo   string
}

// Abs is the amount without its sign (the page colours it instead).
func (t GangTx) Abs() int64 {
	if t.Amount < 0 {
		return -t.Amount
	}
	return t.Amount
}

type GangDetail struct {
	Gang
	MemberList []Member
	Log        []GangLogRow
	Txs        []GangTx
}

func GetGang(ctx context.Context, pool *pgxpool.Pool, id int64) (GangDetail, bool, error) {
	var d GangDetail
	all, err := Gangs(ctx, pool, "")
	if err != nil {
		return d, false, err
	}
	found := false
	for _, g := range all {
		if g.ID == id {
			d.Gang, found = g, true
		}
	}
	if !found {
		return d, false, nil
	}
	rows, err := pool.Query(ctx, `
		SELECT p.id, `+playerName+`, CASE WHEN p.id = g.leader_player_id THEN 'leader' ELSE gm.rank END, gm.joined_at, `+online+`,
		       COALESCE(p.civ_cash, 0) + COALESCE(p.cop_cash, 0) + COALESCE(p.medic_cash, 0)
		         + COALESCE((SELECT sum(balance) FROM bank_accounts b WHERE b.player_id = p.id), 0)::bigint,
		       COALESCE((SELECT sum(bounty_amount) FROM wanted_crimes w WHERE w.player_id = p.id AND w.cleared_at IS NULL), 0)::bigint
		FROM gang_members gm JOIN players p ON p.id = gm.player_id JOIN gangs g ON g.id = gm.gang_id
		WHERE gm.gang_id = $1
		ORDER BY (p.id = g.leader_player_id) DESC, (gm.rank = 'officer') DESC, gm.joined_at`, id)
	if err != nil {
		return d, true, err
	}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.PlayerID, &m.Name, &m.Rank, &m.Joined, &m.Online, &m.Wealth, &m.Bounty); err != nil {
			rows.Close()
			return d, true, err
		}
		d.MemberList = append(d.MemberList, m)
	}
	rows.Close()
	rows, err = pool.Query(ctx, `
		SELECT gl.created_at, COALESCE(`+playerName+`, 'system'), gl.action, COALESCE(gl.details::text, '')
		FROM gang_log gl LEFT JOIN players p ON p.id = gl.actor_player_id
		WHERE gl.gang_id = $1 ORDER BY gl.created_at DESC LIMIT 30`, id)
	if err != nil {
		return d, true, err
	}
	for rows.Next() {
		var l GangLogRow
		var details string
		if err := rows.Scan(&l.When, &l.Actor, &l.Action, &details); err != nil {
			rows.Close()
			return d, true, err
		}
		l.Action = strings.ReplaceAll(l.Action, "_", " ")
		l.Detail = flat(details)
		d.Log = append(d.Log, l)
	}
	rows.Close()
	rows, err = pool.Query(ctx, `
		SELECT t.created_at, t.type, t.amount, COALESCE(`+playerName+`, ''), COALESCE(t.memo, '')
		FROM gang_transactions t JOIN gang_accounts a ON a.id = t.gang_account_id LEFT JOIN players p ON p.id = t.actor_player_id
		WHERE a.gang_id = $1 ORDER BY t.created_at DESC, t.id DESC LIMIT 30`, id)
	if err != nil {
		return d, true, err
	}
	defer rows.Close()
	for rows.Next() {
		var t GangTx
		if err := rows.Scan(&t.When, &t.Type, &t.Amount, &t.By, &t.Memo); err != nil {
			return d, true, err
		}
		t.Type = strings.ReplaceAll(t.Type, "_", " ")
		d.Txs = append(d.Txs, t)
	}
	return d, true, rows.Err()
}

// flat turns a details object into "key: value, key: value".
func flat(raw string) string {
	var m map[string]any
	if raw == "" || json.Unmarshal([]byte(raw), &m) != nil {
		return raw
	}
	var parts []string
	for k, v := range m {
		b, _ := json.Marshal(v)
		parts = append(parts, strings.ReplaceAll(k, "_", " ")+": "+strings.Trim(string(b), `"`))
	}
	return strings.Join(parts, ", ")
}

// ---- Wanted & jail ---------------------------------------------------------

type WantedRow struct {
	PlayerID int64
	Name     string
	Gang     string
	Online   bool
	Bounty   int64
	Crimes   []string // crime names, most recent first ("Robbery x2")
	Since    time.Time
	Latest   time.Time
}

type Jailed struct {
	PlayerID int64
	Name     string
	Online   bool
	Left     time.Duration
	Bail     int64
}

type Wanted struct {
	List    []WantedRow
	Jail    []Jailed
	Total   int64
	ByCrime []CrimeCount
}

type CrimeCount struct {
	Name  string
	Count int
	Total int64
}

func Board(ctx context.Context, pool *pgxpool.Pool, search string) (Wanted, error) {
	var w Wanted
	search = strings.TrimSpace(search)
	rows, err := pool.Query(ctx, `
		SELECT p.id, `+playerName+`, COALESCE(g.name, ''), `+online+`, sum(c.bounty_amount)::bigint,
		       array_agg(c.crime_key ORDER BY c.created_at DESC), min(c.created_at), max(c.created_at)
		FROM wanted_crimes c JOIN players p ON p.id = c.player_id
		LEFT JOIN gang_members gm ON gm.player_id = p.id LEFT JOIN gangs g ON g.id = gm.gang_id
		WHERE c.cleared_at IS NULL
		  AND ($1 = '' OR p.uid = $1 OR p.name ILIKE '%' || $1 || '%' OR p.steam_name ILIKE '%' || $1 || '%' OR g.name ILIKE '%' || $1 || '%')
		GROUP BY p.id, g.name ORDER BY 5 DESC, p.id LIMIT 200`, search)
	if err != nil {
		return w, err
	}
	for rows.Next() {
		var r WantedRow
		var keys []string
		if err := rows.Scan(&r.PlayerID, &r.Name, &r.Gang, &r.Online, &r.Bounty, &keys, &r.Since, &r.Latest); err != nil {
			rows.Close()
			return w, err
		}
		r.Crimes = crimeList(keys)
		w.Total += r.Bounty
		w.List = append(w.List, r)
	}
	rows.Close()
	rows, err = pool.Query(ctx, `
		SELECT p.id, `+playerName+`, `+online+`, p.civ_jail_seconds, COALESCE(p.civ_bail, 0)
		FROM players p WHERE p.civ_jail_seconds > 0
		  AND ($1 = '' OR p.uid = $1 OR p.name ILIKE '%' || $1 || '%' OR p.steam_name ILIKE '%' || $1 || '%')
		ORDER BY p.civ_jail_seconds DESC LIMIT 200`, search)
	if err != nil {
		return w, err
	}
	for rows.Next() {
		var j Jailed
		var secs int64
		if err := rows.Scan(&j.PlayerID, &j.Name, &j.Online, &secs, &j.Bail); err != nil {
			rows.Close()
			return w, err
		}
		j.Left = time.Duration(secs) * time.Second
		w.Jail = append(w.Jail, j)
	}
	rows.Close()
	rows, err = pool.Query(ctx, `
		SELECT crime_key, count(*), sum(bounty_amount)::bigint FROM wanted_crimes WHERE cleared_at IS NULL
		GROUP BY crime_key ORDER BY 3 DESC, 2 DESC`)
	if err != nil {
		return w, err
	}
	defer rows.Close()
	for rows.Next() {
		var c CrimeCount
		var key string
		if err := rows.Scan(&key, &c.Count, &c.Total); err != nil {
			return w, err
		}
		c.Name = gamenames.Crime(key)
		w.ByCrime = append(w.ByCrime, c)
	}
	return w, rows.Err()
}

// crimeList names crimes, newest first, repeats folded: "Robbery x2".
func crimeList(keys []string) []string {
	var order []string
	n := map[string]int{}
	for _, k := range keys {
		if n[k] == 0 {
			order = append(order, k)
		}
		n[k]++
	}
	out := make([]string, 0, len(order))
	for _, k := range order {
		s := gamenames.Crime(k)
		if n[k] > 1 {
			s += " x" + strconv.Itoa(n[k])
		}
		out = append(out, s)
	}
	return out
}
