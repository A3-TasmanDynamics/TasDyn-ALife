package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// requiredPermissions are the guild permissions the bot needs for the
// features built so far (docs/DISCORD_BOT.md §9). Grows as features land.
var requiredPermissions = []struct {
	bit  int64
	name string
}{
	{discordgo.PermissionViewChannel, "View Channels"},
	{discordgo.PermissionSendMessages, "Send Messages"},
	{discordgo.PermissionEmbedLinks, "Embed Links"},
}

func botCommand() *Command {
	return &Command{
		Def: &discordgo.ApplicationCommand{
			Name:        "bot",
			Description: "Bot administration",
			Options: []*discordgo.ApplicationCommandOption{{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "health",
				Description: "Gateway latency, outbox backlog and missing permissions",
			}},
		},
		Permission: "bot.admin",
		Handler: func(ctx context.Context, c *Call) {
			b := c.Bot
			var out []string

			ready, latency := b.Health()
			out = append(out, fmt.Sprintf("Gateway: %s · heartbeat %dms", map[bool]string{true: "ready", false: "not ready"}[ready], latency.Milliseconds()))

			if b.deps.OutboxBacklog != nil {
				if n, err := b.deps.OutboxBacklog(ctx); err == nil {
					out = append(out, fmt.Sprintf("Outbox: %d undelivered", n))
				} else {
					out = append(out, "Outbox: couldn't read backlog")
				}
			}

			if missing, err := b.missingPermissions(); err != nil {
				out = append(out, "Permissions: couldn't check ("+err.Error()+")")
			} else if len(missing) == 0 {
				out = append(out, "Permissions: all required permissions present")
			} else {
				out = append(out, "⚠️ Missing permissions: "+strings.Join(missing, ", "))
			}

			c.Reply(strings.Join(out, "\n"))
		},
	}
}

// missingPermissions compares the bot's effective guild permissions with
// requiredPermissions.
func (b *Bot) missingPermissions() ([]string, error) {
	if b.guildID == "" {
		return nil, fmt.Errorf("no DISCORD_GUILD_ID configured")
	}
	member, err := b.session.GuildMember(b.guildID, b.session.State.User.ID)
	if err != nil {
		return nil, fmt.Errorf("reading bot member")
	}
	roles, err := b.session.GuildRoles(b.guildID)
	if err != nil {
		return nil, fmt.Errorf("reading guild roles")
	}
	var perms int64
	for _, r := range roles {
		if r.ID == b.guildID { // @everyone
			perms |= r.Permissions
			continue
		}
		for _, id := range member.Roles {
			if r.ID == id {
				perms |= r.Permissions
			}
		}
	}
	if perms&discordgo.PermissionAdministrator != 0 {
		return []string{"(bot has Administrator — works, but should be removed: DISCORD_BOT.md §9)"}, nil
	}
	var missing []string
	for _, p := range requiredPermissions {
		if perms&p.bit == 0 {
			missing = append(missing, p.name)
		}
	}
	return missing, nil
}
