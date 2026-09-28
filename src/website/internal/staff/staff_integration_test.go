package staff

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
)

// Runs only with TEST_DATABASE_URL. It commits real rank changes, which the
// change-capture trigger turns into staff_log rows -- so it must NOT run
// while a website with a live #staff-log webhook is running against the
// same database, and it deletes its staff_log rows before finishing.
func TestStaffRules(t *testing.T) {
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

	var ids []int64
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM staff_log WHERE target_player_id = ANY($1) OR staff_player_id = ANY($1)`, ids)
		pool.Exec(ctx, `DELETE FROM players WHERE id = ANY($1)`, ids)
	})
	rank := func(key string) int {
		var id int
		if err := pool.QueryRow(ctx, `SELECT id FROM staff_ranks WHERE key = $1`, key).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	mk := func(uid, rankKey string) int64 {
		var id int64
		err := pool.QueryRow(ctx, `
			INSERT INTO players (uid, staff_rank_id) VALUES ($1, (SELECT id FROM staff_ranks WHERE key = NULLIF($2, '')))
			RETURNING id`, uid, rankKey).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		return id
	}
	head := mk("76561190000000011", "head_admin")
	head2 := mk("76561190000000012", "head_admin")
	admin := mk("76561190000000013", "admin")
	mod := mk("76561190000000014", "moderator")
	civ := mk("76561190000000015", "")

	web := func(id int64) Actor { return Actor{PlayerID: id, Source: audit.SourceWebsite} }
	denied := func(name string, err error) {
		t.Helper()
		if !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%s: expected ErrNotAllowed, got %v", name, err)
		}
	}
	ok := func(name string, err error) {
		t.Helper()
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	denied("own rank", SetRank(ctx, pool, web(admin), admin, rank("head_admin"), "self"))
	denied("no reason", SetRank(ctx, pool, web(admin), mod, rank("trial_mod"), "  "))
	denied("admin assigns admin", SetRank(ctx, pool, web(admin), civ, rank("admin"), "equal rank"))
	denied("mod changes admin", SetRank(ctx, pool, web(mod), admin, rank("trial_mod"), "outranked"))
	ok("admin promotes civ to moderator", SetRank(ctx, pool, web(admin), civ, rank("moderator"), "application accepted"))
	ok("head appoints head", SetRank(ctx, pool, web(head), admin, rank("head_admin"), "promotion"))
	ok("head demotes head", SetRank(ctx, pool, web(head2), admin, rank("admin"), "back to admin"))

	past := time.Now().Add(-time.Hour)
	denied("LOA ending in the past", SetStatus(ctx, pool, web(admin), mod, "loa", "holiday", &past))
	denied("status of non-staff", SetStatus(ctx, pool, web(admin), head2+1000000, "loa", "x", nil))
	ok("admin puts mod on LOA", SetStatus(ctx, pool, web(admin), mod, "loa", "holiday", nil))

	// Expire that LOA and let the sweep end it.
	pool.Exec(ctx, `UPDATE players SET staff_status_until = now() - interval '1 minute' WHERE id = $1`, mod)
	if _, err := EndExpiredLOAs(ctx, pool); err != nil {
		t.Fatal(err)
	}
	m, err := Get(ctx, pool, mod)
	if err != nil || m.Status != "active" {
		t.Errorf("LOA sweep: status=%q err=%v", m.Status, err)
	}

	ok("team", SetTeam(ctx, pool, web(admin), mod, "Moderation", "AU-East"))
	ok("note", AddNote(ctx, pool, web(admin), mod, "note", "Great first week"))
	denied("note about self", AddNote(ctx, pool, web(admin), admin, "note", "me"))
	ok("remove", SetRank(ctx, pool, web(admin), mod, 0, "stepped down"))
	m, _ = Get(ctx, pool, mod)
	if m.RankID != 0 || m.Team != "" || m.Status != "active" {
		t.Errorf("remove should clear rank/team/status: %+v", m)
	}

	// The history for the promoted civilian is attributed to the admin.
	h, err := History(ctx, pool, civ)
	if err != nil || len(h) != 1 || h[0].New != "Moderator" || h[0].Source != "website" || h[0].Reason != "application accepted" {
		t.Errorf("history: %+v err=%v", h, err)
	}
	notes, _ := Notes(ctx, pool, mod)
	if len(notes) != 1 {
		t.Errorf("notes: %+v", notes)
	}
}
