package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"website/internal/auth"
)

// Gateway events (DISCORD_BOT.md §5).
func (b *Bot) addEventHandlers() {
	b.session.AddHandler(b.onMemberAdd)
	b.session.AddHandler(b.onMemberUpdate)
	b.session.AddHandler(b.onAuditLogEntry)
}

// onMemberAdd: a linked player who (re)joins gets their roles back straight
// away -- the website is the source of truth (§5.1). Nobody is removed or
// deleted when they leave (§5.4), so there's no leave handler.
func (b *Bot) onMemberAdd(_ *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m.GuildID != b.guildID || m.User == nil || m.User.Bot {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if id, found, err := auth.FindPlayerByDiscordID(ctx, b.pool, m.User.ID); err == nil && found {
		b.syncPlayerAsync(id)
	}
}

// Drift detection (§5.2): when a linked member's roles change, re-run their
// sync shortly after. The engine only touches mapped roles, so this puts
// back a synced role someone removed by hand (or takes away one they added)
// without waiting for the 15-minute pass. The short delay coalesces bursts
// -- including the bot's own role changes, which also fire this event and
// then find nothing to do.
var driftTimers = struct {
	sync.Mutex
	m map[string]*time.Timer
}{m: map[string]*time.Timer{}}

const driftDelay = 5 * time.Second

func (b *Bot) onMemberUpdate(_ *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m.GuildID != b.guildID || m.User == nil || m.User.Bot {
		return
	}
	userID := m.User.ID
	driftTimers.Lock()
	defer driftTimers.Unlock()
	if t, ok := driftTimers.m[userID]; ok {
		t.Reset(driftDelay)
		return
	}
	driftTimers.m[userID] = time.AfterFunc(driftDelay, func() {
		driftTimers.Lock()
		delete(driftTimers.m, userID)
		driftTimers.Unlock()
		b.checkDrift(userID)
	})
}

func (b *Bot) checkDrift(userID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	e := b.roleSyncEngine()
	if e == nil || !b.Settings.On(ctx, ToggleRevertDrift) {
		return
	}
	playerID, found, err := auth.FindPlayerByDiscordID(ctx, b.pool, userID)
	if err != nil || !found {
		return
	}
	res, err := e.SyncPlayer(ctx, playerID, 0)
	if err != nil {
		slog.Warn("discord bot: drift check failed", "player_id", playerID, "error", err)
		return
	}
	var changes []string
	for _, r := range res {
		if r.Platform != "discord" {
			continue
		}
		for _, g := range r.Added {
			changes = append(changes, "put back <@&"+g+">")
		}
		for _, g := range r.Removed {
			changes = append(changes, "took away <@&"+g+">")
		}
		for _, er := range r.Errors {
			changes = append(changes, "⚠️ "+er)
		}
	}
	if len(changes) > 0 {
		b.postBotAdmin(ctx, fmt.Sprintf("🔁 Synced roles were changed by hand for <@%s>, so I %s. Change ranks on the website (or with `/promote`) instead.",
			userID, strings.Join(changes, ", ")))
	}
}

// Discord-side moderation gets logged (§5.3): kicks, bans, unbans and
// timeouts done directly in Discord become staff_log rows (source
// 'discord'), which the poster then sends to #staff-log like any other.
func (b *Bot) onAuditLogEntry(_ *discordgo.Session, ev *discordgo.GuildAuditLogEntryCreate) {
	if ev.GuildID != b.guildID || ev.AuditLogEntry == nil || ev.ActionType == nil {
		return
	}
	e := ev.AuditLogEntry
	if e.UserID == b.session.State.User.ID {
		return // the bot's own actions are logged where they're made
	}
	var action string
	switch *e.ActionType {
	case discordgo.AuditLogActionMemberKick:
		action = "discord.kick"
	case discordgo.AuditLogActionMemberBanAdd:
		action = "discord.ban"
	case discordgo.AuditLogActionMemberBanRemove:
		action = "discord.unban"
	case discordgo.AuditLogActionMemberUpdate:
		for _, ch := range e.Changes {
			if ch.Key != nil && *ch.Key == discordgo.AuditLogChangeKeyCommunicationDisabledUntil {
				action = "discord.timeout"
				if ch.NewValue == nil {
					action = "discord.timeout_end"
				}
			}
		}
	}
	if action == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if !b.Settings.On(ctx, ToggleLogModeration) {
		return
	}
	if err := b.logDiscordModeration(ctx, action, e.UserID, e.TargetID, e.Reason); err != nil {
		slog.Error("discord bot: logging Discord moderation failed", "action", action, "error", err)
	}
}

func (b *Bot) logDiscordModeration(ctx context.Context, action, executorID, targetID, reason string) error {
	name := func(id string) string {
		if id == "" {
			return ""
		}
		if m, err := b.session.State.Member(b.guildID, id); err == nil && m.User != nil {
			return m.User.Username
		}
		if u, err := b.session.User(id); err == nil {
			return u.Username
		}
		return id
	}
	playerOf := func(id string) *int64 {
		if id == "" {
			return nil
		}
		if pid, found, err := auth.FindPlayerByDiscordID(ctx, b.pool, id); err == nil && found {
			return &pid
		}
		return nil
	}
	after, err := json.Marshal(map[string]string{
		"user": name(targetID), "user_id": targetID, "by": name(executorID), "by_id": executorID,
	})
	if err != nil {
		return err
	}
	var reasonPtr *string
	if reason = strings.TrimSpace(reason); reason != "" {
		reasonPtr = &reason
	}
	_, err = b.pool.Exec(ctx, `
		INSERT INTO staff_log (staff_player_id, target_player_id, action, reason, after_value, source)
		VALUES ($1, $2, $3, $4, $5, 'discord')`,
		playerOf(executorID), playerOf(targetID), action, reasonPtr, after)
	return err
}

// syncPlayerAsync runs one player's role sync in the background (after
// /link, or when they rejoin).
func (b *Bot) syncPlayerAsync(playerID int64) {
	e := b.roleSyncEngine()
	if e == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := e.SyncPlayer(ctx, playerID, 0); err != nil {
			slog.Warn("discord bot: role sync failed", "player_id", playerID, "error", err)
		}
	}()
}

// stripSyncedRoles removes every mapped role from a member before their
// account is unlinked. Returns how many were removed.
func (b *Bot) stripSyncedRoles(ctx context.Context, playerID int64, userID string) int {
	e := b.roleSyncEngine()
	if e == nil {
		return 0
	}
	removed, err := e.Strip(ctx, playerID, "discord", userID)
	if err != nil {
		slog.Warn("discord bot: removing synced roles on unlink failed", "player_id", playerID, "error", err)
	}
	return len(removed)
}

// postBotAdmin queues a message for #bot-admin, if that channel is set.
// It goes through the outbox, so it's delivered even across a restart.
func (b *Bot) postBotAdmin(ctx context.Context, content string) {
	ch := b.Settings.Get(ctx, ChannelBotAdmin)
	if ch == "" {
		return
	}
	if err := EnqueueNow(ctx, b.pool, OutboxMessage{Kind: KindChannelPost, Target: ch, Content: content}); err != nil {
		slog.Error("discord bot: queueing #bot-admin message failed", "error", err)
	}
}
