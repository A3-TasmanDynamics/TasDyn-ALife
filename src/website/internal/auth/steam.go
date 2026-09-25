// Package auth implements "Sign in with Steam" -- classic OpenID 2.0, not
// OAuth2 (Steam has never offered OAuth2 for this; there is no
// client_id/secret or token exchange). The flow is a signed assertion in a
// callback query string, verified by a stateless round-trip back to
// Steam's own endpoint. Verified against Steam's own Web API docs
// (partner.steamgames.com/doc/features/auth) before writing this, not
// assumed -- same verify-don't-assume discipline this project applies to
// Arma commands, applied here to an equally load-bearing external contract.
package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// steamOpenIDEndpoint is Steam's OP endpoint. It is used for BOTH the
// initial redirect and the verification POST. The verification step must
// always POST here, hardcoded -- never to whatever openid.op_endpoint value
// the callback query string claims -- otherwise a forged assertion from an
// attacker-controlled "OP endpoint" could be fed back and self-verify.
const steamOpenIDEndpoint = "https://steamcommunity.com/openid/login"

var claimedIDPattern = regexp.MustCompile(`^https://steamcommunity\.com/openid/id/(\d+)$`)

// BeginLoginURL builds the URL to redirect the browser to in order to start
// a Steam login. returnTo must be this site's own callback URL
// (e.g. SiteBaseURL + "/auth/steam/callback"); realm is this site's origin.
func BeginLoginURL(returnTo, realm string) string {
	v := url.Values{
		"openid.ns":         {"http://specs.openid.net/auth/2.0"},
		"openid.mode":       {"checkid_setup"},
		"openid.return_to":  {returnTo},
		"openid.realm":      {realm},
		"openid.identity":   {"http://specs.openid.net/auth/2.0/identifier_select"},
		"openid.claimed_id": {"http://specs.openid.net/auth/2.0/identifier_select"},
	}
	return steamOpenIDEndpoint + "?" + v.Encode()
}

// VerifyCallback validates the query parameters Steam redirected the
// browser back with, and returns the authenticated user's Steam64 ID (the
// same ID Arma's getPlayerUID returns in-game, and what players.uid stores
// -- see docs/DATA_CONTRACT.md and docs/WEBSITE.md §3). Returns an error if
// the assertion doesn't verify -- never returns a SteamID without Steam
// itself having confirmed it.
func VerifyCallback(ctx context.Context, query url.Values) (steamID64 string, err error) {
	if query.Get("openid.mode") != "id_res" {
		return "", fmt.Errorf("auth: unexpected openid.mode %q", query.Get("openid.mode"))
	}

	claimedID := query.Get("openid.claimed_id")
	m := claimedIDPattern.FindStringSubmatch(claimedID)
	if m == nil {
		return "", fmt.Errorf("auth: unrecognized claimed_id shape %q", claimedID)
	}
	steamID64 = m[1]

	// Re-POST every field Steam sent, with mode swapped to
	// check_authentication, to Steam's real endpoint -- this is what
	// actually proves the assertion is genuine and wasn't tampered with in
	// transit (query string params are trivially forgeable on their own).
	verify := url.Values{}
	for k, vals := range query {
		if len(vals) > 0 {
			verify.Set(k, vals[0])
		}
	}
	verify.Set("openid.mode", "check_authentication")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, steamOpenIDEndpoint, strings.NewReader(verify.Encode()))
	if err != nil {
		return "", fmt.Errorf("auth: build verification request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("auth: verification request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("auth: reading verification response: %w", err)
	}

	// Steam's direct response is newline-separated key:value pairs, e.g.
	// "ns:http://specs.openid.net/auth/2.0\nis_valid:true\n".
	valid := false
	for _, line := range strings.Split(string(body), "\n") {
		if strings.TrimSpace(line) == "is_valid:true" {
			valid = true
			break
		}
	}
	if !valid {
		return "", fmt.Errorf("auth: steam rejected the assertion")
	}

	return steamID64, nil
}
