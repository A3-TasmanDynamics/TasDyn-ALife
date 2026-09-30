package dbbrowser

import "testing"

func TestCheck(t *testing.T) {
	ok := []string{
		"SELECT id, name, created_at, updated_at FROM players WHERE cop_level > 0 ORDER BY civ_cash DESC LIMIT 50;",
		"-- richest\nSELECT * FROM bank_accounts",
		"WITH x AS (SELECT 1) SELECT * FROM x",
		"select locked from houses",
		"EXPLAIN SELECT * FROM players",
	}
	for _, q := range ok {
		if why := Check(q, false); why != "" {
			t.Errorf("should run: %q (%s)", q, why)
		}
	}
	refused := []string{
		"", "DELETE FROM players", "SELECT 1; DROP TABLE players", "UPDATE players SET name = 'x'",
		"SELECT pg_terminate_backend(pid) FROM pg_stat_activity", "SELECT pg_sleep(60)",
		"WITH d AS (DELETE FROM banlist RETURNING *) SELECT * FROM d", "SELECT set_config('x', 'y', false)",
		"SELECT token_hash FROM web_sessions", "EXPLAIN ANALYZE SELECT 1", "SELECT nextval('players_id_seq')",
		"/* sneaky */ TRUNCATE players",
	}
	for _, q := range refused {
		if why := Check(q, false); why == "" {
			t.Errorf("should be refused: %q", q)
		}
	}
	// With a dedicated read-only role the database enforces it; only the
	// shape checks remain.
	if why := Check("SELECT token_hash FROM web_sessions", true); why != "" {
		t.Errorf("dedicated role: %s", why)
	}
	if why := Check("DELETE FROM players", true); why == "" {
		t.Error("dedicated role still only runs SELECT")
	}
}
