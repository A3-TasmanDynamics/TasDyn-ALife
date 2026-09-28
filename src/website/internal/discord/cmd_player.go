package discord

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"

	"website/internal/audit"
	"website/internal/auth"
	"website/internal/discord/webhook"
	"website/internal/players"
	"website/internal/status"
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
			code := c.Options()["code"].StringValue()
			playerID, err := auth.ConsumeLinkCode(ctx, c.Bot.pool, code, c.User.ID, c.User.Username)
			switch {
			case err == nil:
				c.Reply("✅ Linked! Your website account is now connected to this Discord account. Your roles will update in a moment.")
				c.Bot.syncPlayerAsync(playerID)
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

// /unlink asks for confirmation with a button; the button (unlink:confirm)
// does the work, re-resolving the caller on click.
func unlinkCommand() *Command {
	return &Command{
		Def: &discordgo.ApplicationCommand{Name: "unlink", Description: "Unlink your Discord account from your website account"},
		Handler: func(ctx context.Context, c *Call) {
			if c.PlayerID == 0 {
				c.Reply("Your Discord isn't linked to a website account.")
				return
			}
			c.ReplyWith("Unlink this Discord account from your website account? Roles the website gave you (Verified, staff and faction roles) will be removed.",
				[]discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.Button{Label: "Unlink", Style: discordgo.DangerButton, CustomID: "unlink:confirm"},
					discordgo.Button{Label: "Cancel", Style: discordgo.SecondaryButton, CustomID: "unlink:cancel"},
				}}}, nil)
		},
	}
}

func unlinkComponent() *Component {
	return &Component{
		Prefix: "unlink",
		Handler: func(ctx context.Context, c *Call, args []string) {
			if len(args) == 0 || args[0] != "confirm" {
				c.Update("Cancelled. Nothing was changed.")
				return
			}
			if c.PlayerID == 0 {
				c.Update("Your Discord isn't linked to a website account.")
				return
			}
			removed := c.Bot.stripSyncedRoles(ctx, c.PlayerID, c.User.ID)
			if err := unlinkDiscord(ctx, c.Bot, c.PlayerID); err != nil {
				slog.Error("discord bot: unlink failed", "player_id", c.PlayerID, "error", err)
				c.Update("Something went wrong unlinking your account. Please try again shortly.")
				return
			}
			msg := "Unlinked. You can link again any time with a new code from your Dashboard."
			if removed > 0 {
				msg += fmt.Sprintf(" %d synced role(s) were removed.", removed)
			}
			c.Update(msg)
		},
	}
}

// unlinkDiscord clears the player's Discord link. The change trigger
// records it in rank_changes/staff_log, attributed to the player themselves.
func unlinkDiscord(ctx context.Context, b *Bot, playerID int64) error {
	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := audit.SetActor(ctx, tx, playerID, audit.SourceDiscord, "Unlinked with /unlink"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE players SET discord_id = NULL, discord_username = NULL WHERE id = $1`, playerID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func profileCommand() *Command {
	return &Command{
		Def: &discordgo.ApplicationCommand{Name: "profile", Description: "Your linked website account (only you can see the reply)"},
		Handler: func(ctx context.Context, c *Call) {
			if c.PlayerID == 0 {
				c.Reply("Your Discord isn't linked yet. Get a code from your Dashboard on the website, then use `/link`.")
				return
			}
			p, err := players.Get(ctx, c.Bot.pool, c.PlayerID)
			if err != nil {
				slog.Error("discord bot: /profile failed", "error", err)
				c.Reply("Couldn't load your profile right now.")
				return
			}
			var staffLine string
			if p.StaffRank != "" {
				staffLine = p.StaffRank
				var st string
				_ = c.Bot.pool.QueryRow(ctx, `SELECT staff_status FROM players WHERE id = $1`, p.ID).Scan(&st)
				if st != "" && st != "active" {
					staffLine += " (" + map[string]string{"loa": "on leave", "suspended": "suspended"}[st] + ")"
				}
			}
			e := &discordgo.MessageEmbed{
				Title:  webhook.Escape(p.Name),
				URL:    c.Bot.siteURL("/dashboard"),
				Color:  0xf59e0b,
				Fields: profileFields(p, staffLine, false),
			}
			if p.AvatarURL != "" {
				e.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: p.AvatarURL}
			}
			c.ReplyWith("", nil, []*discordgo.MessageEmbed{e})
		},
	}
}

// profileFields renders a profile as embed fields; staffView adds the
// identifiers and moderation details only staff may see.
func profileFields(p players.Profile, staffLine string, staffView bool) []*discordgo.MessageEmbedField {
	f := func(name, value string) *discordgo.MessageEmbedField {
		return &discordgo.MessageEmbedField{Name: name, Value: value, Inline: true}
	}
	level := func(n int, name string) string {
		if n == 0 {
			return "—"
		}
		if name != "" {
			return fmt.Sprintf("%s (level %d)", webhook.Escape(name), n)
		}
		return fmt.Sprintf("Level %d", n)
	}
	out := []*discordgo.MessageEmbedField{
		f("Police", level(p.CopLevel, p.CopName)),
		f("EMS", level(p.MedicLevel, p.MedicName)),
	}
	if p.GangName != "" {
		out = append(out, f("Gang", webhook.Escape(p.GangName)))
	}
	if staffLine != "" {
		out = append(out, f("Staff", staffLine))
	}
	if staffView {
		out = append(out,
			f("Status", strings.ToUpper(p.Status[:1])+p.Status[1:]),
			f("Steam64", "`"+p.UID+"`"),
		)
		if p.BEGUID != "" {
			out = append(out, &discordgo.MessageEmbedField{Name: "BattlEye GUID", Value: "`" + p.BEGUID + "`"})
		}
		last := "Never"
		if p.LastSeen != nil {
			last = fmt.Sprintf("<t:%d:R>", p.LastSeen.Unix())
		}
		out = append(out, f("Last seen", last), f("Joined", fmt.Sprintf("<t:%d:D>", p.Joined.Unix())))
		if p.SteamVAC != nil || p.SteamGame != nil {
			vac, game := 0, 0
			if p.SteamVAC != nil {
				vac = *p.SteamVAC
			}
			if p.SteamGame != nil {
				game = *p.SteamGame
			}
			out = append(out, f("Steam bans", fmt.Sprintf("%d VAC · %d game", vac, game)))
		}
		if p.BanActive {
			until := "permanent"
			if p.BanExpires != nil {
				until = fmt.Sprintf("until <t:%d:f>", p.BanExpires.Unix())
			}
			out = append(out, &discordgo.MessageEmbedField{Name: "⛔ Banned", Value: webhook.Escape(p.BanReason) + " (" + until + ")"})
		}
		if len(p.Aliases) > 0 {
			a := p.Aliases
			if len(a) > 5 {
				a = a[:5]
			}
			out = append(out, &discordgo.MessageEmbedField{Name: "Also known as", Value: webhook.Escape(strings.Join(a, ", "))})
		}
	}
	return out
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
			snap, err := c.Bot.deps.Status(ctx)
			if err != nil {
				slog.Error("discord bot: /status failed", "error", err)
				c.Reply("Couldn't read the server status right now.")
				return
			}
			c.Reply("**" + snap.Headline + "**\n" + strings.Join(statusLines(snap), "\n") + "\n" + c.Bot.siteURL("/status"))
		},
	}
}

var statusIcons = map[string]string{"up": "🟢", "down": "🔴", "unknown": "⚪", "unmonitored": "⚫"}

func statusLines(snap status.Snapshot) []string {
	lines := make([]string, 0, len(snap.Components))
	for _, c := range snap.Components {
		line := statusIcons[c.State] + " " + c.Name + " — " + c.StateText
		if c.Detail != "" {
			line += " (" + c.Detail + ")"
		}
		lines = append(lines, line)
	}
	return lines
}

// /players shows the online total only -- a faction breakdown would let
// rivals time crimes (DISCORD_BOT.md §12, decision 3).
func playersCommand() *Command {
	return &Command{
		Def: &discordgo.ApplicationCommand{Name: "players", Description: "How many players are online right now"},
		Handler: func(ctx context.Context, c *Call) {
			n, err := onlineCount(ctx, c.Bot)
			if err != nil {
				slog.Error("discord bot: /players failed", "error", err)
				c.Reply("Couldn't count players right now.")
				return
			}
			switch n {
			case 0:
				c.Reply("Nobody is online right now.")
			case 1:
				c.Reply("**1** player is online right now.")
			default:
				c.Reply(fmt.Sprintf("**%d** players are online right now.", n))
			}
		},
	}
}

func onlineCount(ctx context.Context, b *Bot) (int, error) {
	var n int
	err := b.pool.QueryRow(ctx, `SELECT count(DISTINCT player_id) FROM player_sessions WHERE disconnected_at IS NULL`).Scan(&n)
	return n, err
}
