package main

import (
	"bufio"
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"website/internal/auth"
	"website/internal/config"
	"website/internal/db"
	"website/internal/discord"
	"website/internal/handlers"
	"website/internal/render"
)

// loadDotEnv applies KEY=VALUE lines from .env (if present) to the process
// environment, without overwriting a variable that's already set -- a real
// env var (production) always wins over the file (local dev convenience).
// Intentionally hand-rolled instead of a dependency: this is ~15 lines and
// the only thing it does is os.Setenv, not worth a third-party package for.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // no .env -- fine, config.Load() falls back to defaults/real env vars
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if _, alreadySet := os.LookupEnv(key); alreadySet {
			continue
		}
		_ = os.Setenv(key, strings.TrimSpace(value))
	}
}

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	loadDotEnv(".env")

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	renderer, err := render.New("web/templates")
	if err != nil {
		return err
	}

	d := &handlers.Deps{
		Pool:   pool,
		Render: renderer,
		Auth:   &auth.Authenticator{Pool: pool, CookieSecure: cfg.CookieSecure},
		Cfg:    cfg,
	}

	// Discord bot (account linking via `/link`) -- optional. Its absence
	// must never stop the website from serving HTTP; see
	// internal/discord/bot.go and docs/WEBSITE.md §9.
	if cfg.DiscordBotToken != "" {
		bot, err := discord.NewBot(cfg.DiscordBotToken, pool, cfg.DiscordGuildID)
		if err != nil {
			slog.Error("discord bot: failed to initialize, continuing without it", "error", err)
		} else if err := bot.Start(ctx); err != nil {
			slog.Error("discord bot: failed to start, continuing without it", "error", err)
		} else {
			defer bot.Stop()
		}
	} else {
		slog.Info("discord bot: DISCORD_BOT_TOKEN not set, /link command disabled")
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(d.Auth.Middleware)

	fileServer := http.FileServer(http.Dir("web/static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	r.Get("/", d.Landing)

	r.Get("/auth/steam/login", d.SteamLogin)
	r.Get("/auth/steam/callback", d.SteamCallback)
	r.Get("/auth/discord/login", d.DiscordLogin)
	r.Get("/auth/discord/callback", d.DiscordCallback)
	r.Post("/auth/logout", d.Logout)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireLogin)
		r.Get("/dashboard", d.Dashboard)
		r.Post("/dashboard/discord/link-code", d.GenerateDiscordLinkCode)
		r.Get("/auth/discord/connect", d.DiscordConnect)

		r.Get("/tickets", d.MyTickets)
		r.Post("/tickets", d.CreateTicket)
		r.Get("/tickets/{id}", d.TicketThread) // ownership/staff check inside -- shared with Support Panel
		r.Post("/tickets/{id}/reply", d.ReplyToTicket)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAdminPanel)
		r.Get("/admin", d.AdminHome)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireSupportPanel)
		r.Get("/support", d.SupportQueue)
		r.Post("/support/tickets/{id}/claim", d.ClaimTicket)
		r.Post("/support/tickets/{id}/close", d.CloseTicket)
		r.Post("/support/tickets/{id}/reopen", d.ReopenTicket)
	})

	srv := &http.Server{Addr: cfg.ListenAddr, Handler: r}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5e9)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("website: listening", "addr", cfg.ListenAddr, "base_url", cfg.SiteBaseURL)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
