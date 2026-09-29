package discord

import (
	"context"
	"errors"

	"github.com/bwmarrin/discordgo"
)

// BanMember bans a user from the guild (bans with Discord scope, issued on
// the website). The reason goes to Discord's audit log; no messages are
// deleted. The audit-log handler skips the bot's own actions, so this isn't
// logged a second time.
func (b *Bot) BanMember(ctx context.Context, userID, reason string) error {
	if b.guildID == "" {
		return errors.New("no DISCORD_GUILD_ID configured")
	}
	if len(reason) > 400 {
		reason = reason[:400]
	}
	return b.session.GuildBanCreateWithReason(b.guildID, userID, reason, 0, discordgo.WithContext(ctx))
}

// UnbanMember lifts a guild ban (when a Discord-scope ban is lifted or an
// appeal is accepted). A user who isn't banned in Discord is not an error.
func (b *Bot) UnbanMember(ctx context.Context, userID string) error {
	if b.guildID == "" {
		return errors.New("no DISCORD_GUILD_ID configured")
	}
	err := b.session.GuildBanDelete(b.guildID, userID, discordgo.WithContext(ctx))
	if isNotFound(err) {
		return nil
	}
	return err
}
