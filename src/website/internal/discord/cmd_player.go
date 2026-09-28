package discord

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"

	"website/internal/auth"
)

func linkCommand() *Command {
	return &Command{
		Def: &discordgo.ApplicationCommand{
			Name:        "link",
			Description: "Link your Discord account to your TasDyn-ALife player account",
			Options: []*discordgo.ApplicationCommandOption{{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "code",
				Description: "The code shown on the website's Dashboard",
				Required:    true,
			}},
		},
		Handler: func(ctx context.Context, c *Call) {
			code := c.Interaction.ApplicationCommandData().Options[0].StringValue()
			_, err := auth.ConsumeLinkCode(ctx, c.Bot.pool, code, c.User.ID, c.User.Username)
			switch {
			case err == nil:
				c.Reply("✅ Linked! Your website account is now connected to this Discord account.")
			case errors.Is(err, auth.ErrLinkCodeInvalid):
				c.Reply("That code is invalid or has expired. Generate a new one from your Dashboard on the website.")
			case errors.Is(err, auth.ErrDiscordAlreadyLinked):
				c.Reply("This Discord account is already linked to a different player account.")
			default:
				slog.Error("discord bot: /link failed", "error", err)
				c.Reply("Something went wrong linking your account. Please try again shortly.")
			}
		},
	}
}

func statusCommand() *Command {
	return &Command{
		Def: &discordgo.ApplicationCommand{
			Name:        "status",
			Description: "Show the TasDyn-ALife server and service status",
		},
		Handler: func(ctx context.Context, c *Call) {
			if c.Bot.deps.Status == nil {
				c.Reply("Status monitoring isn't running right now.")
				return
			}
			headline, lines, err := c.Bot.deps.Status(ctx)
			if err != nil {
				slog.Error("discord bot: /status failed", "error", err)
				c.Reply("Couldn't read the server status right now.")
				return
			}
			c.Reply("**" + headline + "**\n" + strings.Join(lines, "\n"))
		},
	}
}
