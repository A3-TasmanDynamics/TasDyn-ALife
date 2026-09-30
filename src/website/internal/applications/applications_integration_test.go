package applications

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runs only with TEST_DATABASE_URL, and only when no police ranks are
// configured (it sets its own). Stop the live website first: these write
// staff_log rows and Discord DMs to the outbox.
func TestRecruitmentFlow(t *testing.T) {
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
	var n int
	pool.QueryRow(ctx, `SELECT count(*) FROM faction_rank_names WHERE faction = 'police'`).Scan(&n)
	if n > 0 {
		t.Skip("police ranks already configured on this database")
	}
	pool.Exec(ctx, `INSERT INTO faction_rank_names (faction, level, name, promote_up_to) VALUES ('police', 1, 'Cadet', 0), ('police', 2, 'Constable', 0), ('police', 3, 'Inspector', 2)`)

	ids := map[string]int64{}
	for name, row := range map[string][2]string{
		"head": {"76561190000000501", "100"}, "applicant": {"76561190000000502", ""}, "nodiscord": {"76561190000000503", ""},
		"cmd": {"76561190000000504", ""},
	} {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name, discord_id, discord_username) VALUES ($1, $2, $3, $4) RETURNING id`,
			row[0], "t_"+name, map[bool]any{true: nil, false: "99000" + row[0][12:]}[name == "nodiscord"],
			map[bool]any{true: nil, false: "t_" + name}[name == "nodiscord"]).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[name] = id
	}
	pool.Exec(ctx, `UPDATE players SET staff_rank_id = (SELECT id FROM staff_ranks WHERE level = 100 LIMIT 1) WHERE id = $1`, ids["head"])
	pool.Exec(ctx, `UPDATE players SET cop_level = 3 WHERE id = $1`, ids["cmd"])
	all := []int64{ids["head"], ids["applicant"], ids["nodiscord"], ids["cmd"]}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM discord_outbox WHERE dedupe_key LIKE 'staff-app:%' OR dedupe_key LIKE 'faction-app:%'`)
		pool.Exec(ctx, `DELETE FROM faction_log WHERE target_id = ANY($1) OR actor_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM staff_log WHERE staff_player_id = ANY($1) OR target_player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM rank_changes WHERE player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM players WHERE id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM faction_rank_names WHERE faction = 'police'`)
	})

	denied := func(what string, err error) {
		t.Helper()
		if !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%s: expected refusal, got %v", what, err)
		}
	}
	must := func(what string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	form := map[string]string{"age": "22", "tz": "AEST evenings", "about": "Hi", "why": "Helpful", "exp": "None"}

	_, err = SubmitStaff(ctx, pool, ids["nodiscord"], form, true)
	denied("no Discord linked", err)
	_, err = SubmitStaff(ctx, pool, ids["head"], form, true)
	denied("already staff", err)
	_, err = SubmitStaff(ctx, pool, ids["applicant"], form, false)
	denied("rules not agreed", err)
	bad := map[string]string{"age": "old", "tz": "x", "about": "x", "why": "x", "exp": "x"}
	_, err = SubmitStaff(ctx, pool, ids["applicant"], bad, true)
	denied("age not a number", err)
	appID, err := SubmitStaff(ctx, pool, ids["applicant"], form, true)
	must("submit", err)
	_, err = SubmitStaff(ctx, pool, ids["applicant"], form, true)
	denied("second open application", err)

	head := Actor{PlayerID: ids["head"], Source: "website"}
	must("to interview", ToInterview(ctx, pool, head, appID))
	must("save interview", SaveInterview(ctx, pool, head, appID, map[string]string{"hours": "10"}, "pass"))
	a, _ := GetStaffApp(ctx, pool, appID)
	if a.Status != "interview" || a.Interview == nil || a.Interview.Outcome != "pass" || a.Interview.Answers["hours"] != "10" {
		t.Fatalf("after interview: %+v %+v", a, a.Interview)
	}
	var rankID int
	pool.QueryRow(ctx, `SELECT id FROM staff_ranks ORDER BY level LIMIT 1`).Scan(&rankID)
	must("accept", AcceptStaff(ctx, pool, head, appID, rankID, "Support", ""))
	var staffRank *int
	var team *string
	pool.QueryRow(ctx, `SELECT staff_rank_id, staff_team FROM players WHERE id = $1`, ids["applicant"]).Scan(&staffRank, &team)
	if staffRank == nil || *staffRank != rankID || team == nil || *team != "Support" {
		t.Errorf("accepted applicant should be staff in Support: %v %v", staffRank, team)
	}
	denied("reject after accepting", RejectStaff(ctx, pool, head, appID, "x"))
	var dms int
	pool.QueryRow(ctx, `SELECT count(*) FROM discord_outbox WHERE dedupe_key LIKE $1`, "staff-app:%").Scan(&dms)
	if dms != 2 {
		t.Errorf("expected interview + decision DMs, got %d", dms)
	}

	// Faction applications: only that faction's command decides.
	must("faction apply", SubmitFaction(ctx, pool, ids["nodiscord"], "police", map[string]string{"why": "Serve", "hours": "Weekends"}))
	denied("second open faction application", SubmitFaction(ctx, pool, ids["nodiscord"], "police", map[string]string{"why": "Serve", "hours": "Weekends"}))
	denied("already in the faction", SubmitFaction(ctx, pool, ids["cmd"], "police", map[string]string{"why": "x", "hours": "x"}))
	apps, _ := FactionQueue(ctx, pool, "police")
	var fid int64
	for _, fa := range apps {
		if fa.PlayerID == ids["nodiscord"] {
			fid = fa.ID
		}
	}
	denied("non-command decides", DecideFaction(ctx, pool, head, fid, true, ""))
	denied("reject without a reason", DecideFaction(ctx, pool, Actor{PlayerID: ids["cmd"]}, fid, false, ""))
	must("command accepts", DecideFaction(ctx, pool, Actor{PlayerID: ids["cmd"], Source: "website"}, fid, true, "Welcome"))
	var cop int
	pool.QueryRow(ctx, `SELECT cop_level FROM players WHERE id = $1`, ids["nodiscord"]).Scan(&cop)
	if cop != 1 {
		t.Errorf("accepted recruit should be at level 1, got %d", cop)
	}
	var logged int
	pool.QueryRow(ctx, `SELECT count(*) FROM faction_log WHERE target_id = $1 AND kind = 'recruit' AND via = 'command'`, ids["nodiscord"]).Scan(&logged)
	if logged != 1 {
		t.Errorf("recruit should be in the Command log, got %d", logged)
	}
}
