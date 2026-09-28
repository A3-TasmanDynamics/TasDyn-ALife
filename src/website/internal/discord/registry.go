package discord

import (
	"context"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"

	"website/internal/auth"
)

// Command is one slash command. Permission, if set, is checked against the
// caller's LINKED website account before Handler runs -- never against
// Discord roles (docs/DISCORD_BOT.md §1 principle 3).
type Command struct {
	Def        *discordgo.ApplicationCommand
	Permission string // auth.Catalogue key; "" = anyone
	Handler    func(ctx context.Context, c *Call)
}

// Call is one invocation, with the resolved caller.
type Call struct {
	Bot         *Bot
	Interaction *discordgo.InteractionCreate
	User        *discordgo.User
	PlayerID    int64 // 0 = caller hasn't linked a website account
}

// registry is the one place commands are wired in. Staff commands also get
// Discord-side default permissions so they're hidden from regular members
// in the command picker -- a convenience only; the real check is the
// linked-account permission above.
func (b *Bot) registry() map[string]*Command {
	cmds := []*Command{
		linkCommand(),
		statusCommand(),
		botCommand(),
	}
	m := make(map[string]*Command, len(cmds))
	for _, c := range cmds {
		if c.Permission != "" && c.Def.DefaultMemberPermissions == nil {
			perm := int64(discordgo.PermissionManageMessages)
			c.Def.DefaultMemberPermissions = &perm
		}
		m[c.Def.Name] = c
	}
	return m
}

func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	cmd, ok := b.commands[i.ApplicationCommandData().Name]
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// The caller's identity comes from Discord's interaction payload, never
	// from anything a command argument could claim.
	user := i.User
	if user == nil && i.Member != nil {
		user = i.Member.User
	}
	if user == nil {
		b.replyEphemeral(i, "Couldn't determine your Discord identity. Try again from a server channel.")
		return
	}

	call := &Call{Bot: b, Interaction: i, User: user}
	playerID, found, err := auth.FindPlayerByDiscordID(ctx, b.pool, user.ID)
	if err != nil {
		slog.Error("discord bot: resolving caller failed", "command", cmd.Def.Name, "error", err)
		b.replyEphemeral(i, "Something went wrong. Please try again shortly.")
		return
	}
	if found {
		call.PlayerID = playerID
	}

	if cmd.Permission != "" {
		if !found {
			b.replyEphemeral(i, "Link your Discord to your website account first (`/link`), then try again.")
			return
		}
		denial, err := auth.Can(ctx, b.pool, playerID, cmd.Permission)
		if err != nil {
			slog.Error("discord bot: permission check failed", "command", cmd.Def.Name, "error", err)
			b.replyEphemeral(i, "Something went wrong checking your permissions.")
			return
		}
		switch denial {
		case auth.Allowed:
		case auth.DenyInactive:
			b.replyEphemeral(i, "You're currently suspended or on leave, so staff commands are unavailable.")
			return
		default:
			b.replyEphemeral(i, "You don't have permission to use this command.")
			return
		}
	}

	cmd.Handler(ctx, call)
}

// replyEphemeral answers so only the caller sees it. Mentions are always
// disabled: replies can include player-chosen text.
func (b *Bot) replyEphemeral(i *discordgo.InteractionCreate, content string) {
	err := b.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:         content,
			Flags:           discordgo.MessageFlagsEphemeral,
			AllowedMentions: &discordgo.MessageAllowedMentions{},
		},
	})
	if err != nil {
		slog.Error("discord bot: replying to interaction failed", "error", err)
	}
}

// Reply answers the call ephemerally.
func (c *Call) Reply(content string) { c.Bot.replyEphemeral(c.Interaction, content) }
