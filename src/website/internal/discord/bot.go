package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/rolesync"
	"website/internal/status"
)

// Bot is the Discord bot (docs/DISCORD_BOT.md). It runs inside the website
// process and calls the same code the website does; commands live in
// cmd_*.go, events in events.go, and both are wired through registry.go.
//
// Ticket thread sync (DISCORD_BOT.md §6) is not built yet -- do not assume
// it's live just because the bot process is running.
type Bot struct {
	session *discordgo.Session
	pool    *pgxpool.Pool
	guildID string
	deps    Deps

	commands   map[string]*Command
	components map[string]*Component

	// Settings is the admin-configured channel layout (discord_settings).
	Settings *Settings

	mu       sync.RWMutex
	roleSync *rolesync.Engine
}

// Deps are website capabilities the bot uses, passed in from main.go.
type Deps struct {
	// Status returns the public status page's current snapshot; nil if
	// status monitoring isn't running.
	Status func(ctx context.Context) (status.Snapshot, error)
	// OutboxBacklog reports undelivered outbox messages for /bot health.
	OutboxBacklog func(ctx context.Context) (int, error)
	// SiteBaseURL is the website's public origin, for links in replies.
	SiteBaseURL string
}

// Intents: Guilds (channels, roles), Server Members (joins and role
// changes, for onboarding and drift detection -- privileged, enabled in the
// developer portal) and Guild Moderation (audit-log entries, for logging
// kicks/bans/timeouts done directly in Discord).
const intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers | discordgo.IntentsGuildBans

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
	session.Identify.Intents = intents
	b := &Bot{session: session, pool: pool, guildID: guildID, deps: deps, Settings: &Settings{pool: pool}}
	b.commands, b.components = b.registry()
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
	b.addEventHandlers()

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
		if d.Type == discordgo.UserApplicationCommand {
			names = append(names, "["+d.Name+"]")
		} else {
			names = append(names, "/"+d.Name)
		}
	}
	slog.Info("discord bot: connected", "guild_id", b.guildID, "commands", strings.Join(names, " "))
	return nil
}

// SetRoleSync attaches the role-sync engine (built after the bot, because
// its Discord adapter needs the bot). Nil until then.
func (b *Bot) SetRoleSync(e *rolesync.Engine) {
	b.mu.Lock()
	b.roleSync = e
	b.mu.Unlock()
}

func (b *Bot) roleSyncEngine() *rolesync.Engine {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.roleSync
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

// siteURL joins the website's base URL and path.
func (b *Bot) siteURL(path string) string {
	return strings.TrimRight(b.deps.SiteBaseURL, "/") + path
}
