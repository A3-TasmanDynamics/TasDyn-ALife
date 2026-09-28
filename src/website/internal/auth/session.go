package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	CookieName   = "alife_session"
	sessionTTL   = 30 * 24 * time.Hour
	cookieMaxAge = int(sessionTTL / time.Second)
)

// Session is what's resolved from the cookie and attached to the request
// context -- panel access is decided once here, not re-derived from scratch
// on every handler. See docs/WEBSITE.md §4.
type Session struct {
	PlayerID           int64
	Name               string
	AdminPanelAccess   bool
	SupportPanelAccess bool
}

type ctxKey struct{}

// Authenticator holds what the login/session machinery needs: a DB pool and
// the cookie security settings (Secure must be true in production --
// config.Config.CookieSecure, plumbed in from main.go).
type Authenticator struct {
	Pool         *pgxpool.Pool
	CookieSecure bool
}

// generateToken returns a cryptographically random, URL-safe token, and its
// SHA-256 hash (hex) -- the hash is what's stored in web_sessions, the raw
// token is what goes in the cookie. Storing only the hash means a DB read
// (backup, leaked query log, compromised replica) can't be turned directly
// into a working session, same reasoning as never storing a password in
// plaintext.
func generateToken() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	return raw, hash, nil
}

// resolvePanelAccess computes admin/support panel access for a player at
// login time, per docs/WEBSITE.md §4: the rank's default, unless a
// staff_permission_overrides row for 'panel.admin'/'panel.support' says
// otherwise. A player with no staff_rank_id gets false/false without
// needing a rank row to exist at all.
//
// A suspended/LOA staff member (docs/OPERATIONS.md §3) gets false/false
// here regardless of rank/overrides -- checked last, after overrides, so a
// per-player override can never be used to bypass a lifecycle suspension
// (the override mechanism grants/revokes specific panels, it was never
// meant to double as a "staff member is currently inactive" escape hatch).
// This is resolved once at login, same as the rank/override checks above
// it -- see CreateSession's own doc comment for why a mid-session status
// change doesn't take effect until next login without also revoking the
// session row directly.
func resolvePanelAccess(ctx context.Context, pool *pgxpool.Pool, playerID int64) (adminAccess, supportAccess bool, err error) {
	var staffStatus string
	err = pool.QueryRow(ctx, `
		SELECT COALESCE(sr.default_admin_panel, false), COALESCE(sr.default_support_panel, false), p.staff_status
		FROM players p
		LEFT JOIN staff_ranks sr ON sr.id = p.staff_rank_id
		WHERE p.id = $1
	`, playerID).Scan(&adminAccess, &supportAccess, &staffStatus)
	if err != nil {
		return false, false, err
	}

	rows, err := pool.Query(ctx, `
		SELECT command_key, allow FROM staff_permission_overrides
		WHERE player_id = $1 AND command_key IN ('panel.admin', 'panel.support')
	`, playerID)
	if err != nil {
		return false, false, err
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var allow bool
		if err := rows.Scan(&key, &allow); err != nil {
			return false, false, err
		}
		switch key {
		case "panel.admin":
			adminAccess = allow
		case "panel.support":
			supportAccess = allow
		}
	}
	if err := rows.Err(); err != nil {
		return false, false, err
	}

	if staffStatus != "active" {
		return false, false, nil
	}
	return adminAccess, supportAccess, nil
}

// CreateSession resolves panel access, inserts a web_sessions row, and sets
// the session cookie on the response. Called after a Steam login verifies
// successfully.
func (a *Authenticator) CreateSession(ctx context.Context, w http.ResponseWriter, r *http.Request, playerID int64) error {
	adminAccess, supportAccess, err := resolvePanelAccess(ctx, a.Pool, playerID)
	if err != nil {
		return err
	}

	raw, hash, err := generateToken()
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(sessionTTL)
	_, err = a.Pool.Exec(ctx, `
		INSERT INTO web_sessions
			(player_id, token_hash, admin_panel_access, support_panel_access, ip_address, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, playerID, hash, adminAccess, supportAccess, clientIP(r), r.UserAgent(), expiresAt)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    raw,
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		Secure:   a.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})

	return nil
}

// Logout deletes the session row (immediate revocation -- see
// docs/WEBSITE.md §6) and clears the cookie.
func (a *Authenticator) Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil {
		sum := sha256.Sum256([]byte(c.Value))
		hash := hex.EncodeToString(sum[:])
		_, _ = a.Pool.Exec(ctx, `DELETE FROM web_sessions WHERE token_hash = $1`, hash)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// Middleware loads the session (if any) from the cookie and attaches it to
// the request context. It never rejects a request itself -- handlers/
// middleware further down the chain (RequireLogin, RequireAdminPanel,
// RequireSupportPanel) decide what's required for a given route. This one
// only resolves "who, if anyone, is this."
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(CookieName)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		sum := sha256.Sum256([]byte(c.Value))
		hash := hex.EncodeToString(sum[:])

		var sess Session
		var expiresAt time.Time
		// COALESCE/NULLIF: players.name defaults to '' and is only ever set
		// by the game's own save path -- a website-first signup (Steam
		// login before ever connecting in-game) has no name yet, and an
		// empty string here reads as a blank, mysteriously-nameless entry
		// everywhere this session's Name gets displayed (top nav, ticket
		// "Requester" columns, ...) rather than an honest placeholder.
		err = a.Pool.QueryRow(r.Context(), `
			SELECT p.id, COALESCE(NULLIF(p.name, ''), 'Player #' || p.id), ws.admin_panel_access, ws.support_panel_access, ws.expires_at
			FROM web_sessions ws
			JOIN players p ON p.id = ws.player_id
			WHERE ws.token_hash = $1
		`, hash).Scan(&sess.PlayerID, &sess.Name, &sess.AdminPanelAccess, &sess.SupportPanelAccess, &expiresAt)

		if err != nil || time.Now().After(expiresAt) {
			// Invalid, unknown, or expired token -- proceed unauthenticated
			// rather than erroring; a stale cookie is not this site's fault.
			next.ServeHTTP(w, r)
			return
		}

		// Best-effort activity touch; a failure here must never break the request.
		_, _ = a.Pool.Exec(r.Context(), `UPDATE web_sessions SET last_seen_at = now() WHERE token_hash = $1`, hash)

		ctx := context.WithValue(r.Context(), ctxKey{}, &sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext returns the current request's session, if any.
func FromContext(ctx context.Context) (*Session, bool) {
	sess, ok := ctx.Value(ctxKey{}).(*Session)
	return sess, ok
}

var ErrNoSession = errors.New("auth: no session")

// RequireLogin redirects to the landing page (which offers Steam login) if
// no session is present.
func RequireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := FromContext(r.Context()); !ok {
			http.Redirect(w, r, "/?login_required=1", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// denialReason looks up *why* a player lacks panel access, only called on
// the deny path below (never on an allowed request) -- distinguishes "your
// rank/overrides never granted this" from "you're currently suspended/on
// LOA" (docs/OPERATIONS.md §3), so a staff member sees why, not a generic
// 403. Best-effort: a lookup failure here just falls back to the generic
// message rather than failing the whole (already-denying) request.
func (a *Authenticator) denialReason(ctx context.Context, playerID int64) string {
	var status string
	var reason *string
	if err := a.Pool.QueryRow(ctx,
		`SELECT staff_status, staff_status_reason FROM players WHERE id = $1`, playerID,
	).Scan(&status, &reason); err != nil {
		return ""
	}
	switch status {
	case "suspended":
		if reason != nil && *reason != "" {
			return "You're currently suspended from staff duties: " + *reason
		}
		return "You're currently suspended from staff duties."
	case "loa":
		if reason != nil && *reason != "" {
			return "You're currently on a leave of absence: " + *reason
		}
		return "You're currently on a leave of absence."
	default:
		return ""
	}
}

// RequireAdminPanel 403s (not just redirects) when the session lacks admin
// access -- this re-checks the session row's cached flag on every request,
// same as RequireSupportPanel. It does NOT re-resolve rank/overrides from
// scratch per request (that's login-time, §4); a revoked override still
// takes effect immediately because revoking access is expected to delete
// or update the session row itself, not wait for this middleware to notice
// -- see docs/WEBSITE.md §10 for the write-path recheck this alone doesn't
// cover.
func (a *Authenticator) RequireAdminPanel(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := FromContext(r.Context())
		if !ok {
			http.Redirect(w, r, "/?login_required=1", http.StatusSeeOther)
			return
		}
		if !sess.AdminPanelAccess {
			msg := "403 Forbidden: Admin Panel access required"
			if reason := a.denialReason(r.Context(), sess.PlayerID); reason != "" {
				msg = reason
			}
			http.Error(w, msg, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *Authenticator) RequireSupportPanel(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := FromContext(r.Context())
		if !ok {
			http.Redirect(w, r, "/?login_required=1", http.StatusSeeOther)
			return
		}
		if !sess.SupportPanelAccess {
			msg := "403 Forbidden: Support Panel access required"
			if reason := a.denialReason(r.Context(), sess.PlayerID); reason != "" {
				msg = reason
			}
			http.Error(w, msg, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	// No reverse-proxy header trust (X-Forwarded-For) configured yet -- add
	// that only once the real deployment topology (which proxy, if any) is
	// known, since trusting it blindly lets a client spoof its own logged IP.
	//
	// net.SplitHostPort, not a hand-rolled "find the last colon" -- that
	// approach breaks on an IPv6 RemoteAddr like "[::1]:59271" (which is
	// exactly what a local browser hitting localhost produces): the last
	// colon sits before the port as intended, but the result still carries
	// the "[...]" brackets net/http wraps IPv6 hosts in, and Postgres's
	// `inet` column rejects bracketed input outright (invalid input syntax
	// for type inet) -- SplitHostPort strips them correctly instead of
	// needing that as a separate step.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
