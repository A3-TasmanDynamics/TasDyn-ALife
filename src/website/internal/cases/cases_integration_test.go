package cases

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runs only with TEST_DATABASE_URL. Stop the live website first: these
// write staff_log rows a running poster would send to Discord.
func TestModerationFlow(t *testing.T) {
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
	for name, uid := range map[string]string{"mod": "76561190000000301", "helper": "76561190000000302", "suspect": "76561190000000303", "other": "76561190000000304"} {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ($1, $2) RETURNING id`, uid, "t_"+name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[name] = id
	}
	pool.Exec(ctx, `UPDATE players SET staff_rank_id = (SELECT id FROM staff_ranks ORDER BY level LIMIT 1) WHERE id = $1`, ids["helper"])
	var flagID int64
	if err := pool.QueryRow(ctx, `INSERT INTO anti_cheat_flags (player_id, flag_type, confidence, details) VALUES ($1, 'movement', 'high', '{"speed_kmh": 900}') RETURNING id`, ids["suspect"]).Scan(&flagID); err != nil {
		t.Fatal(err)
	}
	all := []int64{ids["mod"], ids["helper"], ids["suspect"], ids["other"]}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM ban_appeals WHERE player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM banlist WHERE player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM staff_cases WHERE lead_staff_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM staff_log WHERE staff_player_id = ANY($1) OR target_player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM rank_changes WHERE player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM anti_cheat_flags WHERE player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM players WHERE id = ANY($1)`, all)
	})

	mod := Actor{PlayerID: ids["mod"], Source: "website"}
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

	_, err = Open(ctx, pool, mod, "exploit", "x", []int64{ids["suspect"]}, 0, "")
	denied("summary too short", err)
	_, err = Open(ctx, pool, mod, "exploit", "Self case", []int64{ids["mod"]}, 0, "")
	denied("case about yourself", err)
	caseID, err := Open(ctx, pool, mod, "cheating", "Teleporting across Altis", []int64{ids["suspect"]}, flagID, "Saw it live.")
	must("open from flag", err)
	if f, _ := GetFlag(ctx, pool, flagID); f.Resolution != "case" || f.CaseID != caseID {
		t.Errorf("flag should be handled in the case: %+v", f)
	}
	denied("review a flag that's in a case", ReviewFlag(ctx, pool, mod, flagID, "dismiss", "no"))

	must("note", AddNote(ctx, pool, mod, caseID, "Checked the logs.", 0))
	must("correction", AddNote(ctx, pool, mod, caseID, "Actually the server logs, not client.", 3))
	denied("correct a missing entry", AddNote(ctx, pool, mod, caseID, "x", 99))
	entries, _ := Entries(ctx, pool, caseID)
	if len(entries) != 4 || entries[3].Corrects != 3 || len(entries[2].CorrectedBy) != 1 {
		t.Fatalf("entries: %+v", entries)
	}

	denied("non-staff as assisting", AddParticipant(ctx, pool, mod, caseID, ids["other"], "assisting_staff"))
	must("assisting staff", AddParticipant(ctx, pool, mod, caseID, ids["helper"], "assisting_staff"))
	must("related player", AddParticipant(ctx, pool, mod, caseID, ids["other"], "related"))

	denied("points to someone not on the case", IssuePoints(ctx, pool, mod, caseID, ids["helper"], 3, "2.1", "", nil))
	future := time.Now().AddDate(0, 1, 0)
	must("points", IssuePoints(ctx, pool, mod, caseID, ids["suspect"], 5, "1.4", "Teleport", &future))
	pts, total, _ := PlayerPoints(ctx, pool, ids["suspect"])
	if total != 5 || len(pts) != 1 {
		t.Fatalf("points: total %d, %+v", total, pts)
	}
	must("revoke", RevokePoints(ctx, pool, mod, pts[0].ID, "Wrong rule"))
	denied("revoke twice", RevokePoints(ctx, pool, mod, pts[0].ID, "again"))
	if _, total, _ = PlayerPoints(ctx, pool, ids["suspect"]); total != 0 {
		t.Errorf("revoked points still count: %d", total)
	}

	banID, _, err := IssueBan(ctx, pool, mod, caseID, ids["suspect"], "Cheating", 7, false, "")
	must("ban", err)
	_, _, err = IssueBan(ctx, pool, mod, caseID, ids["suspect"], "Again", 0, false, "")
	denied("double ban", err)
	ban, _, _ := ActiveBan(ctx, pool, ids["suspect"])
	if ban == nil || ban.ID != banID || ban.ExpiresAt == nil {
		t.Fatalf("active ban: %+v", ban)
	}

	denied("appeal too short", SubmitAppeal(ctx, pool, ids["suspect"], "sorry"))
	must("appeal", SubmitAppeal(ctx, pool, ids["suspect"], "It was a desync, please check the server logs again."))
	denied("second appeal", SubmitAppeal(ctx, pool, ids["suspect"], "It was a desync, please check the server logs again."))
	apps, _ := Appeals(ctx, pool)
	var appID int64
	for _, a := range apps {
		if a.PlayerID == ids["suspect"] {
			appID = a.ID
		}
	}
	if appID == 0 {
		t.Fatal("appeal not in the queue")
	}
	_, err = DecideAppeal(ctx, pool, mod, appID, true, "Logs confirm a desync.")
	must("accept appeal", err)
	if ban, _, _ := ActiveBan(ctx, pool, ids["suspect"]); ban != nil {
		t.Error("accepting the appeal should lift the ban")
	}
	_, err = LiftBan(ctx, pool, mod, banID, "again")
	denied("lift an ended ban", err)

	must("close", SetStatus(ctx, pool, mod, caseID, "closed", "Appeal accepted", "Resolved"))
	denied("close twice", SetStatus(ctx, pool, mod, caseID, "closed", "", "x"))
	_, _, err = IssueBan(ctx, pool, mod, caseID, ids["suspect"], "x", 1, false, "")
	denied("ban from a closed case", err)

	rows, err := List(ctx, pool, Filter{Q: "t_suspect"})
	if err != nil || len(rows) != 1 || rows[0].Outcome != "Appeal accepted" {
		t.Errorf("list by participant name: %+v %v", rows, err)
	}
	if act, _ := StaffActivity(ctx, pool, ids["helper"]); act.AssistAll != 1 {
		t.Errorf("helper activity: %+v", act)
	}
	var logged int
	pool.QueryRow(ctx, `SELECT count(*) FROM staff_log WHERE staff_player_id = $1`, ids["mod"]).Scan(&logged)
	if logged < 6 {
		t.Errorf("expected every action in staff_log, got %d rows", logged)
	}
}
