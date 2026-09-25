package handlers

import (
	"log/slog"
	"net/http"

	"website/internal/auth"
)

// --- Steam ("Sign in with Steam") ---

func (d *Deps) SteamLogin(w http.ResponseWriter, r *http.Request) {
	returnTo := d.Cfg.SiteBaseURL + "/auth/steam/callback"
	http.Redirect(w, r, auth.BeginLoginURL(returnTo, d.Cfg.SiteBaseURL), http.StatusSeeOther)
}

func (d *Deps) SteamCallback(w http.ResponseWriter, r *http.Request) {
	steamID, err := auth.VerifyCallback(r.Context(), r.URL.Query())
	if err != nil {
		slog.Warn("steam login failed", "error", err)
		http.Redirect(w, r, "/?error="+errMsg("Steam sign-in failed. Please try again."), http.StatusSeeOther)
		return
	}

	playerID, err := auth.FindOrCreatePlayerBySteamUID(r.Context(), d.Pool, steamID)
	if err != nil {
		slog.Error("steam login: find/create player failed", "error", err)
		http.Redirect(w, r, "/?error="+errMsg("Something went wrong signing you in."), http.StatusSeeOther)
		return
	}

	if err := d.Auth.CreateSession(r.Context(), w, r, playerID); err != nil {
		slog.Error("steam login: create session failed", "error", err)
		http.Redirect(w, r, "/?error="+errMsg("Something went wrong signing you in."), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (d *Deps) Logout(w http.ResponseWriter, r *http.Request) {
	d.Auth.Logout(r.Context(), w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// --- Discord ("Connect Discord" / "Sign in with Discord") ---

func (d *Deps) discordOAuth() auth.DiscordOAuth {
	return auth.DiscordOAuth{
		ClientID:     d.Cfg.DiscordOAuthClientID,
		ClientSecret: d.Cfg.DiscordOAuthClientSecret,
		RedirectURL:  d.Cfg.SiteBaseURL + "/auth/discord/callback",
	}
}

// DiscordLogin starts the OAuth2 flow as an alternate LOGIN method -- only
// valid for a Discord account that's already linked to an existing player
// (see auth.FindPlayerByDiscordID); a brand-new visitor must still start
// with Steam, since that's what creates a players row at all.
func (d *Deps) DiscordLogin(w http.ResponseWriter, r *http.Request) {
	d.beginDiscordFlow(w, r, auth.IntentLogin)
}

// DiscordConnect starts the OAuth2 flow to LINK Discord to the
// already-logged-in player's account -- requires RequireLogin upstream.
func (d *Deps) DiscordConnect(w http.ResponseWriter, r *http.Request) {
	d.beginDiscordFlow(w, r, auth.IntentConnect)
}

func (d *Deps) beginDiscordFlow(w http.ResponseWriter, r *http.Request, intent string) {
	if d.Cfg.DiscordOAuthClientID == "" {
		http.Redirect(w, r, "/?error="+errMsg("Discord sign-in isn't configured on this server yet."), http.StatusSeeOther)
		return
	}
	state, err := auth.RandomState()
	if err != nil {
		http.Redirect(w, r, "/?error="+errMsg("Something went wrong."), http.StatusSeeOther)
		return
	}
	auth.SetOAuthCookies(w, d.Cfg.CookieSecure, state, intent)
	http.Redirect(w, r, d.discordOAuth().AuthorizeURL(state), http.StatusSeeOther)
}

func (d *Deps) DiscordCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err1 := r.Cookie(auth.StateCookieName)
	intentCookie, err2 := r.Cookie(auth.IntentCookieName)
	auth.ClearOAuthCookies(w, d.Cfg.CookieSecure)

	if err1 != nil || err2 != nil || r.URL.Query().Get("state") != stateCookie.Value {
		http.Redirect(w, r, "/?error="+errMsg("Discord sign-in session expired -- please try again."), http.StatusSeeOther)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/?error="+errMsg("Discord sign-in was cancelled."), http.StatusSeeOther)
		return
	}

	discordID, username, err := d.discordOAuth().ExchangeAndFetchUser(r.Context(), code)
	if err != nil {
		slog.Warn("discord oauth failed", "error", err)
		http.Redirect(w, r, "/?error="+errMsg("Discord sign-in failed. Please try again."), http.StatusSeeOther)
		return
	}

	switch intentCookie.Value {
	case auth.IntentConnect:
		sess, ok := auth.FromContext(r.Context())
		if !ok {
			http.Redirect(w, r, "/?login_required=1", http.StatusSeeOther)
			return
		}
		err := auth.LinkDiscordToPlayer(r.Context(), d.Pool, sess.PlayerID, discordID, username)
		if err == auth.ErrDiscordAlreadyLinked {
			http.Redirect(w, r, "/dashboard?error="+errMsg("That Discord account is already linked to another player."), http.StatusSeeOther)
			return
		}
		if err != nil {
			slog.Error("discord connect: link failed", "error", err)
			http.Redirect(w, r, "/dashboard?error="+errMsg("Something went wrong linking Discord."), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/dashboard?notice="+errMsg("Discord account connected."), http.StatusSeeOther)

	case auth.IntentLogin:
		playerID, found, err := auth.FindPlayerByDiscordID(r.Context(), d.Pool, discordID)
		if err != nil {
			slog.Error("discord login: lookup failed", "error", err)
			http.Redirect(w, r, "/?error="+errMsg("Something went wrong signing you in."), http.StatusSeeOther)
			return
		}
		if !found {
			http.Redirect(w, r, "/?error="+errMsg("No account is linked to this Discord user yet -- sign in with Steam first, then connect Discord from your Dashboard."), http.StatusSeeOther)
			return
		}
		if err := d.Auth.CreateSession(r.Context(), w, r, playerID); err != nil {
			slog.Error("discord login: create session failed", "error", err)
			http.Redirect(w, r, "/?error="+errMsg("Something went wrong signing you in."), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)

	default:
		http.Redirect(w, r, "/?error="+errMsg("Discord sign-in session expired -- please try again."), http.StatusSeeOther)
	}
}
