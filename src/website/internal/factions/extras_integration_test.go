package factions

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Probation, discipline, divisions & quals and rank rules, on EMS. Runs
// only with TEST_DATABASE_URL and only when no EMS ranks are configured (it
// sets its own and removes them, with its test quals and divisions). Stop
// the live website first: level changes write staff_log rows a running
// poster would send to Discord.
func TestCommandExtras(t *testing.T) {
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
	pool.QueryRow(ctx, `SELECT count(*) FROM faction_rank_names WHERE faction = 'ems'`).Scan(&n)
	if n > 0 {
		t.Skip("EMS ranks already configured on this database")
	}
	const f = "ems"
	// 1 Trainee, 2 Paramedic, 3 Senior (needs TQ), 4 Supervisor (command, up to 2), 5 Chief (cabinet, up to 4, top).
	if _, err := pool.Exec(ctx, `
		INSERT INTO faction_rank_names (faction, level, name, promote_up_to, min_days, required_quals, is_command, is_cabinet) VALUES
		('ems', 1, 'Trainee', 0, 0, '{}', false, false), ('ems', 2, 'Paramedic', 0, 0, '{}', false, false), ('ems', 3, 'Senior Paramedic', 0, 0, '{TQ}', false, false),
		('ems', 4, 'Supervisor', 2, 0, '{}', true, false), ('ems', 5, 'Chief', 4, 0, '{}', true, true)`); err != nil {
		t.Fatal(err)
	}
	pool.Exec(ctx, `INSERT INTO faction_quals (faction, key, name) VALUES ('ems', 'TQ', 'Test qual'), ('ems', 'AQ', 'Air qual')`)
	pool.Exec(ctx, `INSERT INTO faction_divisions (faction, key, name, roles, required_qual) VALUES ('ems', 'TAIR', 'Test Air', '{Lead,Pilot}', 'AQ')`)
	ids := map[string]int64{}
	for name, lvl := range map[string]int{"chief": 5, "sup": 4, "para": 2, "para2": 2, "civ": 0, "civ2": 0, "clerk": 1, "clerk2": 1} {
		uid := map[string]string{"chief": "76561190000000181", "sup": "76561190000000182", "para": "76561190000000183",
			"para2": "76561190000000184", "civ": "76561190000000185", "civ2": "76561190000000186",
			"clerk": "76561190000000187", "clerk2": "76561190000000188"}[name]
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name, medic_level) VALUES ($1, $2, $3) RETURNING id`, uid, "t_"+name, lvl).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[name] = id
	}
	t.Cleanup(func() {
		all := []int64{}
		for _, id := range ids {
			all = append(all, id)
		}
		pool.Exec(ctx, `DELETE FROM faction_log WHERE target_id = ANY($1) OR actor_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM faction_discharges WHERE player_id = ANY($1) OR by_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM faction_suspensions WHERE player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM faction_discipline WHERE player_id = ANY($1) AND corrects_id IS NOT NULL`, all)
		pool.Exec(ctx, `DELETE FROM faction_discipline WHERE player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM staff_log WHERE target_player_id = ANY($1) OR staff_player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM rank_changes WHERE player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM players WHERE id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM faction_divisions WHERE faction = 'ems' AND key = 'TAIR'`)
		pool.Exec(ctx, `DELETE FROM faction_quals WHERE faction = 'ems' AND key IN ('TQ', 'AQ')`)
		pool.Exec(ctx, `DELETE FROM faction_rank_names WHERE faction = 'ems'`)
	})
	cmd := func(who string) Actor { return Actor{PlayerID: ids[who], Source: "website", Via: ViaCommand} }
	must := func(what string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	denied := func(what string, err error) {
		t.Helper()
		if !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%s: expected refusal, got %v", what, err)
		}
	}

	// Recruiting starts a probation; confirming needs every item passed.
	_, err = SetLevel(ctx, pool, cmd("sup"), f, ids["civ"], 1, "interviewed")
	must("recruit", err)
	ps, err := Probations(ctx, pool, f, true)
	must("probations", err)
	var prob Probation
	for _, p := range ps {
		if p.PlayerID == ids["civ"] {
			prob = p
		}
	}
	if prob.ID == 0 || len(prob.Results) == 0 {
		t.Fatalf("recruit should be on probation with a training sheet, got %+v", prob)
	}
	_, err = ConfirmProbation(ctx, pool, cmd("sup"), f, prob.ID)
	denied("confirm before training passes", err)
	must("fail theory", SetTraining(ctx, pool, cmd("sup"), f, prob.ID, "theory", "fail"))
	denied("pass theory inside the retake wait", SetTraining(ctx, pool, cmd("sup"), f, prob.ID, "theory", "pass"))
	pool.Exec(ctx, `UPDATE faction_training_results SET created_at = now() - interval '25 hours' WHERE probation_id = $1`, prob.ID)
	for _, r := range prob.Results {
		must("pass "+r.Item.Key, SetTraining(ctx, pool, cmd("sup"), f, prob.ID, r.Item.Key, "pass"))
	}
	denied("FTO ranked at or below the recruit", UpdateProbation(ctx, pool, cmd("sup"), f, prob.ID, ids["civ"], ""))
	must("set FTO", UpdateProbation(ctx, pool, cmd("sup"), f, prob.ID, ids["para"], "ride-along booked"))
	ch, err := ConfirmProbation(ctx, pool, cmd("sup"), f, prob.ID)
	must("confirm", err)
	if ch.To != 2 {
		t.Errorf("confirm should promote one rank, got %+v", ch)
	}

	// Rank rules: the new rank's quals bind command promotions.
	_, err = SetLevel(ctx, pool, cmd("chief"), f, ids["para"], 3, "x")
	denied("promote without the rank's qual", err)
	must("grant TQ", SetQual(ctx, pool, cmd("chief"), f, ids["para"], "TQ", true, ""))
	_, err = SetLevel(ctx, pool, cmd("chief"), f, ids["para"], 3, "ready")
	must("promote with the qual", err)
	denied("grant a qual to someone ranked above you", SetQual(ctx, pool, cmd("sup"), f, ids["chief"], "TQ", true, ""))

	// Divisions need the division's qual.
	denied("join a division without its qual", SetDivision(ctx, pool, cmd("sup"), f, ids["para2"], "TAIR", "", false, ""))
	must("grant AQ", SetQual(ctx, pool, cmd("sup"), f, ids["para2"], "AQ", true, ""))
	must("join division", SetDivision(ctx, pool, cmd("sup"), f, ids["para2"], "TAIR", "", false, ""))
	must("change role", SetDivision(ctx, pool, cmd("sup"), f, ids["para2"], "TAIR", "Lead", false, ""))
	denied("unknown role", SetDivision(ctx, pool, cmd("sup"), f, ids["para2"], "TAIR", "Captain", false, ""))

	// Discipline: offence ranges, the ladder, applied actions, warnings.
	var mvwOff, smallOff, bigOff int64
	pool.QueryRow(ctx, `SELECT id FROM faction_offences WHERE faction = 'ems' AND name = 'Incorrect name layout'`).Scan(&mvwOff)
	pool.QueryRow(ctx, `SELECT id FROM faction_offences WHERE faction = 'ems' AND name = 'Unprofessional communication'`).Scan(&smallOff)
	pool.QueryRow(ctx, `SELECT id FROM faction_offences WHERE faction = 'ems' AND name = 'Abuse of power'`).Scan(&bigOff)
	if mvwOff == 0 || smallOff == 0 || bigOff == 0 {
		t.Fatal("seeded EMS offences missing")
	}
	_, err = IssueDiscipline(ctx, pool, cmd("sup"), f, Issue{TargetID: ids["para2"], OffenceID: smallOff, Points: 30, Notes: "x"})
	denied("points above the offence's range", err)
	_, err = IssueDiscipline(ctx, pool, cmd("sup"), f, Issue{TargetID: ids["para2"], OffenceID: smallOff, MVW: true, Notes: "x"})
	denied("warning for an offence that doesn't allow one", err)
	_, err = IssueDiscipline(ctx, pool, cmd("sup"), f, Issue{TargetID: ids["chief"], OffenceID: smallOff, Points: 5, Notes: "x"})
	denied("discipline someone ranked above you", err)
	_, err = IssueDiscipline(ctx, pool, cmd("sup"), f, Issue{TargetID: ids["para2"], OffenceID: smallOff, Points: 5, Notes: ""})
	denied("no notes", err)

	for i := 0; i < 2; i++ {
		res, err := IssueDiscipline(ctx, pool, cmd("sup"), f, Issue{TargetID: ids["para2"], OffenceID: mvwOff, MVW: true, Notes: "name"})
		must("warning", err)
		if res.Converted {
			t.Error("fewer than three warnings shouldn't convert")
		}
	}
	res, err := IssueDiscipline(ctx, pool, cmd("sup"), f, Issue{TargetID: ids["para2"], OffenceID: mvwOff, MVW: true, Notes: "name", ApplyAction: true})
	must("third warning", err)
	if !res.Converted || res.After != 10 || res.Crossed == nil || res.Crossed.At != 10 {
		t.Errorf("third warning should become 10 points and cross 10, got %+v", res)
	}
	st, _ := StandingOf(ctx, pool, f, ids["para2"])
	if st.Points != 10 || st.MVWs != 0 || st.Suspended == nil {
		t.Errorf("after conversion: want 10 points, 0 warnings, suspended; got %+v", st)
	}

	// Correcting an entry cancels its points and ends its suspension.
	log, _ := DisciplineLog(ctx, pool, f, ids["para2"], 10)
	var mvwID int64
	for _, e := range log {
		if e.Kind == "mvw" {
			mvwID = e.ID
		}
	}
	must("correct a warning", CorrectDiscipline(ctx, pool, cmd("sup"), f, mvwID, "wrong officer"))
	denied("correct twice", CorrectDiscipline(ctx, pool, cmd("sup"), f, mvwID, "again"))

	// 50 points: termination and blacklist, applied.
	res, err = IssueDiscipline(ctx, pool, cmd("sup"), f, Issue{TargetID: ids["para2"], OffenceID: bigOff, Points: 50, Notes: "abuse", ApplyAction: true})
	must("50 points", err)
	if res.Crossed == nil || !res.Crossed.Blacklist || res.Warning != "" {
		t.Errorf("50 points should apply termination and blacklist, got %+v", res)
	}
	if l, _ := levelOf(ctx, pool, f, ids["para2"]); l != 0 {
		t.Errorf("terminated member should be out of EMS, level %d", l)
	}
	if bl, _ := ActiveBlacklist(ctx, pool, f, ids["para2"]); bl == nil {
		t.Error("terminated member should be blacklisted")
	}
	_, err = SetLevel(ctx, pool, cmd("sup"), f, ids["para2"], 1, "rehire")
	denied("recruit someone blacklisted", err)
	var discharges int
	pool.QueryRow(ctx, `SELECT count(*) FROM faction_discharges WHERE player_id = $1 AND type = 'contract_termination'`, ids["para2"]).Scan(&discharges)
	if discharges != 1 {
		t.Errorf("termination should record one discharge, got %d", discharges)
	}

	// Blacklist lift and manual blacklist.
	bls, _ := Blacklist(ctx, pool, f, 10)
	denied("non-cabinet lifts a blacklist", LiftBlacklist(ctx, pool, cmd("sup"), f, bls[0].ID, "appeal upheld"))
	must("cabinet lifts a blacklist", LiftBlacklist(ctx, pool, cmd("chief"), f, bls[0].ID, "appeal upheld"))
	denied("blacklist a member", AddBlacklist(ctx, pool, cmd("sup"), f, ids["chief"], 30, "x"))
	denied("non-cabinet blacklists permanently", AddBlacklist(ctx, pool, cmd("sup"), f, ids["civ2"], 0, "leaked channels"))
	must("cabinet blacklists permanently", AddBlacklist(ctx, pool, cmd("chief"), f, ids["civ2"], 0, "leaked channels"))
	denied("blacklist twice", AddBlacklist(ctx, pool, cmd("sup"), f, ids["civ2"], 30, "x"))

	// Discharge.
	must("discharge", Discharge(ctx, pool, cmd("chief"), f, ids["para"], "resigned", "moving to police"))
	if l, _ := levelOf(ctx, pool, f, ids["para"]); l != 0 {
		t.Error("discharged member should be out")
	}

	// Rank rules: anyone who maintains the panel, for ranks other than their
	// own; only cabinet (or Management) changes the rank type and "promotes
	// up to", or edits their own rank; nobody unticks their own rank.
	rr := RankRules{Name: "Paramedic II", Slots: 4, MinDays: 7, Quals: []string{"TQ"}}
	must("command edits another rank's rules", UpdateRank(ctx, pool, cmd("sup"), f, 2, RankRules{Name: "Paramedic", MinDays: 3}))
	denied("command changes promotes-up-to", UpdateRank(ctx, pool, cmd("sup"), f, 2, RankRules{Name: "Paramedic", PromoteUpTo: 1}))
	denied("command ticks a rank as command", UpdateRank(ctx, pool, cmd("sup"), f, 2, RankRules{Name: "Paramedic", Command: true}))
	denied("command edits own rank", UpdateRank(ctx, pool, cmd("sup"), f, 4, RankRules{Name: "Sup", PromoteUpTo: 2, Command: true}))
	denied("cabinet unticks own rank", UpdateRank(ctx, pool, cmd("chief"), f, 5, RankRules{Name: "Boss", PromoteUpTo: 4}))
	must("cabinet edits own rank", UpdateRank(ctx, pool, cmd("chief"), f, 5, RankRules{Name: "Chief Paramedic", PromoteUpTo: 4, Command: true, Cabinet: true}))
	denied("unknown qual", UpdateRank(ctx, pool, cmd("chief"), f, 2, RankRules{Name: "x", Quals: []string{"ZZ"}}))
	denied("cabinet without command", UpdateRank(ctx, pool, cmd("chief"), f, 2, RankRules{Name: "x", Cabinet: true}))

	// Command comes from the tick: unticking Supervisor takes the panel away.
	must("untick command", UpdateRank(ctx, pool, cmd("chief"), f, 4, RankRules{Name: "Supervisor", PromoteUpTo: 2}))
	if _, isCmd, _ := CommandIn(ctx, pool, ids["sup"], f); isCmd {
		t.Error("an unticked rank shouldn't have command")
	}
	must("tick command, no promotion", UpdateRank(ctx, pool, cmd("chief"), f, 4, RankRules{Name: "Supervisor", Command: true}))
	if c, isCmd, _ := CommandIn(ctx, pool, ids["sup"], f); !isCmd || c.Authority != 0 {
		t.Errorf("a command rank with no promotion range: want command, authority 0; got %v %+v", isCmd, c)
	}
	_, err = SetLevel(ctx, pool, cmd("sup"), f, ids["civ"], 1, "x")
	denied("command rank that can't change ranks", err)
	must("cabinet edits rules", UpdateRank(ctx, pool, cmd("chief"), f, 2, rr))
	ranks, _ := Ranks(ctx, pool, f)
	if r := RankFor(ranks, 2); r.Name != "Paramedic II" || r.MinDays != 7 || len(r.Quals) != 1 {
		t.Errorf("rank rules not saved: %+v", r)
	}
	must("command maintains settings", UpdateSettings(ctx, pool, cmd("sup"), f, Settings{ProbationDays: 10, PointsExpiryDays: 60, MVWDays: 7, BlacklistDays: 90}, ""))

	// Administration division: panel access whatever the rank; records and
	// maintenance, not rank changes or discipline. Only cabinet, Management
	// and the Administration Commander appoint to it.
	mgmt := Actor{PlayerID: ids["civ2"], Source: "website", Via: ViaStaffOverride}
	if _, isCmd, _ := CommandIn(ctx, pool, ids["clerk"], f); isCmd {
		t.Error("a trainee outside Administration has no panel access")
	}
	denied("command appoints to Administration", SetDivision(ctx, pool, cmd("sup"), f, ids["clerk"], "ADMIN", "Administrator", false, ""))
	must("cabinet appoints to Administration", SetDivision(ctx, pool, cmd("chief"), f, ids["clerk"], "ADMIN", "Administrator", false, ""))
	if c, isCmd, _ := CommandIn(ctx, pool, ids["clerk"], f); !isCmd || !c.Admin || c.IsCommand() || c.AdminCommander {
		t.Errorf("Administration member: want panel access as Admin, not command; got %v %+v", isCmd, c)
	}
	must("Administration keeps records", func() error {
		_, err := UpdateMember(ctx, pool, cmd("clerk"), f, ids["sup"], MemberDetails{Badge: "S01", Status: "active"})
		return err
	}())
	_, err = UpdateMember(ctx, pool, cmd("clerk"), f, ids["chief"], MemberDetails{Status: "active"})
	denied("Administration edits a cabinet member", err)
	must("Administration maintains settings", UpdateSettings(ctx, pool, cmd("clerk"), f, Settings{ProbationDays: 12, PointsExpiryDays: 60, MVWDays: 7, BlacklistDays: 90}, ""))
	must("Administration edits rank rules", UpdateRank(ctx, pool, cmd("clerk"), f, 2, RankRules{Name: "Paramedic II", Slots: 5, MinDays: 7, Quals: []string{"TQ"}}))
	_, err = IssueDiscipline(ctx, pool, cmd("clerk"), f, Issue{TargetID: ids["clerk2"], OffenceID: smallOff, Points: 5, Notes: "x"})
	denied("Administration issues discipline", err)
	_, err = SetLevel(ctx, pool, cmd("clerk"), f, ids["clerk2"], 2, "x")
	denied("Administration changes a rank", err)
	denied("Administrator appoints to Administration", SetDivision(ctx, pool, cmd("clerk"), f, ids["clerk2"], "ADMIN", "Administrator", false, ""))
	must("cabinet makes them Commander", SetDivision(ctx, pool, cmd("chief"), f, ids["clerk"], "ADMIN", "Commander", false, ""))
	must("Administration Commander appoints", SetDivision(ctx, pool, cmd("clerk"), f, ids["clerk2"], "ADMIN", "Administrator", false, ""))
	denied("Administration Commander appoints a Commander", SetDivision(ctx, pool, cmd("clerk"), f, ids["clerk2"], "ADMIN", "Commander", false, ""))
	denied("Administration Commander removes themselves", SetDivision(ctx, pool, cmd("clerk"), f, ids["clerk"], "ADMIN", "", true, ""))
	must("Management removes from Administration", SetDivision(ctx, pool, mgmt, f, ids["clerk2"], "ADMIN", "", true, ""))
	must("Management appoints to Administration", SetDivision(ctx, pool, mgmt, f, ids["clerk2"], "ADMIN", "Deputy Commander", false, ""))
	// Administration sits alongside a specialist division.
	must("grant AQ to clerk", SetQual(ctx, pool, cmd("chief"), f, ids["clerk"], "AQ", true, ""))
	must("clerk joins a specialist division", SetDivision(ctx, pool, cmd("chief"), f, ids["clerk"], "TAIR", "", false, ""))
	// Cabinet is part of Administration automatically, and can't be removed from it.
	if c, _, _ := CommandIn(ctx, pool, ids["chief"], f); !c.Admin || c.AdminRole != CabinetRole {
		t.Errorf("cabinet should be in Administration automatically, got %+v", c)
	}
	denied("remove cabinet from Administration", SetDivision(ctx, pool, mgmt, f, ids["chief"], "ADMIN", "", true, ""))
	spec, _ := MemberDivisions(ctx, pool, f)
	adm, _ := AdminPostings(ctx, pool, f)
	if p := adm[ids["chief"]]; !p.Auto || p.Role != CabinetRole {
		t.Errorf("cabinet should be listed in Administration automatically, got %+v", p)
	}
	if _, ok := adm[ids["sup"]]; ok {
		t.Error("non-cabinet command isn't in Administration unless appointed")
	}
	if spec[ids["clerk"]].Key != "TAIR" || adm[ids["clerk"]].Role != "Commander" {
		t.Errorf("Administration and a specialist division should both hold: %+v %+v", spec[ids["clerk"]], adm[ids["clerk"]])
	}

	// Personnel roster: details and roll call, for members ranked below you.
	now := time.Now()
	var enrolled *time.Time
	pool.QueryRow(ctx, `SELECT enrolled_on FROM faction_members WHERE faction = 'ems' AND player_id = $1`, ids["civ"]).Scan(&enrolled)
	if enrolled == nil {
		t.Fatal("recruiting should record the enrollment date")
	}
	md := MemberDetails{Badge: "Z21B", Region: "NZ", Status: "loa", Notes: "back in Nov", Enrolled: enrolled.Format("2006-01-02")}
	changed, err2 := UpdateMember(ctx, pool, cmd("sup"), f, ids["civ"], md)
	must("update personnel file", err2)
	if len(changed) == 0 {
		t.Error("personnel file update should report changes")
	}
	if again, _ := UpdateMember(ctx, pool, cmd("sup"), f, ids["civ"], md); len(again) != 0 {
		t.Errorf("saving the same file again should change nothing, got %v", again)
	}
	_, err = UpdateMember(ctx, pool, cmd("sup"), f, ids["chief"], md)
	denied("edit someone ranked above you", err)
	_, err = UpdateMember(ctx, pool, cmd("sup"), f, ids["civ"], MemberDetails{Region: "Mars", Status: "active", Enrolled: md.Enrolled})
	denied("unknown region", err)
	must("roll call", SetRollCall(ctx, pool, cmd("sup"), f, ids["civ"], now, "present"))
	denied("same roll call mark twice", SetRollCall(ctx, pool, cmd("sup"), f, ids["civ"], now, "present"))
	denied("roll call for a future month", SetRollCall(ctx, pool, cmd("sup"), f, ids["civ"], now.AddDate(0, 2, 0), "present"))
	denied("roll call for someone ranked above you", SetRollCall(ctx, pool, cmd("sup"), f, ids["chief"], now, "present"))
	hist, err := RollCallHistory(ctx, pool, f, ids["civ"], 6)
	must("roll call history", err)
	if len(hist) != 6 || hist[0].Mark != "present" || hist[1].Mark != "" {
		t.Errorf("roll call history: want 6 months, this one present; got %+v", hist)
	}
	roster, err := PersonnelRoster(ctx, pool, f, now)
	must("personnel roster", err)
	var row Personnel
	for _, p := range roster {
		if p.ID == ids["civ"] {
			row = p
		}
	}
	if row.Badge != "Z21B" || row.Region != "NZ" || row.Shown != "LOA" || row.RollCall != "present" || row.Enrolled == nil {
		t.Errorf("roster row not as saved: %+v", row)
	}
	pool.Exec(ctx, `DELETE FROM faction_roll_call WHERE player_id = ANY($1)`, []int64{ids["civ"]})
	pool.Exec(ctx, `DELETE FROM faction_members WHERE player_id = ANY($1)`, []int64{ids["civ"], ids["para"], ids["para2"]})

	var kinds int
	pool.QueryRow(ctx, `SELECT count(DISTINCT kind) FROM faction_log WHERE faction = 'ems' AND actor_id = ANY($1)`,
		[]int64{ids["chief"], ids["sup"]}).Scan(&kinds)
	if kinds < 8 {
		t.Errorf("expected the command log to record many kinds of action, got %d", kinds)
	}
}
