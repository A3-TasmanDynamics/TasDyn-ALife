package cases

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTicketContext(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var mod, req, bad int64
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000811', 'Ctx Mod') RETURNING id`).Scan(&mod)
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000812', 'Ctx Requester') RETURNING id`).Scan(&req)
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000813', 'Ctx Bad') RETURNING id`).Scan(&bad)
	var ticket, kase int64
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM support_tickets WHERE id = $1`, ticket)
		pool.Exec(ctx, `DELETE FROM staff_cases WHERE id = $1`, kase)
		pool.Exec(ctx, `DELETE FROM banlist WHERE uid = '76561190000000813'`)
		pool.Exec(ctx, `DELETE FROM staff_log WHERE staff_player_id = $1`, mod)
		pool.Exec(ctx, `DELETE FROM players WHERE id IN ($1, $2, $3)`, mod, req, bad)
	})
	pool.QueryRow(ctx, `INSERT INTO support_tickets (player_id, subject, category_id, subcategory_id)
		SELECT $1, 'Ctx report', s.parent_id, s.id FROM ticket_categories s WHERE s.key = 'gameplay_player_report' RETURNING id`, req).Scan(&ticket)
	// The reported player: banned, watched, kicked, a flag, and the subject of the report's case.
	pool.Exec(ctx, `INSERT INTO banlist (uid, reason, banned_by, expires_at) VALUES ('76561190000000813', 'Ctx duping', $1, now() + interval '2 days')`, mod)
	pool.Exec(ctx, `INSERT INTO player_watchlist (player_id, reason) VALUES ($1, 'ctx test')`, bad)
	pool.Exec(ctx, `INSERT INTO kick_log (target_player_id, kicked_by, kick_type, reason) VALUES ($1, $2, 'manual', 'ctx')`, bad, mod)
	pool.Exec(ctx, `INSERT INTO anti_cheat_flags (player_id, flag_type, confidence) VALUES ($1, 'dupe', 'high')`, bad)
	actor := Actor{PlayerID: mod, Source: "website"}
	if kase, err = Open(ctx, pool, actor, "exploit", "Ctx case", []int64{bad}, 0, ""); err != nil {
		t.Fatal(err)
	}
	if err := LinkReport(ctx, pool, actor, kase, ticket); err != nil {
		t.Fatal(err)
	}

	all := ContextPerms{Cases: true, Flags: true, Watch: true}
	got, err := TicketContext(ctx, pool, ticket, req, all)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Role != "Requester" || got[1].Role != "Reported" || got[1].PlayerID != bad {
		t.Fatalf("contexts: %+v", got)
	}
	if !got[0].Clean() {
		t.Errorf("the requester should have a clean record: %+v", got[0])
	}
	r := got[1]
	if r.Ban == nil || r.Ban.Reason != "Ctx duping" || r.Watched != "ctx test" || r.Kicks30 != 1 || r.OpenFlags != 1 || len(r.OpenCases) != 1 || r.Clean() {
		t.Errorf("reported: %+v", r)
	}
	// Support-only staff: ban and kicks, but not cases, flags or the watchlist.
	limited, _ := TicketContext(ctx, pool, ticket, req, ContextPerms{})
	l := limited[1]
	if l.Ban == nil || l.Kicks30 != 1 || l.Watched != "" || l.OpenFlags != 0 || len(l.OpenCases) != 0 || l.ShowCases {
		t.Errorf("limited: %+v", l)
	}
}
