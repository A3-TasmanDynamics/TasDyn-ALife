package quicksearch

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSearch(t *testing.T) {
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
	var p, lead, gid, vid, cid, fid int64
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000971', 'Quinn Zephyrine') RETURNING id`).Scan(&p)
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000972', 'QS Lead') RETURNING id`).Scan(&lead)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM staff_cases WHERE id = $1`, cid)
		pool.Exec(ctx, `DELETE FROM vehicles WHERE id = $1`, vid)
		pool.Exec(ctx, `DELETE FROM gangs WHERE id = $1`, gid)
		pool.Exec(ctx, `DELETE FROM players WHERE id IN ($1, $2)`, p, lead)
	})
	pool.QueryRow(ctx, `INSERT INTO gangs (name, tag, leader_player_id) VALUES ('Zephyr Syndicate', 'ZPH', $1) RETURNING id`, p).Scan(&gid)
	if err := pool.QueryRow(ctx, `INSERT INTO vehicles (owner_player_id, classname, plate) VALUES ($1, 'C_Offroad_01_F', 'ZPH-4417') RETURNING id`, p).Scan(&vid); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO staff_cases (case_type, summary, lead_staff_id) VALUES ('exploit', 'Zephyrine duping case', $1) RETURNING id`, lead).Scan(&cid); err != nil {
		t.Fatal(err)
	}
	pool.Exec(ctx, `INSERT INTO staff_case_participants (case_id, player_id, role) VALUES ($1, $2, 'subject')`, cid, p)
	pool.QueryRow(ctx, `INSERT INTO anti_cheat_flags (player_id, flag_type, confidence) VALUES ($1, 'gear', 'high') RETURNING id`, p).Scan(&fid)

	all := map[string]bool{"players.view": true, "players.vehicles": true, "cases.view": true, "anticheat.review": true}
	urls := func(groups []Group) string {
		var b strings.Builder
		for _, g := range groups {
			for _, h := range g.Hits {
				fmt.Fprintf(&b, "%s|%s|%s|%s\n", g.Name, h.Title, h.Sub, h.URL)
			}
		}
		return b.String()
	}
	check := func(q string, perms map[string]bool, want, notWant []string) {
		t.Helper()
		g, err := Search(ctx, pool, q, perms)
		if err != nil {
			t.Fatal(q, err)
		}
		got := urls(g)
		for _, w := range want {
			if !strings.Contains(got, w) {
				t.Errorf("%q: missing %q in\n%s", q, w, got)
			}
		}
		for _, w := range notWant {
			if strings.Contains(got, w) {
				t.Errorf("%q: should not have %q in\n%s", q, w, got)
			}
		}
	}
	check("zephyr", all, []string{
		fmt.Sprintf("Players|Quinn Zephyrine|offline · 76561190000000971|/admin/players?id=%d", p),
		fmt.Sprintf("Gangs|Zephyr Syndicate [ZPH]|0 members|/admin/gangs?id=%d", gid),
		fmt.Sprintf("/admin/cases/%d", cid),
	}, nil)
	check("zph-44", all, []string{"Vehicles|ZPH-4417 · ", fmt.Sprintf("/admin/vehicles?id=%d", vid), "Quinn Zephyrine · garaged"}, nil)
	check(fmt.Sprintf("#%d", fid), all, []string{fmt.Sprintf("Anti-cheat flags|Flag #%d · gear|Quinn Zephyrine · high · open|/admin/anticheat?flag=%d", fid, fid)}, nil)
	check("76561190000000971", all, []string{fmt.Sprintf("/admin/players?id=%d", p), fmt.Sprintf("/admin/cases/%d", cid)}, nil)
	// Without the permissions, none of it.
	check("zephyr", map[string]bool{}, nil, []string{"Players|", "Gangs|", "Cases|", "Vehicles|"})
	check("zph-44", map[string]bool{"players.view": true}, nil, []string{"Vehicles|"})
	check(fmt.Sprintf("#%d", fid), map[string]bool{"players.view": true}, nil, []string{"Anti-cheat flags|"})
	// Pages: by title and by extra words, only those the viewer may open.
	check("plates", all, []string{"Pages|Vehicle Lookup|Page|/admin/vehicles"}, nil)
	check("discord", all, nil, []string{"/admin/discord"})
	check("discord", map[string]bool{"bot.admin": true}, []string{"Pages|Discord Bot"}, nil)
	if g, _ := Search(ctx, pool, "   ", all); g != nil {
		t.Errorf("blank: %v", g)
	}
}
