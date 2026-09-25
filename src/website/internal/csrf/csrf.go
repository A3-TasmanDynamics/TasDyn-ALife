// Package csrf implements the classic double-submit-cookie pattern: a
// random token in an HttpOnly cookie, the same value echoed into every
// rendered form as a hidden field, and every state-changing request
// required to submit a matching value. Neither side needs to be
// individually secret from an attacker -- what makes forgery hard is that
// a cross-site request can't read the cookie to put its value in the form
// field, only same-origin JS/page renders can. See docs/WEBSITE.md §10.
package csrf

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
)

const CookieName = "csrf_token"

type ctxKey struct{}

func randomToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// ensure returns the request's current CSRF token, setting a fresh cookie
// (and returning the new value) if none exists yet. Called once per
// request by Middleware -- never assume a caller can just read the request
// cookie directly, since on a token's very first issuance the value only
// exists in the outgoing Set-Cookie, not yet in any request.
func ensure(w http.ResponseWriter, r *http.Request, secure bool) string {
	if c, err := r.Cookie(CookieName); err == nil && c.Value != "" {
		return c.Value
	}
	tok, err := randomToken()
	if err != nil {
		return ""
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    tok,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	return tok
}

// Middleware resolves the token (issuing one if needed) and stashes it in
// the request context for handlers/templates to read via FromContext, then
// -- for anything other than GET/HEAD/OPTIONS -- requires the request's
// parsed form to carry a "csrf_token" field matching it exactly.
func Middleware(secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := ensure(w, r, secure)
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, token))

			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			if err := r.ParseForm(); err != nil {
				http.Error(w, "400 Bad Request", http.StatusBadRequest)
				return
			}
			submitted := r.PostForm.Get("csrf_token")
			if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(submitted)) != 1 {
				http.Error(w, "403 Forbidden: CSRF token missing or invalid", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// FromContext returns the current request's CSRF token, for embedding into
// a rendered form as <input type="hidden" name="csrf_token" value="...">.
func FromContext(ctx context.Context) string {
	tok, _ := ctx.Value(ctxKey{}).(string)
	return tok
}
