package discord

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"website/internal/audit"
	"website/internal/auth"
	"website/internal/discord/webhook"
	"website/internal/players"
	"website/internal/staff"
)

// Staff commands (DISCORD_BOT.md §4.2). Each one is a thin wrapper over the
// same service function the website's Admin Panel calls, so the seniority
// rules, the staff_log row and the role sync are identical.

func userOption(desc string) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: desc, Required: true}
}

func reasonOption() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Why (logged to the Staff Log)", Required: true, MaxLength: 500}
}

// targetPlayer resolves the command's Discord user to their linked player,
// replying and returning 0 if they aren't linked.
func targetPlayer(ctx context.Context, c *Call) (int64, *discordgo.User) {
	u := c.TargetUser()
	if u == nil {
		c.Reply("Pick a user.")
		return 0, nil
	}
	id, found, err := auth.FindPlayerByDiscordID(ctx, c.Bot.pool, u.ID)
	if err != nil {
		slog.Error("discord bot: resolving target failed", "error", err)
		c.Reply("Something went wrong. Please try again shortly.")
		return 0, nil
	}
	if !found {
		c.Reply(fmt.Sprintf("%s hasn't linked a website account, so there's nothing to show or change. Use `/lookup` with their Steam64 or name instead.", mention(u)))
		return 0, nil
	}
	return id, u
}

func mention(u *discordgo.User) string { return "<@" + u.ID + ">" }

func whoisCommand() *Command {
	return &Command{
		Def: &discordgo.ApplicationCommand{
			Name: "whois", Description: "A linked member's player profile (only you see the reply)",
			Options: []*discordgo.ApplicationCommandOption{userOption("Who to look up")},
		},
		Permission: "players.view",
		Handler:    whoisHandler,
	}
}

// whoisUserMenu is the right-click → Apps → Whois version.
func whoisUserMenu() *Command {
	perm := int64(discordgo.PermissionManageMessages)
	return &Command{
		Def:        &discordgo.ApplicationCommand{Name: "Whois", Type: discordgo.UserApplicationCommand, DefaultMemberPermissions: &perm},
		Permission: "players.view",
		Handler:    whoisHandler,
	}
}

func whoisHandler(ctx context.Context, c *Call) {
	id, _ := targetPlayer(ctx, c)
	if id == 0 {
		return
	}
	replyPlayerCard(ctx, c, id)
}

func replyPlayerCard(ctx context.Context, c *Call, id int64) {
	p, err := players.Get(ctx, c.Bot.pool, id)
	if err != nil {
		slog.Error("discord bot: loading player failed", "player_id", id, "error", err)
		c.Reply("Couldn't load that player right now.")
		return
	}
	var staffLine string
	if p.StaffRank != "" {
		staffLine = p.StaffRank
		if m, err := staff.Get(ctx, c.Bot.pool, id); err == nil && m.Status != "active" {
			staffLine += " · " + strings.ToUpper(m.Status)
		}
	}
	e := &discordgo.MessageEmbed{
		Title:  webhook.Escape(p.Name),
		URL:    c.Bot.siteURL(fmt.Sprintf("/admin/players?id=%d", p.ID)),
		Color:  map[string]int{"online": 0x22c55e, "banned": 0xef4444}[p.Status],
		Fields: profileFields(p, staffLine, true),
		Footer: &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Player #%d · open in Player Lookup for full details", p.ID)},
	}
	if p.Discord != "" {
		e.Description = "Discord: " + webhook.Escape(p.Discord)
	}
	if p.AvatarURL != "" {
		e.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: p.AvatarURL}
	}
	c.ReplyWith("", nil, []*discordgo.MessageEmbed{e})
}

func lookupCommand() *Command {
	return &Command{
		Def: &discordgo.ApplicationCommand{
			Name: "lookup", Description: "Find a player by Steam64, BattlEye GUID, name or player id",
			Options: []*discordgo.ApplicationCommandOption{{
				Type: discordgo.ApplicationCommandOptionString, Name: "query", Description: "Steam64, GUID, current or past name, or #id", Required: true, MaxLength: 100,
			}},
		},
		Permission: "players.view",
		Handler: func(ctx context.Context, c *Call) {
			q := strings.TrimPrefix(strings.TrimSpace(c.Options()["query"].StringValue()), "#")
			results, err := players.Search(ctx, c.Bot.pool, q)
			if err != nil {
				slog.Error("discord bot: /lookup failed", "error", err)
				c.Reply("Search failed. Please try again shortly.")
				return
			}
			switch len(results) {
			case 0:
				c.Reply("No players match that.")
			case 1:
				replyPlayerCard(ctx, c, results[0].ID)
			default:
				var b strings.Builder
				fmt.Fprintf(&b, "%d matches — run `/lookup #<id>` for one:\n", len(results))
				for i, r := range results {
					if i == 10 {
						fmt.Fprintf(&b, "…and %d more. Narrow the search.", len(results)-10)
						break
					}
					fmt.Fprintf(&b, "`#%d` **%s** · %s · %s\n", r.ID, webhook.Escape(r.Name), webhook.Escape(r.Hint), r.Status)
				}
				c.Reply(b.String())
			}
		},
	}
}

// rankChoices autocompletes staff ranks the caller may assign (below their
// own level, unless Head Admin). withRemove adds "Remove from staff".
func rankChoices(ctx context.Context, c *Call, withRemove bool) []*discordgo.ApplicationCommandOptionChoice {
	_, typed := c.Focused()
	typed = strings.ToLower(typed)
	var myLevel int
	_ = c.Bot.pool.QueryRow(ctx, `SELECT COALESCE(sr.level, 0) FROM players p LEFT JOIN staff_ranks sr ON sr.id = p.staff_rank_id WHERE p.id = $1`, c.PlayerID).Scan(&myLevel)
	ranks, err := staff.Ranks(ctx, c.Bot.pool)
	if err != nil {
		return nil
	}
	var out []*discordgo.ApplicationCommandOptionChoice
	if withRemove && strings.Contains("remove from staff", typed) {
		out = append(out, &discordgo.ApplicationCommandOptionChoice{Name: "Remove from staff", Value: "0"})
	}
	for i := len(ranks) - 1; i >= 0; i-- {
		r := ranks[i]
		if r.Level >= myLevel && myLevel < staff.HeadAdminLevel {
			continue
		}
		if typed != "" && !strings.Contains(strings.ToLower(r.Name), typed) {
			continue
		}
		out = append(out, &discordgo.ApplicationCommandOptionChoice{Name: fmt.Sprintf("%s (level %d)", r.Name, r.Level), Value: strconv.Itoa(r.ID)})
	}
	return out
}

func rankCommand(name, desc string, demote bool) *Command {
	return &Command{
		Def: &discordgo.ApplicationCommand{
			Name: name, Description: desc,
			Options: []*discordgo.ApplicationCommandOption{
				userOption("The staff member"),
				{Type: discordgo.ApplicationCommandOptionString, Name: "rank", Description: "New rank", Required: true, Autocomplete: true},
				reasonOption(),
			},
		},
		Permission: "staff.edit",
		Autocomplete: func(ctx context.Context, c *Call) []*discordgo.ApplicationCommandOptionChoice {
			return rankChoices(ctx, c, demote)
		},
		Handler: func(ctx context.Context, c *Call) {
			target, u := targetPlayer(ctx, c)
			if target == 0 {
				return
			}
			opts := c.Options()
			rankID, err := strconv.Atoi(opts["rank"].StringValue())
			if err != nil {
				c.Reply("Pick a rank from the list.")
				return
			}
			if rankID == 0 && !c.can(ctx, "staff.remove") {
				c.Reply("Removing someone from staff needs the `staff.remove` permission.")
				return
			}
			cur, err := staff.Get(ctx, c.Bot.pool, target)
			if err != nil {
				c.Reply("Couldn't load that member.")
				return
			}
			var newLevel int
			if rankID != 0 {
				if err := c.Bot.pool.QueryRow(ctx, `SELECT level FROM staff_ranks WHERE id = $1`, rankID).Scan(&newLevel); err != nil {
					c.Reply("That rank doesn't exist.")
					return
				}
			}
			switch {
			case !demote && newLevel <= cur.RankLevel:
				c.Reply("That isn't a promotion. Use `/demote` to lower someone's rank.")
				return
			case demote && (cur.RankLevel == 0 || newLevel >= cur.RankLevel):
				c.Reply("That isn't a demotion. Use `/promote` to raise someone's rank.")
				return
			}
			err = staff.SetRank(ctx, c.Bot.pool, staff.Actor{PlayerID: c.PlayerID, Source: audit.SourceDiscord}, target, rankID, opts["reason"].StringValue())
			if replyServiceError(c, err) {
				return
			}
			what := "removed from staff"
			if rankID != 0 {
				var rn string
				_ = c.Bot.pool.QueryRow(ctx, `SELECT display_name FROM staff_ranks WHERE id = $1`, rankID).Scan(&rn)
				what = "is now **" + webhook.Escape(rn) + "**"
			}
			c.Reply(fmt.Sprintf("✅ %s %s. Logged to the Staff Log; their Discord roles update automatically.", mention(u), what))
		},
	}
}

func promoteCommand() *Command {
	return rankCommand("promote", "Promote a staff member (or add someone to staff)", false)
}
func demoteCommand() *Command {
	return rankCommand("demote", "Demote a staff member, or remove them from staff", true)
}

func loaCommand() *Command {
	return &Command{
		Def: &discordgo.ApplicationCommand{
			Name: "loa", Description: "Put a staff member on leave of absence",
			Options: []*discordgo.ApplicationCommandOption{
				userOption("The staff member"),
				reasonOption(),
				{Type: discordgo.ApplicationCommandOptionInteger, Name: "days", Description: "Ends automatically after this many days (leave out for open-ended)", MinValue: ptrFloat(1), MaxValue: 365},
			},
		},
		Permission: "staff.loa",
		Handler: func(ctx context.Context, c *Call) {
			setStatusHandler(ctx, c, "loa")
		},
	}
}

func reinstateCommand() *Command {
	return &Command{
		Def: &discordgo.ApplicationCommand{
			Name: "reinstate", Description: "End a staff member's LOA or suspension",
			Options: []*discordgo.ApplicationCommandOption{userOption("The staff member"), reasonOption()},
		},
		Permission: "staff.loa",
		Handler: func(ctx context.Context, c *Call) {
			setStatusHandler(ctx, c, "active")
		},
	}
}

func setStatusHandler(ctx context.Context, c *Call, next string) {
	target, u := targetPlayer(ctx, c)
	if target == 0 {
		return
	}
	cur, err := staff.Get(ctx, c.Bot.pool, target)
	if err != nil {
		c.Reply("Couldn't load that member.")
		return
	}
	if cur.RankLevel == 0 {
		c.Reply(mention(u) + " isn't staff.")
		return
	}
	if cur.Status == next {
		c.Reply(mention(u) + " is already " + map[string]string{"active": "active", "loa": "on LOA"}[next] + ".")
		return
	}
	if perm := staff.RequiredStatusPermission(cur.Status, next); !c.can(ctx, perm) {
		c.Reply("That needs the `" + perm + "` permission.")
		return
	}
	opts := c.Options()
	var until *time.Time
	if o := opts["days"]; o != nil {
		t := time.Now().Add(time.Duration(o.IntValue()) * 24 * time.Hour)
		until = &t
	}
	err = staff.SetStatus(ctx, c.Bot.pool, staff.Actor{PlayerID: c.PlayerID, Source: audit.SourceDiscord}, target, next, opts["reason"].StringValue(), until)
	if replyServiceError(c, err) {
		return
	}
	msg := "✅ " + mention(u) + " is back to active."
	if next == "loa" {
		msg = "✅ " + mention(u) + " is on LOA"
		if until != nil {
			msg += fmt.Sprintf(" until <t:%d:D>", until.Unix())
		}
		msg += "."
	}
	c.Reply(msg + " Logged to the Staff Log.")
}

// replyServiceError replies for a failed staff/players service call and
// reports whether it did.
func replyServiceError(c *Call, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, staff.ErrNotAllowed):
		msg := strings.TrimPrefix(err.Error(), staff.ErrNotAllowed.Error()+": ")
		c.Reply("Not done: " + msg + ".")
	default:
		slog.Error("discord bot: staff command failed", "error", err)
		c.Reply("Something went wrong. Nothing was changed.")
	}
	return true
}

// can checks an extra permission for the caller (beyond the command's own).
func (c *Call) can(ctx context.Context, perm string) bool {
	d, err := auth.Can(ctx, c.Bot.pool, c.PlayerID, perm)
	return err == nil && d == auth.Allowed
}

func ptrFloat(f float64) *float64 { return &f }

func syncCommand() *Command {
	return &Command{
		Def: &discordgo.ApplicationCommand{
			Name: "sync", Description: "Re-apply synced Discord roles now",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "user", Description: "One member", Options: []*discordgo.ApplicationCommandOption{userOption("Who to sync")}},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "all", Description: "Everyone who has linked (can take a while)"},
			},
		},
		Permission: "roles.manage",
		Handler: func(ctx context.Context, c *Call) {
			e := c.Bot.roleSyncEngine()
			if e == nil {
				c.Reply("Role sync isn't running.")
				return
			}
			if c.Subcommand() == "all" {
				c.Defer()
				runCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
				defer cancel()
				p, ch, f := e.SyncAll(runCtx)
				c.Reply(fmt.Sprintf("Synced %d linked players: %d role change(s), %d failure(s). Details on the website's Discord Role Sync page.", p, ch, f))
				return
			}
			target, u := targetPlayer(ctx, c)
			if target == 0 {
				return
			}
			c.Defer()
			res, err := e.SyncPlayer(ctx, target, 0)
			if err != nil {
				slog.Error("discord bot: /sync failed", "error", err)
				c.Reply("Sync failed: " + err.Error())
				return
			}
			var lines []string
			for _, r := range res {
				if r.Platform != "discord" {
					continue
				}
				for _, g := range r.Added {
					lines = append(lines, "+ <@&"+g+">")
				}
				for _, g := range r.Removed {
					lines = append(lines, "− <@&"+g+">")
				}
				for _, e := range r.Errors {
					lines = append(lines, "⚠️ "+e)
				}
			}
			if len(lines) == 0 {
				c.Reply(mention(u) + "'s roles were already correct.")
				return
			}
			c.Reply(mention(u) + ":\n" + strings.Join(lines, "\n"))
		},
	}
}
