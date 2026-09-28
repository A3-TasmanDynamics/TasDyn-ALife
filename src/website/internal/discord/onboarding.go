package discord

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Onboarding (DISCORD_BOT.md §5.1): one message in #welcome, posted from
// /admin/discord, with a Link account button (to the website) and an
// "Update my roles" button for after linking. It's a channel message rather
// than a DM per new member (§12, decision 1): bot DMs to brand-new members
// often go unseen or get flagged.

// ErrNoChannel is returned when the feature's channel isn't set.
var ErrNoChannel = errors.New("channel not set")

func (b *Bot) welcomeMessage() (string, []discordgo.MessageComponent) {
	content := "## Welcome to TasDyn-ALife\n" +
		"Link your Steam account to get access to the rest of the server:\n" +
		"1. Press **Link account** and sign in to the website with Steam.\n" +
		"2. On your Dashboard, press **Connect Discord** (or get a code and use `/link <code>` here).\n" +
		"3. Press **Update my roles** below, or wait a moment: your roles are given automatically.\n\n" +
		"If you leave and come back, your roles come back with you."
	return content, []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{Label: "Link account", Style: discordgo.LinkButton, URL: b.siteURL("/dashboard")},
		discordgo.Button{Label: "Update my roles", Style: discordgo.SecondaryButton, CustomID: "onboard:sync"},
	}}}
}

// PostWelcome posts the welcome message to #welcome, or edits it in place
// if it was already posted there. Returns a link to the message.
func (b *Bot) PostWelcome(ctx context.Context) (string, error) {
	ch := b.Settings.Get(ctx, ChannelWelcome)
	if ch == "" {
		return "", ErrNoChannel
	}
	content, comps := b.welcomeMessage()
	if oldCh, oldMsg := b.Settings.state(ctx, stateWelcomeMessage); oldCh == ch && oldMsg != "" {
		_, err := b.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Channel: ch, ID: oldMsg, Content: &content, Components: &comps, AllowedMentions: noMentions,
		}, discordgo.WithContext(ctx))
		if err == nil {
			return b.messageLink(ch, oldMsg), nil
		}
		if !isNotFound(err) {
			return "", err
		}
		slog.Info("discord bot: welcome message was deleted, posting a new one")
	}
	m, err := b.session.ChannelMessageSendComplex(ch, &discordgo.MessageSend{
		Content: content, Components: comps, AllowedMentions: noMentions,
	}, discordgo.WithContext(ctx))
	if err != nil {
		return "", err
	}
	if err := b.Settings.setState(ctx, stateWelcomeMessage, ch, m.ID); err != nil {
		return "", err
	}
	return b.messageLink(ch, m.ID), nil
}

func (b *Bot) messageLink(channelID, messageID string) string {
	return fmt.Sprintf("https://discord.com/channels/%s/%s/%s", b.guildID, channelID, messageID)
}

func onboardingComponent() *Component {
	return &Component{
		Prefix: "onboard",
		Handler: func(ctx context.Context, c *Call, args []string) {
			if c.PlayerID == 0 {
				c.Reply("You haven't linked yet. Press **Link account**, sign in with Steam, then press **Connect Discord** on your Dashboard. Come back and press this button when you're done.")
				return
			}
			e := c.Bot.roleSyncEngine()
			if e == nil {
				c.Reply("You're linked. Role sync isn't running right now, so your roles will arrive a little later.")
				return
			}
			c.Defer()
			syncCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if _, err := e.SyncPlayer(syncCtx, c.PlayerID, 0); err != nil {
				slog.Warn("discord bot: onboarding sync failed", "player_id", c.PlayerID, "error", err)
				c.Reply("You're linked, but updating your roles failed. It'll be retried automatically within 15 minutes.")
				return
			}
			c.Reply("✅ You're linked and your roles are up to date.")
		},
	}
}
