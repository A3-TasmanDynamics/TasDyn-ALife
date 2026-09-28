package discord

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runs only with TEST_DATABASE_URL. Stop the live website first: Save
// writes a staff_log row, which a running poster would send to #staff-log.
func TestSettingsSaveAndDefaults(t *testing.T) {
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

	var before map[string]string
	var actor int64
	if err := pool.QueryRow(ctx, `INSERT INTO players (uid) VALUES ('76561190000000061') RETURNING id`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	var maxLog int64
	pool.QueryRow(ctx, `SELECT COALESCE(max(id), 0) FROM staff_log`).Scan(&maxLog)
	s := &Settings{pool: pool}
	if before, err = s.All(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Restore what was there (the dev DB may have real settings).
		pool.Exec(ctx, `DELETE FROM discord_settings WHERE updated_by = $1`, actor)
		for k, v := range before {
			if d, _ := settingDef(k); v != d.Default {
				pool.Exec(ctx, `INSERT INTO discord_settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = $2`, k, v)
			}
		}
		pool.Exec(ctx, `DELETE FROM staff_log WHERE id > $1 AND staff_player_id = $2`, maxLog, actor)
		pool.Exec(ctx, `DELETE FROM players WHERE id = $1`, actor)
	})

	if !s.On(ctx, ToggleRevertDrift) && before[ToggleRevertDrift] == "" {
		t.Error("toggle default should be on")
	}
	err = s.Save(ctx, actor, map[string]string{ChannelBotAdmin: "123456789012345678", ToggleRevertDrift: "off"})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Get(ctx, ChannelBotAdmin); got != "123456789012345678" {
		t.Errorf("channel = %q", got)
	}
	if s.On(ctx, ToggleRevertDrift) {
		t.Error("toggle should be off")
	}
	var logged int
	pool.QueryRow(ctx, `SELECT count(*) FROM staff_log WHERE id > $1 AND action = 'discord.settings' AND staff_player_id = $2`, maxLog, actor).Scan(&logged)
	if logged != 1 {
		t.Errorf("expected 1 discord.settings log row, got %d", logged)
	}

	// Saving the same values again changes nothing and logs nothing.
	if err := s.Save(ctx, actor, map[string]string{ChannelBotAdmin: "123456789012345678", ToggleRevertDrift: "off"}); err != nil {
		t.Fatal(err)
	}
	pool.QueryRow(ctx, `SELECT count(*) FROM staff_log WHERE id > $1 AND staff_player_id = $2`, maxLog, actor).Scan(&logged)
	if logged != 1 {
		t.Errorf("no-op save logged a row (%d total)", logged)
	}

	if err := s.Save(ctx, actor, map[string]string{"channel.nope": "1"}); !errors.Is(err, ErrUnknownSetting) {
		t.Errorf("unknown key: err = %v", err)
	}
	if err := s.Save(ctx, actor, map[string]string{ToggleLogModeration: "maybe"}); !errors.Is(err, ErrUnknownSetting) {
		t.Errorf("bad toggle: err = %v", err)
	}

	// Internal state pointers round-trip and are hidden from All.
	if err := s.setState(ctx, stateStatusMessage, "111", "222"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM discord_settings WHERE key = $1`, stateStatusMessage) })
	if c, m := s.state(ctx, stateStatusMessage); c != "111" || m != "222" {
		t.Errorf("state = %q/%q", c, m)
	}
	all, _ := s.All(ctx)
	if _, ok := all[stateStatusMessage]; ok {
		t.Error("state.* leaked into All")
	}
}
