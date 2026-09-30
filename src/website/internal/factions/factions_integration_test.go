package factions

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runs only with TEST_DATABASE_URL, and only when no police ranks are
// configured (it sets its own and removes them). Stop the live website
// first: level changes write staff_log rows a running poster would send
// to Discord.
func TestCommandAuthorityRules(t *testing.T) {
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

	// Ranks 1-5; 4 (Inspector) can set up to 2, 5 (Supt) up to 3. Level 2 has one slot.
	if _, err := pool.Exec(ctx, `
		INSERT INTO faction_rank_names (faction, level, name, slots, promote_up_to, is_command) VALUES
		('police', 1, 'Probationer', NULL, 0, false), ('police', 2, 'Constable', 1, 0, false), ('police', 3, 'Sergeant', NULL, 2, false),
		('police', 4, 'Inspector', NULL, 2, true), ('police', 5, 'Superintendent', NULL, 3, true)`); err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{}
	for name, lvl := range map[string]int{"insp": 4, "prob": 1, "sgt": 3, "civ": 0, "staffcop": 1, "staffciv": 0} {
		var id int64
		uid := map[string]string{"insp": "76561190000000091", "prob": "76561190000000092", "sgt": "76561190000000093",
			"civ": "76561190000000094", "staffcop": "76561190000000095", "staffciv": "76561190000000096"}[name]
		if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name, cop_level) VALUES ($1, $2, $3) RETURNING id`, uid, "t_"+name, lvl).Scan(&id); err != nil {
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
		pool.Exec(ctx, `DELETE FROM staff_log WHERE target_player_id = ANY($1) OR staff_player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM rank_changes WHERE player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM players WHERE id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM faction_rank_names WHERE faction = 'police'`)
	})

	cmd := func(who string) Actor { return Actor{PlayerID: ids[who], Source: "website", Via: ViaCommand} }
	ovr := func(who string) Actor { return Actor{PlayerID: ids[who], Source: "website", Via: ViaStaffOverride} }
	denied := func(what string, err error) {
		t.Helper()
		if !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%s: expected refusal, got %v", what, err)
		}
	}
	ok := func(what string, ch Change, err error, kind string) Change {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if ch.Kind != kind {
			t.Errorf("%s: kind = %q, want %q", what, ch.Kind, kind)
		}
		return ch
	}

	if c, isCmd, _ := CommandIn(ctx, pool, ids["insp"], "police"); !isCmd || c.Authority != 2 {
		t.Fatalf("inspector should command up to 2, got %+v %v", c, isCmd)
	}
	if _, isCmd, _ := CommandIn(ctx, pool, ids["sgt"], "police"); isCmd {
		t.Error("a sergeant isn't ticked as command, so isn't command even with promote_up_to set")
	}

	ch, err := SetLevel(ctx, pool, cmd("insp"), "police", ids["prob"], 2, "passed probation")
	ok("promote within authority", ch, err, "promote")
	_, err = SetLevel(ctx, pool, cmd("insp"), "police", ids["prob"], 3, "x")
	denied("promote above authority", err)
	_, err = SetLevel(ctx, pool, cmd("insp"), "police", ids["sgt"], 1, "x")
	denied("act on someone above authority", err)
	_, err = SetLevel(ctx, pool, cmd("insp"), "police", ids["insp"], 1, "x")
	denied("change yourself", err)
	_, err = SetLevel(ctx, pool, cmd("sgt"), "police", ids["prob"], 1, "x")
	denied("non-command acting", err)
	_, err = SetLevel(ctx, pool, cmd("insp"), "police", ids["civ"], 1, "")
	denied("missing reason", err)

	ch, err = SetLevel(ctx, pool, cmd("insp"), "police", ids["civ"], 1, "interviewed")
	ok("recruit", ch, err, "recruit")
	_, err = SetLevel(ctx, pool, cmd("insp"), "police", ids["civ"], 2, "x")
	denied("rank with no open slots", err)

	// Management override: not bound by slots; flagged when the staff member is in the faction.
	ch, err = SetLevel(ctx, pool, ovr("staffcop"), "police", ids["civ"], 2, "dispute ruling")
	if ok("override past slots", ch, err, "promote"); !ch.OwnFaction {
		t.Error("override by a police member should be flagged own_faction")
	}
	ch, err = SetLevel(ctx, pool, ovr("staffciv"), "police", ids["civ"], 0, "cleanup")
	if ok("override remove", ch, err, "remove"); ch.OwnFaction {
		t.Error("override by a non-member isn't own_faction")
	}
	_, err = SetLevel(ctx, pool, ovr("staffcop"), "police", ids["staffcop"], 3, "x")
	denied("override on yourself", err)

	var cmdRows, ovrRows, own int
	pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE via = 'command'), count(*) FILTER (WHERE via = 'staff_override'),
	                           count(*) FILTER (WHERE own_faction) FROM faction_log WHERE target_id = ANY($1)`,
		[]int64{ids["prob"], ids["civ"]}).Scan(&cmdRows, &ovrRows, &own)
	if cmdRows != 2 || ovrRows != 2 || own != 1 {
		t.Errorf("faction_log: command=%d override=%d own=%d, want 2/2/1", cmdRows, ovrRows, own)
	}
	entries, err := Log(ctx, pool, "police", "", "", ids["civ"], 10)
	if err != nil || len(entries) != 3 || entries[0].Change() != "Removed (was Constable)" {
		t.Errorf("log for civ: %+v %v", entries, err)
	}
}
