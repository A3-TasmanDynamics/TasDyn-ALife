package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Bot is the Discord bot (docs/DISCORD_BOT.md). It runs inside the website
// process and calls the same code the website does; commands live in
// cmd_*.go and are registered through registry.go.
//
// Ticket thread sync (DISCORD_BOT.md §6) is not built yet -- do not assume
// it's live just because the bot process is running.
type Bot struct {
	session *discordgo.Session
	pool    *pgxpool.Pool
	guildID string
	deps    Deps

	commands map[string]*Command
}

// Deps are website capabilities the bot's commands use, passed in from
// main.go so this package doesn't import the handlers.
type Deps struct {
	// Status returns the public status page's current summary lines, or
	// nil if status monitoring isn't running.
	Status func(ctx context.Context) (headline string, lines []string, err error)
	// OutboxBacklog reports undelivered outbox messages for /bot health.
	OutboxBacklog func(ctx context.Context) (int, error)
}

// NewBot constructs a Bot. Returns an error only for a malformed token --
// actually connecting happens in Start, so main.go can decide what "the bot
// failed to start" should mean (currently: log and keep serving HTTP --
// Discord being unconfigured/unreachable must never take the site down).
func NewBot(token string, pool *pgxpool.Pool, guildID string, deps Deps) (*Bot, error) {
	if token == "" {
		return nil, fmt.Errorf("discord: bot token is empty")
	}
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("discord: creating session: %w", err)
	}
	session.Identify.Intents = discordgo.IntentsGuilds
	b := &Bot{session: session, pool: pool, guildID: guildID, deps: deps}
	b.commands = b.registry()
	return b, nil
}

// Start opens the gateway connection and registers every command in one
// bulk overwrite. That's idempotent: removed commands disappear, changed
// ones update, and nothing is deleted on shutdown -- the old behaviour
// (delete /link in Stop) made the command vanish during every restart.
//
// guildID scopes commands to one server so they update instantly; an empty
// guildID registers them globally, which Discord can take up to an hour to
// propagate.
func (b *Bot) Start(ctx context.Context) error {
	b.session.AddHandler(b.handleInteraction)

	if err := b.session.Open(); err != nil {
		return fmt.Errorf("discord: opening gateway session: %w", err)
	}

	defs := make([]*discordgo.ApplicationCommand, 0, len(b.commands))
	for _, c := range b.commands {
		defs = append(defs, c.Def)
	}
	if _, err := b.session.ApplicationCommandBulkOverwrite(b.session.State.User.ID, b.guildID, defs); err != nil {
		_ = b.session.Close()
		return fmt.Errorf("discord: registering commands: %w", err)
	}

	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, "/"+d.Name)
	}
	slog.Info("discord bot: connected", "guild_id", b.guildID, "commands", strings.Join(names, " "))
	return nil
}

// Health reports whether the gateway session is connected and ready, plus
// its last heartbeat round-trip -- used by the public status page.
func (b *Bot) Health() (bool, time.Duration) {
	return b.session.DataReady, b.session.HeartbeatLatency()
}

func (b *Bot) Stop() {
	_ = b.session.Close()
}

// Session exposes the gateway session to other parts of the website that
// send through the bot (the outbox worker).
func (b *Bot) Session() *discordgo.Session { return b.session }
