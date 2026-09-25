package discord

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/auth"
)

// Bot is the two-way Discord integration (docs/WEBSITE.md §9) -- a
// persistent gateway connection, unlike webhook.go's fire-and-forget HTTP
// POSTs. Currently implements account linking (`/link <code>`, the
// Discord-side counterpart to the member portal's OAuth "Connect Discord"
// button). Ticket thread creation/mirroring is designed in WEBSITE.md §9
// but not yet wired here -- see the TODO at the bottom of this file; do not
// assume ticket sync is live just because the bot process is running.
type Bot struct {
	session *discordgo.Session
	pool    *pgxpool.Pool
	guildID string

	registeredCommandID string
}

// NewBot constructs a Bot. Returns an error only for a malformed token --
// actually connecting happens in Start, so main.go can decide what "the bot
// failed to start" should mean (currently: log and keep serving HTTP,
// same "Discord being unconfigured/unreachable must never take the site
// down" rule as webhook.go).
func NewBot(token string, pool *pgxpool.Pool, guildID string) (*Bot, error) {
	if token == "" {
		return nil, fmt.Errorf("discord: bot token is empty")
	}
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("discord: creating session: %w", err)
	}
	return &Bot{session: session, pool: pool, guildID: guildID}, nil
}

// Start opens the gateway connection and registers the /link command.
// guildID scopes the command to one server for near-instant availability in
// dev/staging; an empty guildID registers it globally instead, which can
// take up to an hour to propagate on Discord's side -- expected, not a bug,
// if a freshly-registered global command doesn't show up right away.
func (b *Bot) Start(ctx context.Context) error {
	b.session.AddHandler(b.handleInteraction)

	if err := b.session.Open(); err != nil {
		return fmt.Errorf("discord: opening gateway session: %w", err)
	}

	cmd, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, b.guildID, &discordgo.ApplicationCommand{
		Name:        "link",
		Description: "Link your Discord account to your TasDyn-ALife player account",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "code",
				Description: "The code shown on the website's Settings page",
				Required:    true,
			},
		},
	})
	if err != nil {
		_ = b.session.Close()
		return fmt.Errorf("discord: registering /link command: %w", err)
	}
	b.registeredCommandID = cmd.ID

	slog.Info("discord bot: connected and /link registered", "guild_id", b.guildID)
	return nil
}

func (b *Bot) Stop() {
	if b.registeredCommandID != "" && b.session.State.User != nil {
		_ = b.session.ApplicationCommandDelete(b.session.State.User.ID, b.guildID, b.registeredCommandID)
	}
	_ = b.session.Close()
}

func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	data := i.ApplicationCommandData()
	if data.Name != "link" {
		return
	}

	var code string
	for _, opt := range data.Options {
		if opt.Name == "code" {
			code = opt.StringValue()
		}
	}

	// The invoking user's Discord identity comes from Discord itself
	// (i.Member.User over a guild interaction, i.User over a DM) -- never
	// taken from anything the command's own arguments could claim, which is
	// exactly the property that makes this flow trustworthy without a
	// second OAuth round-trip.
	discordUser := i.User
	if discordUser == nil && i.Member != nil {
		discordUser = i.Member.User
	}
	if discordUser == nil {
		b.reply(s, i, "Couldn't determine your Discord identity -- try again in a server channel or DM.")
		return
	}

	_, err := auth.ConsumeLinkCode(context.Background(), b.pool, code, discordUser.ID, discordUser.Username)
	switch {
	case err == nil:
		b.reply(s, i, "✅ Linked! Your website account is now connected to this Discord account.")
	case err == auth.ErrLinkCodeInvalid:
		b.reply(s, i, "That code is invalid or has expired. Generate a new one from the website's Settings page.")
	case err == auth.ErrDiscordAlreadyLinked:
		b.reply(s, i, "This Discord account is already linked to a different player account.")
	default:
		slog.Error("discord bot: /link failed", "error", err)
		b.reply(s, i, "Something went wrong linking your account -- please try again shortly.")
	}
}

func (b *Bot) reply(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral, // only the invoking user sees the result -- it's not chat content
		},
	})
	if err != nil {
		slog.Error("discord bot: replying to interaction failed", "error", err)
	}
}

// TODO(WEBSITE.md §9, Phase W): two-way support-ticket sync --
// CreateThreadForTicket(ticketID) on new ticket, PostMessageToThread(...)
// on new web reply, and a MessageCreate handler here mirroring a staff
// reply from the thread back into support_ticket_messages. Not built yet;
// account linking above was the concretely-requested, self-contained piece
// to ship first.
