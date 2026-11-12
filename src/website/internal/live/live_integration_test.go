package live

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPoll(t *testing.T) {
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
	var w, n int64
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000911', 'Live Watched') RETURNING id`).Scan(&w)
	pool.QueryRow(ctx, `INSERT INTO players (uid, name) VALUES ('76561190000000912', 'Live Normal') RETURNING id`).Scan(&n)
	var kase int64
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM staff_cases WHERE id = $1`, kase)
		pool.Exec(ctx, `DELETE FROM players WHERE id IN ($1, $2)`, w, n)
	})
	pool.Exec(ctx, `INSERT INTO player_watchlist (player_id, reason) VALUES ($1, 'testing live')`, w)

	h := NewHub(pool)
	first, err := h.Poll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Topic != "counts" {
		t.Fatalf("first poll should only carry counts, got %+v", first)
	}
	before := first[0].Data.(Counts)

	// Nothing changed: nothing to send.
	if again, _ := h.Poll(ctx); len(ours(again)) != 0 {
		t.Errorf("unchanged poll sent %+v", again)
	}

	pool.Exec(ctx, `INSERT INTO anti_cheat_flags (player_id, flag_type, confidence) VALUES ($1, 'dupe', 'high')`, n)
	pool.QueryRow(ctx, `INSERT INTO staff_cases (case_type, summary) VALUES ('exploit', 'Live test case') RETURNING id`).Scan(&kase)
	pool.Exec(ctx, `INSERT INTO player_sessions (player_id) VALUES ($1), ($2)`, w, n)

	msgs, err := h.Poll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]Msg{}
	for _, m := range msgs {
		got[m.Topic] = append(got[m.Topic], m)
	}
	if len(got["counts"]) != 1 {
		t.Fatalf("no counts update: %+v", msgs)
	}
	after := got["counts"][0].Data.(Counts)
	if after.OpenFlags < before.OpenFlags+1 || after.OpenCases < before.OpenCases+1 || after.Online < before.Online+2 {
		t.Errorf("counts before %+v after %+v", before, after)
	}
	find := func(topic, text string) *Msg {
		for i, m := range got[topic] {
			if strings.Contains(m.Data.(Toast).Text, text) || strings.Contains(m.Data.(Toast).Title, text) {
				return &got[topic][i]
			}
		}
		return nil
	}
	if m := find("flag", "Live Normal"); m == nil || m.Perm != "anticheat.review" || m.Data.(Toast).Level != "warning" {
		t.Errorf("flag toast: %+v", got["flag"])
	}
	if m := find("case", "Live test case"); m == nil || m.Perm != "cases.view" {
		t.Errorf("case toast: %+v", got["case"])
	}
	if m := find("watch", "Live Watched"); m == nil || !strings.Contains(m.Data.(Toast).Text, "testing live") {
		t.Errorf("watch toast: %+v", got["watch"])
	}
	if find("watch", "Live Normal") != nil {
		t.Error("an unwatched player's join was toasted")
	}
	if len(got["online"]) != 1 {
		t.Errorf("online: %+v", got["online"])
	}

	// Permissions.
	flag := *find("flag", "Live Normal")
	if Allowed(flag, map[string]bool{}, true, false) || !Allowed(flag, map[string]bool{"anticheat.review": true}, true, false) {
		t.Error("flag permission")
	}
	if Allowed(flag, map[string]bool{"anticheat.review": true}, false, true) {
		t.Error("support-only staff got an admin message")
	}
	ticket := Msg{Topic: "ticket", Perm: "-support"}
	if Allowed(ticket, map[string]bool{"cases.view": true}, true, false) || !Allowed(ticket, nil, false, true) {
		t.Error("ticket permission")
	}
	if got := string(Encode(Msg{Topic: "online", Data: map[string]int{"online": 3}})); got != "event: online\ndata: {\"online\":3}\n\n" {
		t.Errorf("encode: %q", got)
	}
}

// ours drops topic-less noise from other tests sharing the database.
func ours(ms []Msg) []Msg {
	var out []Msg
	for _, m := range ms {
		if m.Topic != "counts" && m.Topic != "online" {
			out = append(out, m)
		}
	}
	return out
}
