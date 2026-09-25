// Package config loads this app's runtime configuration from environment
// variables. Env vars, not an .ini file like the C++ extension's config.ini
// -- this is a long-running server process typically deployed under
// systemd/Docker, where env vars are the idiomatic way in, not a config
// file the process has to be told where to find.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	// DatabaseURL is a standard libpq connection string, e.g.
	// "postgres://alife_admin:alife_admin@127.0.0.1:5432/alife_db".
	DatabaseURL string

	ListenAddr string

	// SiteBaseURL is this site's own externally-reachable origin, e.g.
	// "https://play.tasmandynamics.com" in production or
	// "http://localhost:8080" in dev. Used to build the Steam OpenID
	// return_to/realm values -- Steam redirects the browser back here.
	SiteBaseURL string

	// CookieSecure gates the session cookie's Secure flag. Must be true in
	// production (HTTPS only, per docs/WEBSITE.md §10) -- false is only for
	// local HTTP development, where a browser would refuse a Secure cookie
	// over plain http://.
	CookieSecure bool

	// Discord webhook URLs, one per log category (docs/WEBSITE.md §9). Any
	// left empty simply means that category doesn't post to Discord --
	// never a startup failure, since Discord being unconfigured must not
	// block the site from running.
	DiscordStaffLogWebhook  string
	DiscordTicketLogWebhook string
	DiscordAntiCheatWebhook string

	// Discord bot token + IDs for the two-way ticket sync (docs/WEBSITE.md
	// §9). Left empty until the bot is actually wired up -- see
	// internal/discord/bot.go.
	DiscordBotToken         string
	DiscordGuildID          string
	DiscordTicketCategoryID string

	// Discord OAuth2 app credentials for "Sign in / Connect with Discord"
	// (internal/auth/discord.go) -- a DIFFERENT credential pair from
	// DiscordBotToken above. The bot token authenticates the ticket-sync
	// bot's own gateway/API calls; these authenticate the OAuth2
	// authorization-code flow a player's browser goes through. Both come
	// from the same Discord application, but are used for unrelated
	// purposes and must not be confused with each other.
	DiscordOAuthClientID     string
	DiscordOAuthClientSecret string
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:             getenv("DATABASE_URL", "postgres://alife_admin:alife_admin@127.0.0.1:5432/alife_db"),
		ListenAddr:              getenv("LISTEN_ADDR", ":8080"),
		SiteBaseURL:             getenv("SITE_BASE_URL", "http://localhost:8080"),
		CookieSecure:            getenv("COOKIE_SECURE", "false") == "true",
		DiscordStaffLogWebhook:  os.Getenv("DISCORD_STAFF_LOG_WEBHOOK"),
		DiscordTicketLogWebhook: os.Getenv("DISCORD_TICKET_LOG_WEBHOOK"),
		DiscordAntiCheatWebhook: os.Getenv("DISCORD_ANTICHEAT_WEBHOOK"),
		DiscordBotToken:         os.Getenv("DISCORD_BOT_TOKEN"),
		DiscordGuildID:          os.Getenv("DISCORD_GUILD_ID"),
		DiscordTicketCategoryID: os.Getenv("DISCORD_TICKET_CATEGORY_ID"),
	}

	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL must not be empty")
	}
	if c.SiteBaseURL == "" {
		return c, fmt.Errorf("SITE_BASE_URL must not be empty")
	}

	return c, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
