package discord

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/bwmarrin/discordgo"

	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
)

// Settings is the bot's admin-configured layout (DISCORD_BOT.md §3),
// stored in discord_settings and edited on /admin/discord. Nobody pastes
// channel IDs into .env; a channel left unset means that feature simply
// doesn't post.
type Settings struct {
	pool *pgxpool.Pool
}

// SettingDef describes one admin-editable setting.
type SettingDef struct {
	Key     string
	Label   string
	Help    string
	Kind    string // "channel" or "toggle"
	Default string // for toggles: "on" or "off"
}

// Setting keys.
const (
	ChannelWelcome      = "channel.welcome"
	ChannelServerStatus = "channel.server_status"
	ChannelStaffLog     = "channel.staff_log"
	ChannelBotAdmin     = "channel.bot_admin"
	ToggleLogModeration = "toggle.log_discord_moderation"
	ToggleRevertDrift   = "toggle.revert_role_drift"

	// Internal state, not shown on the settings page: where the bot's own
	// long-lived messages are, as "channelID/messageID", so they're edited
	// in place rather than reposted.
	stateWelcomeMessage = "state.welcome_message"
	stateStatusMessage  = "state.status_message"
)

// SettingDefs lists the settings in page order.
var SettingDefs = []SettingDef{
	{ChannelWelcome, "#welcome", "Onboarding message with the Link account button. Post it from this page once the channel is set.", "channel", ""},
	{ChannelServerStatus, "#server-status", "One live status message, edited every minute, plus a post when something goes down or comes back.", "channel", ""},
	{ChannelStaffLog, "#staff-log", "Every staff action, posted by the bot. If unset, DISCORD_STAFF_LOG_WEBHOOK is used instead.", "channel", ""},
	{ChannelBotAdmin, "#bot-admin", "Role drift that was reverted, role sync failures and messages that couldn't be delivered.", "channel", ""},
	{ToggleLogModeration, "Log moderation done in Discord", "Kicks, bans and timeouts done directly in Discord are written to the Staff Log (source: discord).", "toggle", "on"},
	{ToggleRevertDrift, "Revert hand-edited synced roles", "If someone adds or removes a synced role by hand, put it back straight away and report it in #bot-admin.", "toggle", "on"},
}

func settingDef(key string) (SettingDef, bool) {
	for _, d := range SettingDefs {
		if d.Key == key {
			return d, true
		}
	}
	return SettingDef{}, false
}

// Get returns a setting's value, or its default ("" for unset channels).
func (s *Settings) Get(ctx context.Context, key string) string {
	var v string
	err := s.pool.QueryRow(ctx, `SELECT value FROM discord_settings WHERE key = $1`, key).Scan(&v)
	if err != nil {
		if d, ok := settingDef(key); ok {
			return d.Default
		}
		return ""
	}
	return v
}

// On reports whether a toggle is on.
func (s *Settings) On(ctx context.Context, key string) bool { return s.Get(ctx, key) == "on" }

// All returns every admin-editable setting's current value.
func (s *Settings) All(ctx context.Context) (map[string]string, error) {
	out := map[string]string{}
	for _, d := range SettingDefs {
		out[d.Key] = d.Default
	}
	rows, err := s.pool.Query(ctx, `SELECT key, value FROM discord_settings WHERE key NOT LIKE 'state.%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// ErrUnknownSetting is returned by Save for a key not in SettingDefs.
var ErrUnknownSetting = errors.New("unknown setting")

// Save writes the given settings and logs the change (discord.settings)
// with before/after values. Channel values must already be validated by
// the caller against the guild's channels.
func (s *Settings) Save(ctx context.Context, actorID int64, values map[string]string) error {
	before, err := s.All(ctx)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	changedBefore, changedAfter := map[string]any{}, map[string]any{}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		d, ok := settingDef(k)
		if !ok {
			return fmt.Errorf("%w: %s", ErrUnknownSetting, k)
		}
		v := values[k]
		if d.Kind == "toggle" && v != "on" && v != "off" {
			return fmt.Errorf("%w: %s must be on or off", ErrUnknownSetting, k)
		}
		if before[k] == v {
			continue
		}
		changedBefore[k], changedAfter[k] = before[k], v
		if _, err := tx.Exec(ctx, `
			INSERT INTO discord_settings (key, value, updated_by, updated_at) VALUES ($1, $2, $3, now())
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now()`,
			k, v, actorID); err != nil {
			return err
		}
	}
	if len(changedAfter) == 0 {
		return nil
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actorID, Action: "discord.settings", Reason: "Discord settings updated",
		Before: changedBefore, After: changedAfter,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// state reads an internal "channelID/messageID" pointer.
func (s *Settings) state(ctx context.Context, key string) (channelID, messageID string) {
	var v string
	if err := s.pool.QueryRow(ctx, `SELECT value FROM discord_settings WHERE key = $1`, key).Scan(&v); err != nil {
		return "", ""
	}
	for i := 0; i < len(v); i++ {
		if v[i] == '/' {
			return v[:i], v[i+1:]
		}
	}
	return "", ""
}

func (s *Settings) setState(ctx context.Context, key, channelID, messageID string) error {
	var err error
	if channelID == "" {
		_, err = s.pool.Exec(ctx, `DELETE FROM discord_settings WHERE key = $1`, key)
	} else {
		_, err = s.pool.Exec(ctx, `
			INSERT INTO discord_settings (key, value, updated_at) VALUES ($1, $2, now())
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, key, channelID+"/"+messageID)
	}
	return err
}

// StaffLogPoster returns a function posting to the configured #staff-log
// channel, or nil when none is set (audit.Poster then uses the webhook).
func (b *Bot) StaffLogPoster(ctx context.Context) func(ctx context.Context, content string) error {
	ch := b.Settings.Get(ctx, ChannelStaffLog)
	if ch == "" {
		return nil
	}
	return func(ctx context.Context, content string) error {
		_, err := b.session.ChannelMessageSendComplex(ch, &discordgo.MessageSend{Content: content, AllowedMentions: noMentions}, discordgo.WithContext(ctx))
		return err
	}
}
