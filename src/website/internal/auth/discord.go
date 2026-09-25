// Discord OAuth2 ("Connect Discord" / "Sign in with Discord") -- real
// OAuth2 (authorization code grant), unlike Steam's OpenID 2.0 above. Two
// different protocols for two different providers; do not try to unify
// them behind one abstraction, that's exactly the kind of premature
// generalization that makes each flow harder to verify independently.
// Endpoints verified against Discord's own developer docs
// (docs.discord.com/developers/topics/oauth2) before writing this.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	discordAuthorizeURL = "https://discord.com/api/oauth2/authorize"
	discordTokenURL     = "https://discord.com/api/v10/oauth2/token"
	discordUserURL      = "https://discord.com/api/v10/users/@me"

	// StateCookieName carries the CSRF state token between the redirect to
	// Discord and the callback. IntentCookieName carries whether this
	// round-trip is a fresh login or linking an already-logged-in account
	// -- Discord's OAuth2 doesn't have a first-class way to carry
	// application-specific intent through the flow, so a second short-lived
	// cookie does it instead of overloading the `state` param.
	StateCookieName  = "discord_oauth_state"
	IntentCookieName = "discord_oauth_intent"

	IntentLogin   = "login"
	IntentConnect = "connect"
)

// DiscordOAuth holds this app's Discord application credentials -- see
// config.Config.DiscordOAuthClientID/Secret. A different credential pair
// from the ticket bot's DiscordBotToken (internal/discord/bot.go).
type DiscordOAuth struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string // this site's own /auth/discord/callback URL
}

func randomState() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// AuthorizeURL builds the URL to send the browser to. scope is just
// "identify" -- this app only needs the Discord user's ID and username, not
// their email, guild list, or anything else.
func (d DiscordOAuth) AuthorizeURL(state string) string {
	v := url.Values{
		"response_type": {"code"},
		"client_id":     {d.ClientID},
		"scope":         {"identify"},
		"state":         {state},
		"redirect_uri":  {d.RedirectURL},
		"prompt":        {"none"},
	}
	return discordAuthorizeURL + "?" + v.Encode()
}

// ExchangeAndFetchUser trades the authorization code for an access token,
// then immediately uses it to fetch the authenticated user's ID/username.
// The access token itself is discarded afterward -- this app only needs the
// identity fact, not ongoing API access on the user's behalf, so there's no
// reason to persist a token that would just be one more credential to leak.
func (d DiscordOAuth) ExchangeAndFetchUser(ctx context.Context, code string) (discordID, username string, err error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {d.RedirectURL},
		"client_id":     {d.ClientID},
		"client_secret": {d.ClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, discordTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("discord: token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", "", fmt.Errorf("discord: token exchange returned %d: %s", resp.StatusCode, body)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", "", fmt.Errorf("discord: decoding token response: %w", err)
	}

	userReq, err := http.NewRequestWithContext(ctx, http.MethodGet, discordUserURL, nil)
	if err != nil {
		return "", "", err
	}
	userReq.Header.Set("Authorization", tokenResp.TokenType+" "+tokenResp.AccessToken)

	userResp, err := client.Do(userReq)
	if err != nil {
		return "", "", fmt.Errorf("discord: user fetch request failed: %w", err)
	}
	defer userResp.Body.Close()

	if userResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(userResp.Body, 2048))
		return "", "", fmt.Errorf("discord: user fetch returned %d: %s", userResp.StatusCode, body)
	}

	var user struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	if err := json.NewDecoder(userResp.Body).Decode(&user); err != nil {
		return "", "", fmt.Errorf("discord: decoding user response: %w", err)
	}

	return user.ID, user.Username, nil
}

// SetOAuthCookies stashes the CSRF state and the flow's intent
// (login/connect) as short-lived cookies before redirecting to Discord.
func SetOAuthCookies(w http.ResponseWriter, secure bool, state, intent string) {
	opts := func(name, value string) *http.Cookie {
		return &http.Cookie{
			Name:     name,
			Value:    value,
			Path:     "/",
			MaxAge:   600, // 10 minutes -- this round-trip should take seconds
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		}
	}
	http.SetCookie(w, opts(StateCookieName, state))
	http.SetCookie(w, opts(IntentCookieName, intent))
}

// ClearOAuthCookies removes the state/intent cookies once the callback has
// consumed them -- they're single-use.
func ClearOAuthCookies(w http.ResponseWriter, secure bool) {
	for _, name := range []string{StateCookieName, IntentCookieName} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: "", Path: "/", MaxAge: -1,
			HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
		})
	}
}

// RandomState is exported for handlers to call when starting the flow.
func RandomState() (string, error) { return randomState() }
