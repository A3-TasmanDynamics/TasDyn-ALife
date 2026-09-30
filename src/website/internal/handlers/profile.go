package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"website/internal/auth"
	"website/internal/cases"
	"website/internal/profile"
	"website/internal/steam"
)

type profileData struct {
	Base
	Profile    profile.Profile
	History    profile.History
	Tab        string // "account" or "moderation"
	DiscordOn  bool   // Discord OAuth configured
	RoleSyncOn bool   // the bot's role sync is running
	SteamOn    bool   // a Steam Web API key is set
}

// CaseType labels a case type for the profile template.
func (profileData) CaseType(key string) string { return cases.TypeLabel(key) }

// Profile is the player's own profile: account details and linked
// accounts, or (?tab=moderation) their moderation history.
func (d *Deps) Profile(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	data := profileData{Base: baseFrom(r, "Your profile"), Tab: "account",
		DiscordOn: d.Cfg.DiscordOAuthClientID != "", RoleSyncOn: d.RoleSyncEngine != nil,
		SteamOn: d.Steam != nil && d.Steam.Client != nil && d.Steam.Client.Key != ""}
	if r.URL.Query().Get("tab") == "moderation" {
		data.Tab = "moderation"
		data.Title = "Moderation history"
	}
	var err error
	if data.Profile, err = profile.Load(r.Context(), d.Pool, sess.PlayerID); err != nil {
		slog.Error("profile: load failed", "error", err)
		http.Error(w, "Failed to load your profile.", http.StatusInternalServerError)
		return
	}
	if data.History, err = profile.Moderation(r.Context(), d.Pool, sess.PlayerID); err != nil {
		slog.Error("profile: moderation history failed", "error", err)
		data.Error = "Couldn't load your moderation history."
	}
	d.Render.Render(w, "profile.html", data)
}

func profileBack(w http.ResponseWriter, r *http.Request, err error, notice string) {
	if err != nil {
		msg := "Something went wrong. Please try again."
		if profile.IsUserError(err) {
			msg = err.Error()
		} else {
			slog.Error("profile: update failed", "path", r.URL.Path, "error", err)
		}
		http.Redirect(w, r, "/profile?error="+errMsg(msg), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/profile?notice="+errMsg(notice), http.StatusSeeOther)
}

// ProfileName changes (or clears) the player's website username.
func (d *Deps) ProfileName(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	name := strings.TrimSpace(r.FormValue("display_name"))
	if r.FormValue("reset") == "1" {
		name = ""
	}
	err := profile.SetDisplayName(r.Context(), d.Pool, sess.PlayerID, name)
	notice := "Username updated."
	if name == "" {
		notice = "Username reset to your in-game name."
	}
	profileBack(w, r, err, notice)
}

// ProfileTeamSpeak links, replaces or unlinks the player's TeamSpeak
// identity, then re-syncs their groups.
func (d *Deps) ProfileTeamSpeak(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	uid := r.FormValue("teamspeak_uid")
	if r.FormValue("unlink") == "1" {
		uid = ""
	}
	if err := profile.SetTeamSpeak(r.Context(), d.Pool, sess.PlayerID, uid); err != nil {
		profileBack(w, r, err, "")
		return
	}
	if uid == "" {
		profileBack(w, r, nil, "TeamSpeak unlinked.")
		return
	}
	d.syncRolesAsync(sess.PlayerID)
	profileBack(w, r, nil, "TeamSpeak identity saved. Your server groups follow your ranks once TeamSpeak sync is running.")
}

// resyncLimit stops the re-sync buttons being hammered: one per player per
// kind a minute.
var resyncLimit = struct {
	sync.Mutex
	last map[string]time.Time
}{last: map[string]time.Time{}}

func resyncAllowed(kind string, playerID int64) bool {
	key := fmt.Sprintf("%s:%d", kind, playerID)
	resyncLimit.Lock()
	defer resyncLimit.Unlock()
	if t, ok := resyncLimit.last[key]; ok && time.Since(t) < time.Minute {
		return false
	}
	resyncLimit.last[key] = time.Now()
	return true
}

// ProfileSteamSync refreshes the player's Steam name, avatar and ban data.
func (d *Deps) ProfileSteamSync(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	if !resyncAllowed("steam", sess.PlayerID) {
		profileBack(w, r, &profile.UserError{Msg: "You re-synced Steam a moment ago. Try again in a minute."}, "")
		return
	}
	p, err := profile.Load(r.Context(), d.Pool, sess.PlayerID)
	if err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		err = d.Steam.Refresh(ctx, p.UID)
		cancel()
	}
	if errors.Is(err, steam.ErrNoAPIKey) {
		err = &profile.UserError{Msg: "Steam sync isn't configured on this server yet."}
	} else if err != nil {
		slog.Warn("profile: steam refresh failed", "error", err)
		err = &profile.UserError{Msg: "Steam didn't answer. Please try again shortly."}
	}
	profileBack(w, r, err, "Steam profile re-synced.")
}

// ProfileDiscordSync re-applies the player's Discord roles.
func (d *Deps) ProfileDiscordSync(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	if d.RoleSyncEngine == nil {
		profileBack(w, r, &profile.UserError{Msg: "Role sync isn't running right now (the Discord bot is offline)."}, "")
		return
	}
	if !resyncAllowed("discord", sess.PlayerID) {
		profileBack(w, r, &profile.UserError{Msg: "You re-synced your roles a moment ago. Try again in a minute."}, "")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	res, err := d.RoleSyncEngine.SyncPlayer(ctx, sess.PlayerID, 0)
	if err != nil {
		slog.Error("profile: role sync failed", "error", err)
		profileBack(w, r, &profile.UserError{Msg: "Role sync failed. Please try again shortly."}, "")
		return
	}
	added, removed, failed := 0, 0, 0
	for _, x := range res {
		added += len(x.Added)
		removed += len(x.Removed)
		failed += len(x.Errors)
	}
	switch {
	case len(res) == 0:
		profileBack(w, r, nil, "Nothing to sync: link Discord and join the Discord server first.")
	case failed > 0:
		profileBack(w, r, &profile.UserError{Msg: fmt.Sprintf("Roles partly synced (%d added, %d removed, %d failed). Staff can see the details.", added, removed, failed)}, "")
	case added+removed == 0:
		profileBack(w, r, nil, "Your roles were already up to date.")
	default:
		profileBack(w, r, nil, fmt.Sprintf("Roles re-synced: %d added, %d removed.", added, removed))
	}
}

// reconnectDiscord finishes the profile page's Discord OAuth: when the
// player links a different account, the old one's synced roles are
// stripped first, then roles are synced to the new one.
func (d *Deps) reconnectDiscord(w http.ResponseWriter, r *http.Request, playerID int64, discordID, username string) {
	ctx := r.Context()
	var old string
	d.Pool.QueryRow(ctx, `SELECT COALESCE(discord_id, '') FROM players WHERE id = $1`, playerID).Scan(&old)
	if old != "" && old != discordID && d.RoleSyncEngine != nil {
		if _, err := d.RoleSyncEngine.Strip(ctx, playerID, "discord", old); err != nil {
			slog.Warn("profile: stripping old discord roles failed", "error", err)
		}
	}
	err := auth.LinkDiscordToPlayer(ctx, d.Pool, playerID, discordID, username)
	if errors.Is(err, auth.ErrDiscordAlreadyLinked) {
		profileBack(w, r, &profile.UserError{Msg: "That Discord account is already linked to another player."}, "")
		return
	}
	if err != nil {
		profileBack(w, r, err, "")
		return
	}
	d.syncRolesAsync(playerID)
	if old != "" && old != discordID {
		profileBack(w, r, nil, "Discord account switched to @"+username+". Your roles are moving across.")
		return
	}
	profileBack(w, r, nil, "Discord re-synced as @"+username+".")
}

func (d *Deps) syncRolesAsync(playerID int64) {
	if d.RoleSyncEngine == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if _, err := d.RoleSyncEngine.SyncPlayer(ctx, playerID, 0); err != nil {
			slog.Warn("profile: role sync failed", "player", playerID, "error", err)
		}
	}()
}
