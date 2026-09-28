package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"website/internal/auth"
	"website/internal/discord"
)

type discordSettingsData struct {
	Base
	AdminShell
	BotRunning bool
	BotError   string
	BotName    string
	GuildName  string
	Missing    []string
	// HasAdministrator: works, but more access than the bot needs.
	HasAdministrator bool
	Channels         []discord.Channel
	Settings         []settingView
	WelcomeSet       bool
}

type settingView struct {
	discord.SettingDef
	Value string
	// Missing: a channel is saved but no longer exists in the server.
	Missing bool
}

// DiscordSettings is /admin/discord: which channels the bot posts to, its
// toggles and the welcome message (DISCORD_BOT.md §3). Needs bot.admin.
func (d *Deps) DiscordSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := discordSettingsData{Base: baseFrom(r, "Discord Bot"), AdminShell: d.adminShell(r, "discord")}
	if d.Bot == nil {
		data.BotError = "The Discord bot isn't running (DISCORD_BOT_TOKEN not set, or it failed to start). Settings can't be edited until it is."
		d.Render.Render(w, "discord_settings.html", data)
		return
	}
	data.BotName, data.GuildName = d.Bot.Identity()
	chans, err := d.Bot.TextChannels()
	if err != nil {
		slog.Error("discord settings: listing channels failed", "error", err)
		data.BotError = "Couldn't read the Discord server's channels: " + err.Error()
	} else {
		data.BotRunning = true
		data.Channels = chans
	}
	if missing, err := d.Bot.MissingPermissions(); err == nil {
		if len(missing) == 1 && strings.HasPrefix(missing[0], "(bot has Administrator") {
			data.HasAdministrator = true
		} else {
			data.Missing = missing
		}
	}
	values, err := d.Bot.Settings.All(ctx)
	if err != nil {
		slog.Error("discord settings: loading failed", "error", err)
		http.Error(w, "Failed to load Discord settings.", http.StatusInternalServerError)
		return
	}
	exists := map[string]bool{}
	for _, c := range chans {
		exists[c.ID] = true
	}
	for _, def := range discord.SettingDefs {
		v := settingView{SettingDef: def, Value: values[def.Key]}
		v.Missing = def.Kind == "channel" && v.Value != "" && data.BotRunning && !exists[v.Value]
		data.Settings = append(data.Settings, v)
		if def.Key == discord.ChannelWelcome && v.Value != "" && !v.Missing {
			data.WelcomeSet = true
		}
	}
	d.Render.Render(w, "discord_settings.html", data)
}

// DiscordSettingsSave saves the form. Channels must be text channels that
// exist in the server now (or be left unset).
func (d *Deps) DiscordSettingsSave(w http.ResponseWriter, r *http.Request) {
	const back = "/admin/discord"
	if d.Bot == nil {
		http.Redirect(w, r, back+"?error="+errMsg("The Discord bot isn't running."), http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form", http.StatusBadRequest)
		return
	}
	chans, err := d.Bot.TextChannels()
	if err != nil {
		http.Redirect(w, r, back+"?error="+errMsg("Couldn't read the Discord server's channels, so nothing was saved."), http.StatusSeeOther)
		return
	}
	exists := map[string]bool{}
	for _, c := range chans {
		exists[c.ID] = true
	}
	current, err := d.Bot.Settings.All(r.Context())
	if err != nil {
		finishRolesAction(w, r, back, err, "")
		return
	}
	values := map[string]string{}
	for _, def := range discord.SettingDefs {
		v := r.FormValue(def.Key)
		switch def.Kind {
		case "channel":
			// A channel that has since been deleted may be kept as-is
			// (the page flags it) but can't be newly chosen.
			if v != "" && !exists[v] && v != current[def.Key] {
				http.Redirect(w, r, back+"?error="+errMsg("One of the chosen channels doesn't exist any more. Nothing was saved."), http.StatusSeeOther)
				return
			}
		case "toggle":
			if v != "on" {
				v = "off"
			}
		}
		values[def.Key] = v
	}
	sess, _ := auth.FromContext(r.Context())
	err = d.Bot.Settings.Save(r.Context(), sess.PlayerID, values)
	finishRolesAction(w, r, back, err, "Discord settings saved. Changes take effect within a minute.")
}

// DiscordPostWelcome posts (or updates in place) the #welcome message.
func (d *Deps) DiscordPostWelcome(w http.ResponseWriter, r *http.Request) {
	const back = "/admin/discord"
	if d.Bot == nil {
		http.Redirect(w, r, back+"?error="+errMsg("The Discord bot isn't running."), http.StatusSeeOther)
		return
	}
	link, err := d.Bot.PostWelcome(r.Context())
	switch {
	case err == nil:
		http.Redirect(w, r, back+"?notice="+errMsg("Welcome message posted: "+link+". Pin it in Discord so new members see it first."), http.StatusSeeOther)
	case errors.Is(err, discord.ErrNoChannel):
		http.Redirect(w, r, back+"?error="+errMsg("Choose a #welcome channel and save first."), http.StatusSeeOther)
	default:
		slog.Error("discord settings: posting welcome message failed", "error", err)
		http.Redirect(w, r, back+"?error="+errMsg("Couldn't post the welcome message: "+err.Error()), http.StatusSeeOther)
	}
}
