package discord

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"website/internal/auth"
)

// Command is one slash command or context-menu command. Permission, if set,
// is checked against the caller's LINKED website account before Handler
// runs -- never against Discord roles (docs/DISCORD_BOT.md §1 principle 3).
type Command struct {
	Def        *discordgo.ApplicationCommand
	Permission string // auth.Catalogue key; "" = anyone
	Handler    func(ctx context.Context, c *Call)
	// Autocomplete, if set, suggests values for the focused option. It only
	// runs for callers who pass the permission check.
	Autocomplete func(ctx context.Context, c *Call) []*discordgo.ApplicationCommandOptionChoice
}

// Component handles buttons, select menus and modal submits whose
// custom_id is "<Prefix>:<args...>". A custom_id carries only an action and
// record IDs; the permission is checked again on every click, so an old
// button in a channel can't be used by someone who has since lost access
// (DISCORD_BOT.md §4.4).
type Component struct {
	Prefix     string
	Permission string
	Handler    func(ctx context.Context, c *Call, args []string)
}

// Call is one interaction, with the resolved caller.
type Call struct {
	Bot         *Bot
	Interaction *discordgo.InteractionCreate
	User        *discordgo.User
	PlayerID    int64 // 0 = caller hasn't linked a website account
	deferred    bool
}

// registry is the one place commands and components are wired in. Staff
// commands also get Discord-side default permissions so they're hidden
// from regular members in the command picker -- a convenience only; the
// real check is the linked-account permission above.
func (b *Bot) registry() (map[string]*Command, map[string]*Component) {
	cmds := []*Command{
		linkCommand(),
		unlinkCommand(),
		profileCommand(),
		statusCommand(),
		playersCommand(),
		whoisCommand(),
		whoisUserMenu(),
		lookupCommand(),
		promoteCommand(),
		demoteCommand(),
		loaCommand(),
		reinstateCommand(),
		syncCommand(),
		botCommand(),
	}
	comps := []*Component{
		unlinkComponent(),
		onboardingComponent(),
	}
	cm := make(map[string]*Command, len(cmds))
	for _, c := range cmds {
		if c.Permission != "" && c.Def.DefaultMemberPermissions == nil {
			perm := int64(discordgo.PermissionManageMessages)
			c.Def.DefaultMemberPermissions = &perm
		}
		cm[c.Def.Name] = c
	}
	pm := make(map[string]*Component, len(comps))
	for _, c := range comps {
		pm[c.Prefix] = c
	}
	return cm, pm
}

func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var (
		name   string
		perm   string
		args   []string
		cmd    *Command
		comp   *Component
		silent bool // autocomplete: no visible replies, just no suggestions
	)
	switch i.Type {
	case discordgo.InteractionApplicationCommand, discordgo.InteractionApplicationCommandAutocomplete:
		name = i.ApplicationCommandData().Name
		if cmd = b.commands[name]; cmd == nil {
			return
		}
		perm = cmd.Permission
		silent = i.Type == discordgo.InteractionApplicationCommandAutocomplete
		if silent && cmd.Autocomplete == nil {
			return
		}
	case discordgo.InteractionMessageComponent, discordgo.InteractionModalSubmit:
		id := ""
		if i.Type == discordgo.InteractionMessageComponent {
			id = i.MessageComponentData().CustomID
		} else {
			id = i.ModalSubmitData().CustomID
		}
		parts := strings.Split(id, ":")
		name, args = parts[0], parts[1:]
		if comp = b.components[name]; comp == nil {
			b.replyEphemeral(i, "This button is no longer active.")
			return
		}
		perm = comp.Permission
	default:
		return
	}

	call, ok := b.authorize(ctx, i, name, perm, silent)
	if !ok {
		if silent {
			b.autocompleteRespond(i, nil)
		}
		return
	}
	switch {
	case silent:
		b.autocompleteRespond(i, cmd.Autocomplete(ctx, call))
	case cmd != nil:
		cmd.Handler(ctx, call)
	default:
		comp.Handler(ctx, call, args)
	}
}

// authorize resolves the caller from Discord's interaction payload (never
// from anything an argument could claim) and applies perm. On failure it
// has already replied, unless silent.
func (b *Bot) authorize(ctx context.Context, i *discordgo.InteractionCreate, name, perm string, silent bool) (*Call, bool) {
	reply := func(msg string) {
		if !silent {
			b.replyEphemeral(i, msg)
		}
	}
	user := i.User
	if user == nil && i.Member != nil {
		user = i.Member.User
	}
	if user == nil {
		reply("Couldn't determine your Discord identity. Try again from a server channel.")
		return nil, false
	}

	call := &Call{Bot: b, Interaction: i, User: user}
	playerID, found, err := auth.FindPlayerByDiscordID(ctx, b.pool, user.ID)
	if err != nil {
		slog.Error("discord bot: resolving caller failed", "interaction", name, "error", err)
		reply("Something went wrong. Please try again shortly.")
		return nil, false
	}
	if found {
		call.PlayerID = playerID
	}
	if perm == "" {
		return call, true
	}
	if !found {
		reply("Link your Discord to your website account first (`/link`), then try again.")
		return nil, false
	}
	denial, err := auth.Can(ctx, b.pool, playerID, perm)
	if err != nil {
		slog.Error("discord bot: permission check failed", "interaction", name, "error", err)
		reply("Something went wrong checking your permissions.")
		return nil, false
	}
	switch denial {
	case auth.Allowed:
		return call, true
	case auth.DenyInactive:
		reply("You're currently suspended or on leave, so staff commands are unavailable.")
	default:
		reply("You don't have permission to do that.")
	}
	return nil, false
}

// noMentions is used on every message: replies can include player-chosen
// text, and a name like "@everyone" must never ping.
var noMentions = &discordgo.MessageAllowedMentions{}

// replyEphemeral answers so only the caller sees it.
func (b *Bot) replyEphemeral(i *discordgo.InteractionCreate, content string) {
	b.respond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content, Flags: discordgo.MessageFlagsEphemeral, AllowedMentions: noMentions},
	})
}

func (b *Bot) autocompleteRespond(i *discordgo.InteractionCreate, choices []*discordgo.ApplicationCommandOptionChoice) {
	if choices == nil {
		choices = []*discordgo.ApplicationCommandOptionChoice{}
	}
	if len(choices) > 25 {
		choices = choices[:25]
	}
	b.respond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

func (b *Bot) respond(i *discordgo.InteractionCreate, r *discordgo.InteractionResponse) {
	if err := b.session.InteractionRespond(i.Interaction, r); err != nil {
		slog.Error("discord bot: replying to interaction failed", "error", err)
	}
}

// Reply answers the call ephemerally. After Defer it edits the deferred
// "thinking" message instead.
func (c *Call) Reply(content string) { c.ReplyWith(content, nil, nil) }

// ReplyWith is Reply with optional buttons and embeds.
func (c *Call) ReplyWith(content string, components []discordgo.MessageComponent, embeds []*discordgo.MessageEmbed) {
	if c.deferred {
		if _, err := c.Bot.session.InteractionResponseEdit(c.Interaction.Interaction, &discordgo.WebhookEdit{
			Content: &content, Components: &components, Embeds: &embeds, AllowedMentions: noMentions,
		}); err != nil {
			slog.Error("discord bot: editing deferred reply failed", "error", err)
		}
		return
	}
	c.Bot.respond(c.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content, Components: components, Embeds: embeds,
			Flags: discordgo.MessageFlagsEphemeral, AllowedMentions: noMentions,
		},
	})
}

// Defer shows "Bot is thinking…" (only to the caller) so slow work can run
// past Discord's 3-second limit; finish with Reply.
func (c *Call) Defer() {
	c.Bot.respond(c.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	})
	c.deferred = true
}

// Update replaces the message a button was clicked on (used to turn a
// confirmation prompt into its result, removing the buttons).
func (c *Call) Update(content string) {
	c.Bot.respond(c.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{Content: content, Components: []discordgo.MessageComponent{}, AllowedMentions: noMentions},
	})
}

// Options flattens the command's options, descending into a subcommand.
func (c *Call) Options() map[string]*discordgo.ApplicationCommandInteractionDataOption {
	out := map[string]*discordgo.ApplicationCommandInteractionDataOption{}
	opts := c.Interaction.ApplicationCommandData().Options
	if len(opts) == 1 && (opts[0].Type == discordgo.ApplicationCommandOptionSubCommand || opts[0].Type == discordgo.ApplicationCommandOptionSubCommandGroup) {
		opts = opts[0].Options
	}
	for _, o := range opts {
		out[o.Name] = o
	}
	return out
}

// Subcommand returns the invoked subcommand's name, or "".
func (c *Call) Subcommand() string {
	opts := c.Interaction.ApplicationCommandData().Options
	if len(opts) == 1 && opts[0].Type == discordgo.ApplicationCommandOptionSubCommand {
		return opts[0].Name
	}
	return ""
}

// Focused returns the option being autocompleted and its typed text.
func (c *Call) Focused() (name, value string) {
	for n, o := range c.Options() {
		if o.Focused {
			v, _ := o.Value.(string)
			return n, v
		}
	}
	return "", ""
}

// TargetUser is the user a command's "user" option or a user context menu
// points at.
func (c *Call) TargetUser() *discordgo.User {
	data := c.Interaction.ApplicationCommandData()
	if data.CommandType == discordgo.UserApplicationCommand && data.Resolved != nil {
		return data.Resolved.Users[data.TargetID]
	}
	o := c.Options()["user"]
	if o == nil {
		return nil
	}
	id, _ := o.Value.(string)
	if data.Resolved != nil && data.Resolved.Users[id] != nil {
		return data.Resolved.Users[id]
	}
	return &discordgo.User{ID: id}
}
