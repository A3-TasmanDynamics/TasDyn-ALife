package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"website/internal/status"
)

// Server status in Discord (DISCORD_BOT.md §7):
//   - one live message in #server-status, edited every minute (edits don't
//     notify anyone, so it's never a stream of posts);
//   - a separate post when a component goes down or comes back, which does
//     notify;
//   - the bot's presence: "Watching 12/64 on Altis".

// RunStatus updates the live message, up/down posts and presence every
// minute until ctx is cancelled.
func (b *Bot) RunStatus(ctx context.Context) {
	if b.deps.Status == nil {
		return
	}
	var (
		prev     map[string]string // component key -> "up"/"down"
		presence string
	)
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		snap, err := b.deps.Status(ctx)
		if err == nil {
			if p := presenceText(snap, b, ctx); p != presence {
				if err := b.session.UpdateWatchStatus(0, p); err == nil {
					presence = p
				}
			}
			ch := b.Settings.Get(ctx, ChannelServerStatus)
			if ch != "" {
				if err := b.updateLiveStatus(ctx, ch, snap); err != nil {
					slog.Warn("discord bot: updating live status message failed", "error", err)
				}
			}
			cur := map[string]string{}
			for _, c := range snap.Components {
				if c.State == "up" || c.State == "down" {
					cur[c.Key] = c.State
				}
			}
			if prev != nil && ch != "" {
				for _, c := range snap.Components {
					was, now := prev[c.Key], cur[c.Key]
					if was == "" || now == "" || was == now {
						continue
					}
					msg := "🟢 **" + c.Name + "** is back up."
					if now == "down" {
						msg = "🔴 **" + c.Name + "** is down"
						if c.Detail != "" {
							msg += ": " + c.Detail
						}
						msg += ". We're on it."
					}
					if err := EnqueueNow(ctx, b.pool, OutboxMessage{Kind: KindChannelPost, Target: ch, Content: msg + "\n" + b.siteURL("/status")}); err != nil {
						slog.Error("discord bot: queueing status post failed", "error", err)
					}
				}
			}
			prev = cur
		} else {
			slog.Warn("discord bot: reading status failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// presenceText is the bot's "Watching …" line.
func presenceText(snap status.Snapshot, b *Bot, ctx context.Context) string {
	for _, c := range snap.Components {
		if c.Key != "game" {
			continue
		}
		switch c.State {
		case "up":
			// Detail is "12 / 64 players".
			if n, max, ok := parsePlayers(c.Detail); ok {
				return fmt.Sprintf("%d/%d on Altis", n, max)
			}
			return "Altis"
		case "down":
			return "Server offline"
		}
	}
	// Game server not monitored: fall back to open in-game sessions.
	if n, err := onlineCount(ctx, b); err == nil {
		return fmt.Sprintf("%d online on Altis", n)
	}
	return "Altis"
}

func parsePlayers(detail string) (n, max int, ok bool) {
	_, err := fmt.Sscanf(strings.TrimSpace(detail), "%d / %d players", &n, &max)
	return n, max, err == nil
}

func (b *Bot) statusEmbed(snap status.Snapshot) *discordgo.MessageEmbed {
	color := map[string]int{"up": 0x22c55e, "down": 0xef4444}[snap.State]
	if color == 0 {
		color = 0x8b98b3
	}
	return &discordgo.MessageEmbed{
		Title:       "TasDyn-ALife status",
		URL:         b.siteURL("/status"),
		Description: "**" + snap.Headline + "**\n\n" + strings.Join(statusLines(snap), "\n"),
		Color:       color,
		Footer:      &discordgo.MessageEmbedFooter{Text: "Updated every minute"},
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
}

// updateLiveStatus edits the live message in place, posting a new one if
// it doesn't exist yet (or was deleted, or the channel changed).
func (b *Bot) updateLiveStatus(ctx context.Context, ch string, snap status.Snapshot) error {
	embeds := []*discordgo.MessageEmbed{b.statusEmbed(snap)}
	if oldCh, oldMsg := b.Settings.state(ctx, stateStatusMessage); oldCh == ch && oldMsg != "" {
		empty := ""
		_, err := b.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Channel: ch, ID: oldMsg, Content: &empty, Embeds: &embeds, AllowedMentions: noMentions,
		}, discordgo.WithContext(ctx))
		if err == nil {
			return nil
		}
		if !isNotFound(err) {
			return err // transient: try the edit again next minute rather than repost
		}
		slog.Info("discord bot: live status message was deleted, posting a new one")
	}
	m, err := b.session.ChannelMessageSendComplex(ch, &discordgo.MessageSend{Embeds: embeds, AllowedMentions: noMentions}, discordgo.WithContext(ctx))
	if err != nil {
		return err
	}
	return b.Settings.setState(ctx, stateStatusMessage, ch, m.ID)
}
