package dashboard

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMilestones(t *testing.T) {
	o := Overview{
		PlaytimeHours: 12, Cash: 40_000, Bank: 70_000,
		Vehicles: []Vehicle{{}}, Licences: []string{"Driver", "Boat"},
		Sides: []Side{{Key: "civilian", Member: true}, {Key: "police"}, {Key: "ems", Member: true}},
	}
	ms := Milestones(o, false)
	get := func(title string) Milestone {
		for _, m := range ms {
			if m.Title == title {
				return m
			}
		}
		t.Fatalf("no milestone %q", title)
		return Milestone{}
	}
	cases := []struct {
		title    string
		done     bool
		progress int
	}{
		{"First steps", true, 100},
		{"Regular", true, 100},
		{"Veteran", false, 12},
		{"Wheels", true, 100},
		{"Homeowner", false, 0},
		{"Licensed", false, 66},
		{"Six figures", true, 100},
		{"Millionaire", false, 11},
		{"Crew", false, 0},
		{"Sworn in", true, 100},
	}
	for _, c := range cases {
		m := get(c.title)
		if m.Done != c.done || m.Progress != c.progress {
			t.Errorf("%s: done %v progress %d, want %v %d", c.title, m.Done, m.Progress, c.done, c.progress)
		}
	}
	if got := get("Licensed").Detail; got != "2 of 3 licences" {
		t.Errorf("licensed detail %q", got)
	}
	if n := MilestonesDone(ms); n != 5 {
		t.Errorf("done = %d, want 5", n)
	}
	if !Milestones(o, true)[8].Done {
		t.Error("in a gang should complete Crew")
	}
}

func TestBankHistory(t *testing.T) {
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
	var me, civ, cop int64
	if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000902', 'Bank History') RETURNING id`).Scan(&me); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM players WHERE id = $1`, me) })

	// Nothing yet: no movement, all zero.
	hist, moved, err := BankHistory(ctx, pool, me, 30)
	if err != nil {
		t.Fatal(err)
	}
	if moved || len(hist) != 30 || hist[29].Balance != 0 {
		t.Fatalf("empty: moved %v, %d days, last %d", moved, len(hist), hist[len(hist)-1].Balance)
	}

	pool.QueryRow(ctx, `INSERT INTO bank_accounts (player_id, faction) VALUES ($1, 'civilian') RETURNING id`, me).Scan(&civ)
	pool.QueryRow(ctx, `INSERT INTO bank_accounts (player_id, faction) VALUES ($1, 'police') RETURNING id`, me).Scan(&cop)
	// 40 days ago +1000 (before the window), 10 days ago +500 civ,
	// 5 days ago -300 civ, today +200 police. Local noon so the day is certain.
	for _, tx := range []struct {
		acct    int64
		typ     string
		amount  int64
		daysAgo int
	}{
		{civ, "deposit", 1000, 40}, {civ, "payout", 500, 10}, {civ, "purchase", -300, 5}, {cop, "payout", 200, 0},
	} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO bank_transactions (account_id, type, amount, balance_after, created_at)
			VALUES ($1, $2, $3, 0, LEAST(now(), ((now() AT TIME ZONE $5)::date - $4::int + time '12:00') AT TIME ZONE $5))`,
			tx.acct, tx.typ, tx.amount, tx.daysAgo, Region); err != nil {
			t.Fatal(err)
		}
	}
	hist, moved, err = BankHistory(ctx, pool, me, 30)
	if err != nil {
		t.Fatal(err)
	}
	if !moved {
		t.Error("movement in the window not noticed")
	}
	want := map[int]int64{0: 1000, 29 - 10: 1500, 29 - 6: 1500, 29 - 5: 1200, 28: 1200, 29: 1400}
	for i, b := range want {
		if hist[i].Balance != b {
			t.Errorf("day %d: balance %d, want %d", i, hist[i].Balance, b)
		}
	}
}
