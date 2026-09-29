package main

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"website/internal/audit"
	"website/internal/auth"
	"website/internal/config"
	"website/internal/csrf"
	"website/internal/db"
	"website/internal/discord"
	"website/internal/handlers"
	"website/internal/render"
	"website/internal/rolesync"
	"website/internal/staff"
	"website/internal/status"
	"website/internal/steam"
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
	d.Auth.Denied = d.Denied

	// Discord bot (account linking via `/link`) -- optional. Its absence
	// must never stop the website from serving HTTP; see
	// internal/discord/bot.go and docs/WEBSITE.md §9.
	var discordHealth func() (bool, time.Duration)
	if cfg.DiscordBotToken != "" {
		botDeps := discord.Deps{
			// Lazy: the status monitor is created below, after the bot,
			// because it in turn checks the bot's health.
			Status: func(ctx context.Context) (status.Snapshot, error) {
				if d.StatusMonitor == nil {
					return status.Snapshot{}, errors.New("status monitor not running")
				}
				return d.StatusMonitor.Snapshot(ctx, time.UTC)
			},
			OutboxBacklog: func(ctx context.Context) (int, error) { return discord.Backlog(ctx, pool) },
			SiteBaseURL:   cfg.SiteBaseURL,
		}
		bot, err := discord.NewBot(cfg.DiscordBotToken, pool, cfg.DiscordGuildID, botDeps)
		if err != nil {
			slog.Error("discord bot: failed to initialize, continuing without it", "error", err)
		} else if err := bot.Start(ctx); err != nil {
			slog.Error("discord bot: failed to start, continuing without it", "error", err)
		} else {
			defer bot.Stop()
			discordHealth = bot.Health
			// Reliable DMs/posts (docs/DISCORD_BOT.md §8). Messages queued
			// while the bot is down wait in the table until next start.
			go (&discord.Outbox{Pool: pool, Sender: bot.Session()}).Run(ctx)
			// Role sync (docs/INTEGRATIONS.md §2.3): does nothing until roles
			// are mapped on /admin/role-sync.
			d.Bot = bot
			d.RoleSyncEngine = &rolesync.Engine{Pool: pool, Adapters: []rolesync.Adapter{discord.RoleSyncAdapter{Bot: bot}}}
			bot.SetRoleSync(d.RoleSyncEngine)
			go d.RoleSyncEngine.Run(ctx)
			// Live #server-status message, up/down posts and presence (§7).
			go bot.RunStatus(ctx)
		}
	} else {
		slog.Info("discord bot: DISCORD_BOT_TOKEN not set, /link command disabled")
	}

	// Public status page checks (internal/status) -- one probe per
	// component per minute, recorded to status_checks.
	d.StatusMonitor = status.New(pool, status.Options{
		ListenAddr:    cfg.ListenAddr,
		GameQueryAddr: cfg.GameQueryAddr,
		DiscordHealth: discordHealth,
	})
	go d.StatusMonitor.Run(ctx)

	// Steam profile/ban cache + BattlEye GUID backfill (internal/steam).
	d.Steam = &steam.Refresher{Pool: pool, Client: steam.NewClient(cfg.SteamWebAPIKey)}
	go d.Steam.Run(ctx)

	// Return staff whose LOA end date has passed to active.
	go staff.RunLOASweep(ctx, pool, func(n int64, err error) {
		if err != nil {
			slog.Error("staff: LOA sweep failed", "error", err)
		} else if n > 0 {
			slog.Info("staff: LOAs ended", "count", n)
		}
	})

	// Every staff_log row (from any source) -> #staff-log (internal/audit).
	// Posts through the bot when a #staff-log channel is set on
	// /admin/discord, otherwise through DISCORD_STAFF_LOG_WEBHOOK.
	poster := &audit.Poster{Pool: pool, WebhookURL: cfg.DiscordStaffLogWebhook}
	if d.Bot != nil {
		poster.Via = d.Bot.StaffLogPoster
	}
	go poster.Run(ctx)
	if cfg.GameQueryAddr == "" {
		slog.Info("status: GAME_QUERY_ADDR not set, game server shown as not monitored")
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(d.Auth.Middleware)
	r.Use(csrf.Middleware(cfg.CookieSecure))

	// no-cache (not no-store): the browser still caches the response, but
	// must revalidate with the server on every request instead of serving
	// a stale copy for whatever heuristic period it guesses is safe with no
	// explicit Cache-Control header at all -- exactly what let a player see
	// stale CSS across several style.css updates today with no visible
	// error, just a page that silently looked wrong. http.FileServer
	// already sets ETag/Last-Modified, so revalidation correctly comes
	// back 304 when nothing changed and 200 with the new bytes when it has.
	fileServer := http.FileServer(http.Dir("web/static"))
	r.Handle("/static/*", http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		fileServer.ServeHTTP(w, r)
	})))

	r.Get("/", d.Landing)
	r.Get("/devlog", d.DevlogList)
	r.Get("/devlog/{slug}", d.DevlogPost)
	r.Get("/status", d.Status)
	r.Get("/rules", d.Rules)

	r.Get("/auth/steam/login", d.SteamLogin)
	r.Get("/auth/steam/callback", d.SteamCallback)
	r.Get("/auth/discord/login", d.DiscordLogin)
	r.Get("/auth/discord/callback", d.DiscordCallback)
	r.Post("/auth/logout", d.Logout)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireLogin)
		r.Get("/dashboard", d.Dashboard)
		r.Post("/dashboard/discord/link-code", d.GenerateDiscordLinkCode)
		r.Post("/dashboard/transfer", d.Transfer)
		r.Post("/dashboard/leaderboards", d.SetLeaderboardOptIn)
		r.Post("/dashboard/appeal", d.SubmitAppeal)

		// Recruitment (GAMEPANEL_PARITY §4, §6.2).
		r.Get("/staff/apply", d.StaffApply)
		r.Post("/staff/apply", d.StaffApplySubmit)
		r.Post("/staff/apply/withdraw", d.StaffApplyWithdraw)
		r.Get("/factions", d.FactionApply)
		r.Post("/factions", d.FactionApplySubmit)
		r.Get("/command/{faction}/recruits", d.CommandRecruits)
		r.Post("/command/{faction}/recruits/{id}", d.CommandDecideApp)
		r.Post("/dashboard/gang/invite", d.InviteToGang)
		r.Post("/dashboard/gang/remove", d.RemoveFromGang)
		r.Post("/dashboard/gang/rank", d.SetGangRank)
		r.Get("/auth/discord/connect", d.DiscordConnect)

		// Faction command panel (GAMEPANEL_PARITY §6.1): access comes from
		// the player's faction rank, checked in each handler.
		r.Get("/command", d.CommandHome)
		r.Get("/command/{faction}", d.CommandOverview)
		r.Get("/command/{faction}/roster", d.CommandRoster)
		r.Post("/command/{faction}/recruit", d.CommandRecruit)
		r.Get("/command/{faction}/members/{id}", d.CommandMember)
		r.Post("/command/{faction}/members/{id}/rank", d.CommandSetRank)
		r.Get("/command/{faction}/log", d.CommandLog)

		r.Get("/tickets", d.MyTickets)
		r.Post("/tickets", d.CreateTicket)
		r.Get("/tickets/{id}", d.TicketThread) // ownership/staff check inside -- shared with Support Panel
		r.Post("/tickets/{id}/reply", d.ReplyToTicket)
	})

	r.Group(func(r chi.Router) {
		r.Use(d.Auth.RequireAdminPanel)
		r.Get("/admin", d.AdminHome)
		r.Get("/admin/devlog/new", d.DevlogNewForm)
		r.Post("/admin/devlog/new", d.DevlogCreate)

		// Staff directory (GAMEPANEL_PARITY §2.2-2.3). Viewing needs
		// staff.view; each action checks its own permission inside, and
		// internal/staff enforces the seniority rules.
		r.Group(func(r chi.Router) {
			r.Use(d.Auth.RequirePermission("staff.view"))
			r.Get("/admin/staff", d.StaffList)
			r.Post("/admin/staff/add", d.StaffAdd)
			r.Get("/admin/staff/{id}", d.StaffProfile)
			r.Post("/admin/staff/{id}/rank", d.StaffSetRank)
			r.Post("/admin/staff/{id}/status", d.StaffSetStatus)
			r.Post("/admin/staff/{id}/placement", d.StaffSetPlacement)
			r.Post("/admin/staff/{id}/notes", d.StaffAddNote)
		})

		// Roles & Permissions (GAMEPANEL_PARITY §2.1). The page itself opens
		// for roles.manage OR factions.configure (checked in the handler);
		// rank changes need roles.manage, and internal/roles enforces the
		// level and "only grant what you hold" rules.
		// Staff Log (GAMEPANEL_PARITY §2.4) -- Admin Panel access is enough to read it.
		r.Get("/admin/staff-log", d.StaffLog)
		r.Get("/admin/staff-log.csv", d.StaffLogCSV)

		// Player Lookup (GAMEPANEL_PARITY #20-#24). Actions check their own
		// permissions (players.edit_*, players.compensate[_large]).
		r.Group(func(r chi.Router) {
			r.Use(d.Auth.RequirePermission("players.view"))
			r.Get("/admin/players", d.PlayerLookup)
			r.Post("/admin/players/{id}/faction-level", d.PlayerSetFactionLevel)
			r.Post("/admin/players/{id}/compensate", d.PlayerCompensate)
		})

		r.Get("/admin/roles", d.Roles)
		r.Post("/admin/roles/faction-names", d.RoleFactionNames)
		r.Group(func(r chi.Router) {
			r.Use(d.Auth.RequirePermission("roles.manage"))
			r.Post("/admin/roles/save", d.RoleSave)
			r.Post("/admin/roles/move", d.RoleMove)
			r.Post("/admin/roles/create", d.RoleCreate)
			r.Post("/admin/roles/delete", d.RoleDelete)

			// Discord role sync mapping (INTEGRATIONS §2.3).
			r.Get("/admin/role-sync", d.RoleSync)
			r.Post("/admin/role-sync/save", d.RoleSyncSave)
			r.Post("/admin/role-sync/run", d.RoleSyncRun)
		})

		// Moderation (GAMEPANEL_PARITY §3): cases, bans, anti-cheat. Each
		// write action checks its own permission in the handler.
		r.Group(func(r chi.Router) {
			r.Use(d.Auth.RequirePermission("cases.view"))
			r.Get("/admin/cases", d.CasesList)
			r.Get("/admin/cases/{id}", d.CaseView)
			r.Post("/admin/cases/{id}/entries", d.CaseAddEntry)
			r.Post("/admin/cases/{id}/status", d.CaseSetStatus)
			r.Post("/admin/cases/{id}/participants", d.CaseAddParticipant)
			r.Post("/admin/cases/{id}/points", d.CaseIssuePoints)
			r.Post("/admin/points/{id}/revoke", d.PointsRevoke)
			r.Get("/admin/bans", d.BansList)
			r.Post("/admin/bans/issue", d.BanIssue)
			r.Post("/admin/bans/{id}/lift", d.BanLift)
			r.Post("/admin/appeals/{id}", d.AppealDecide)
		})
		r.Group(func(r chi.Router) {
			r.Use(d.Auth.RequirePermission("cases.lead"))
			r.Get("/admin/cases/new", d.CaseNew)
			r.Post("/admin/cases/new", d.CaseCreate)
		})
		r.Group(func(r chi.Router) {
			r.Use(d.Auth.RequirePermission("anticheat.review"))
			r.Get("/admin/anticheat", d.AntiCheat)
			r.Post("/admin/anticheat/{id}", d.FlagReview)
		})

		r.Group(func(r chi.Router) {
			r.Use(d.Auth.RequirePermission("applications.view"))
			r.Get("/admin/applications", d.Applications)
			r.Post("/admin/applications/{id}", d.ApplicationAction)
		})

		// Server rules editor (public page is /rules).
		r.Group(func(r chi.Router) {
			r.Use(d.Auth.RequirePermission("rules.edit"))
			r.Get("/admin/rules", d.RulesEdit)
			r.Post("/admin/rules/save", d.RulesSave)
		})

		// Discord bot settings: channels, toggles, welcome message
		// (DISCORD_BOT.md §3).
		r.Group(func(r chi.Router) {
			r.Use(d.Auth.RequirePermission("bot.admin"))
			r.Get("/admin/discord", d.DiscordSettings)
			r.Post("/admin/discord/save", d.DiscordSettingsSave)
			r.Post("/admin/discord/welcome", d.DiscordPostWelcome)
		})
	})

	r.Group(func(r chi.Router) {
		r.Use(d.Auth.RequireSupportPanel)
		r.Get("/support", d.SupportDashboard)
		r.Get("/support/tickets", d.SupportQueue)
		r.Post("/support/tickets/{id}/assign", d.AssignTicket)
		r.Post("/support/tickets/{id}/priority", d.SetTicketPriority)
		r.Post("/support/tickets/{id}/category", d.SetTicketCategory)
		r.Post("/support/tickets/{id}/subject", d.SetTicketSubject)
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
