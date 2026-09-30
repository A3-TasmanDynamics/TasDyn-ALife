package profile

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runs only with TEST_DATABASE_URL (a throwaway database).
func TestProfile(t *testing.T) {
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
	for name, uid := range map[string]string{"me": "76561190000000401", "other": "76561190000000402", "staff": "76561190000000403"} {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ($1, $2) RETURNING id`, uid, "t_"+name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[name] = id
	}
	all := []int64{ids["me"], ids["other"], ids["staff"]}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM banlist WHERE uid IN ('76561190000000401', '76561190000000402')`)
		pool.Exec(ctx, `DELETE FROM staff_cases WHERE lead_staff_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM support_tickets WHERE player_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM players WHERE id = ANY($1)`, all)
	})
	me := ids["me"]
	userError := func(what string, err error) {
		t.Helper()
		if !IsUserError(err) {
			t.Errorf("%s: want a user error, got %v", what, err)
		}
	}

	// Username.
	userError("too short", SetDisplayName(ctx, pool, me, "ab"))
	userError("bad characters", SetDisplayName(ctx, pool, me, "<script>"))
	userError("another player's game name", SetDisplayName(ctx, pool, me, "T_OTHER"))
	if err := SetDisplayName(ctx, pool, me, "  Sgt   Mommers "); err != nil {
		t.Fatal(err)
	}
	p, _ := Load(ctx, pool, me)
	if p.DisplayName != "Sgt Mommers" || p.Name() != "Sgt Mommers" || p.NextNameChange().IsZero() {
		t.Errorf("display name not saved as expected: %+v", p)
	}
	userError("cooldown", SetDisplayName(ctx, pool, me, "Another Name"))
	pool.Exec(ctx, `UPDATE players SET display_name_changed_at = now() - interval '8 days' WHERE id = $1`, me)
	pool.Exec(ctx, `UPDATE players SET display_name_changed_at = NULL WHERE id = $1`, ids["other"])
	if err := SetDisplayName(ctx, pool, ids["other"], "sgt mommers"); !IsUserError(err) {
		t.Errorf("taken names are unique ignoring case, got %v", err)
	}
	if err := SetDisplayName(ctx, pool, me, ""); err != nil {
		t.Fatal(err)
	}
	if p, _ = Load(ctx, pool, me); p.DisplayName != "" || p.Name() != "t_me" {
		t.Errorf("reset should fall back to the game name, got %q", p.Name())
	}

	// TeamSpeak.
	const ts = "aBcD1234eFgH5678iJkL9012mNo="
	userError("bad TeamSpeak ID", SetTeamSpeak(ctx, pool, me, "not-an-id"))
	if err := SetTeamSpeak(ctx, pool, me, " "+ts+" "); err != nil {
		t.Fatal(err)
	}
	userError("TeamSpeak linked elsewhere", SetTeamSpeak(ctx, pool, ids["other"], ts))
	if p, _ = Load(ctx, pool, me); p.TeamSpeakUID != ts || p.TeamSpeakLinked == nil {
		t.Errorf("TeamSpeak not linked: %+v", p)
	}
	if err := SetTeamSpeak(ctx, pool, me, ""); err != nil {
		t.Fatal(err)
	}
	if p, _ = Load(ctx, pool, me); p.TeamSpeakUID != "" || p.TeamSpeakLinked != nil {
		t.Errorf("TeamSpeak not unlinked: %+v", p)
	}

	// Moderation history: only this player's records, notes never loaded.
	staff := ids["staff"]
	pool.Exec(ctx, `INSERT INTO banlist (uid, player_id, reason, banned_by, expires_at, note) VALUES
		('76561190000000401', $1, 'RDM', $2, now() + interval '3 days', 'secret staff note'),
		('76561190000000401', $1, 'Old ban', $2, now() - interval '1 day', NULL),
		('76561190000000402', $3, 'Not mine', $2, NULL, NULL)`, me, staff, ids["other"])
	pool.Exec(ctx, `UPDATE banlist SET created_at = now() - interval '8 days' WHERE reason = 'Old ban' AND player_id = $1`, me)
	pool.Exec(ctx, `INSERT INTO kick_log (target_player_id, kicked_by, kick_type, reason) VALUES ($1, $2, 'manual', 'Mic spam'), ($1, NULL, 'battleye', NULL), ($3, $2, 'manual', 'x')`, me, staff, ids["other"])
	var caseID int64
	if err := pool.QueryRow(ctx, `INSERT INTO staff_cases (case_type, summary, lead_staff_id) VALUES ('rdm_vdm', 'Staff-only summary', $1) RETURNING id`, staff).Scan(&caseID); err != nil {
		t.Fatal(err)
	}
	pool.Exec(ctx, `INSERT INTO staff_case_participants (case_id, player_id, role) VALUES ($1, $2, 'subject'), ($1, $2, 'witness'), ($1, $3, 'assisting_staff')`, caseID, me, staff)
	pool.Exec(ctx, `INSERT INTO punishment_points (case_id, player_id, points, rules, issued_by) VALUES ($1, $2, 5, '2.1 RDM', $3), ($1, $2, 3, '4.2', $3)`, caseID, me, staff)
	pool.Exec(ctx, `UPDATE punishment_points SET revoked_at = now() WHERE player_id = $1 AND points = 3`, me)
	var tk int64
	pool.QueryRow(ctx, `INSERT INTO support_tickets (player_id, subject, category_id) VALUES ($1, 'Help', (SELECT id FROM ticket_categories ORDER BY id LIMIT 1)) RETURNING id`, me).Scan(&tk)
	pool.Exec(ctx, `INSERT INTO support_ticket_messages (ticket_id, author_player_id, body, internal) VALUES ($1, $2, 'hi', false), ($1, $3, 'staff note', true)`, tk, me, staff)

	h, err := Moderation(ctx, pool, me)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Bans) != 2 || h.Bans[0].Reason != "RDM" || h.Bans[0].State() != "Active" || h.Bans[0].Length() != "3 days" || h.Bans[1].State() != "Expired" {
		t.Errorf("bans: %+v", h.Bans)
	}
	if len(h.Kicks) != 2 {
		t.Errorf("kicks: %+v", h.Kicks)
	} else if h.Kicks[0].By != "System" && h.Kicks[1].By != "System" {
		t.Errorf("a system kick shows as System: %+v", h.Kicks)
	}
	if len(h.Points) != 2 || h.ActivePoints != 5 {
		t.Errorf("points: %d active from %+v", h.ActivePoints, h.Points)
	}
	if len(h.Cases) != 1 || h.Cases[0].Role != "subject" {
		t.Errorf("cases: one case, as subject: %+v", h.Cases)
	}
	if len(h.Tickets) != 1 || h.Tickets[0].Replies != 1 || h.OpenTickets != 1 {
		t.Errorf("tickets (internal notes not counted): %+v", h.Tickets)
	}
	if h.Empty() {
		t.Error("history isn't empty")
	}
	if o, _ := Moderation(ctx, pool, staff); !o.Empty() {
		t.Errorf("staff's own history should be empty (assisting a case isn't a record): %+v", o)
	}
}
