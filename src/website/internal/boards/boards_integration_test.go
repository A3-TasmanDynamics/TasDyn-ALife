package boards

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBoards(t *testing.T) {
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

	ids := map[string]int64{}
	for name, uid := range map[string]string{"boss": "76561190000000901", "officer": "76561190000000902", "grunt": "76561190000000903", "lone": "76561190000000904"} {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name, civ_cash) VALUES ($1, $2, 1000) RETURNING id`, uid, "Board "+name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[name] = id
	}
	all := []int64{ids["boss"], ids["officer"], ids["grunt"], ids["lone"]}
	var gid int64
	if err := pool.QueryRow(ctx, `INSERT INTO gangs (name, tag, leader_player_id) VALUES ('Board Test Crew', 'BTC', $1) RETURNING id`, ids["boss"]).Scan(&gid); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM gangs WHERE id = $1`, gid)
		pool.Exec(ctx, `DELETE FROM players WHERE id = ANY($1)`, all)
	})
	for who, rank := range map[string]string{"boss": "member", "officer": "officer", "grunt": "member"} {
		if _, err := pool.Exec(ctx, `INSERT INTO gang_members (gang_id, player_id, rank) VALUES ($1, $2, $3)`, gid, ids[who], rank); err != nil {
			t.Fatal(err)
		}
	}
	var acct int64
	if err := pool.QueryRow(ctx, `INSERT INTO gang_accounts (gang_id) VALUES ($1) RETURNING id`, gid).Scan(&acct); err != nil {
		t.Fatal(err)
	}
	pool.Exec(ctx, `INSERT INTO gang_transactions (gang_account_id, type, amount, balance_after, actor_player_id) VALUES ($1, 'deposit', 50000, 50000, $2)`, acct, ids["boss"])
	pool.Exec(ctx, `UPDATE gang_accounts SET balance = 50000 WHERE id = $1`, acct) // in case no trigger keeps it
	pool.Exec(ctx, `INSERT INTO gang_log (gang_id, actor_player_id, action, details) VALUES ($1, $2, 'member_added', '{"member":"Board grunt"}')`, gid, ids["boss"])
	pool.Exec(ctx, `INSERT INTO player_sessions (player_id) VALUES ($1)`, ids["officer"]) // online
	pool.Exec(ctx, `INSERT INTO wanted_crimes (player_id, crime_key, bounty_amount, created_at) VALUES
		($1, 'evidence_robbery', 20000, now() - interval '2 hours'), ($1, 'evidence_robbery', 20000, now() - interval '1 hour'), ($2, 'evidence_robbery', 20000, now())`, ids["grunt"], ids["lone"])
	pool.Exec(ctx, `INSERT INTO player_log (player_id, event_type, details) VALUES ($1, 'robbery.gas', '{"cash":100}')`, ids["grunt"])
	pool.Exec(ctx, `UPDATE players SET civ_jail_seconds = 900, civ_bail = 7500 WHERE id = $1`, ids["lone"])

	gangs, err := Gangs(ctx, pool, "board test")
	if err != nil {
		t.Fatal(err)
	}
	if len(gangs) != 1 {
		t.Fatalf("gangs: %+v", gangs)
	}
	g := gangs[0]
	if g.Members != 3 || g.Online != 1 || g.Bank != 50000 || g.Wealth != 3000 || g.Wanted != 1 || g.Bounties != 40000 || g.Crimes != 1 || g.Leader != "Board boss" {
		t.Errorf("gang: %+v", g)
	}
	d, ok, err := GetGang(ctx, pool, gid)
	if err != nil || !ok {
		t.Fatalf("detail: ok=%v err=%v", ok, err)
	}
	if len(d.MemberList) != 3 || d.MemberList[0].Rank != "leader" || d.MemberList[1].Rank != "officer" || !d.MemberList[1].Online {
		t.Errorf("members: %+v", d.MemberList)
	}
	if len(d.Txs) != 1 || d.Txs[0].Amount != 50000 || d.Txs[0].By != "Board boss" || len(d.Log) != 1 || d.Log[0].Action != "member added" {
		t.Errorf("bank/log: %+v %+v", d.Txs, d.Log)
	}
	if _, ok, _ := GetGang(ctx, pool, -1); ok {
		t.Error("missing gang found")
	}

	w, err := Board(ctx, pool, "Board")
	if err != nil {
		t.Fatal(err)
	}
	if len(w.List) != 2 || w.List[0].PlayerID != ids["grunt"] || w.List[0].Bounty != 40000 || w.List[0].Gang != "Board Test Crew" {
		t.Fatalf("wanted: %+v", w.List)
	}
	if len(w.List[0].Crimes) != 1 || !strings.HasSuffix(w.List[0].Crimes[0], " x2") || w.List[0].Latest.Sub(w.List[0].Since) < 50*time.Minute {
		t.Errorf("crimes: %+v", w.List[0])
	}
	if len(w.Jail) != 1 || w.Jail[0].Left != 15*time.Minute || w.Jail[0].Bail != 7500 {
		t.Errorf("jail: %+v", w.Jail)
	}
	if w2, _ := Board(ctx, pool, "Board Test Crew"); len(w2.List) != 1 {
		t.Errorf("search by gang: %d", len(w2.List))
	}
}
