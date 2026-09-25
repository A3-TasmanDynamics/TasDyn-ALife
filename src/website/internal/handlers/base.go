// Package handlers holds this app's HTTP route handlers, grouped by
// audience (public/member/admin/support) per docs/WEBSITE.md §1 -- access
// control is middleware (internal/auth), not a check duplicated inside
// each handler.
package handlers

import (
	"net/http"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/auth"
	"website/internal/config"
	"website/internal/render"
)

// errMsg URL-encodes a message for use in a redirect's ?error=/?notice=
// query param.
func errMsg(s string) string { return url.QueryEscape(s) }

// Deps is the shared dependency set every handler group needs -- passed in
// once at wiring time (main.go) rather than each handler reaching for
// package-level globals, which would make this untestable in isolation.
type Deps struct {
	Pool   *pgxpool.Pool
	Render *render.Renderer
	Auth   *auth.Authenticator
	Cfg    config.Config
}

// Base is the common template data every page needs -- embedded into each
// page's own data struct so layout.html's {{.Session}}/{{.Error}}/etc.
// always resolve regardless of which page is rendering.
type Base struct {
	Title   string
	Session *auth.Session
	Error   string
	Notice  string
}

func baseFrom(r *http.Request, title string) Base {
	sess, _ := auth.FromContext(r.Context())
	b := Base{Title: title, Session: sess}
	q := r.URL.Query()
	if q.Get("login_required") == "1" {
		b.Error = "Please sign in to continue."
	}
	if e := q.Get("error"); e != "" {
		b.Error = e
	}
	if n := q.Get("notice"); n != "" {
		b.Notice = n
	}
	return b
}
