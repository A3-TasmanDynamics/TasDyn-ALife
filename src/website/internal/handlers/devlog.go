package handlers

import (
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"website/internal/auth"
)

type devlogSummary struct {
	Slug        string
	Title       string
	Snippet     string
	PublishedAt string
}

type devlogListData struct {
	Base
	Posts []devlogSummary
}

// snippetOf returns the first ~180 characters of body, cut at a word
// boundary, for a list-page teaser -- never the raw body (which could be
// much longer than a card should show).
func snippetOf(body string) string {
	body = strings.TrimSpace(body)
	if len(body) <= 180 {
		return body
	}
	cut := body[:180]
	if i := strings.LastIndexByte(cut, ' '); i > 0 {
		cut = cut[:i]
	}
	return cut + "…"
}

// DevlogList shows every published post, newest first -- public, no login
// required, same as the landing page.
func (d *Deps) DevlogList(w http.ResponseWriter, r *http.Request) {
	data := devlogListData{Base: baseFrom(r, "Devlog")}

	rows, err := d.Pool.Query(r.Context(), `
		SELECT slug, title, body, published_at
		FROM devlog_posts
		WHERE published_at IS NOT NULL
		ORDER BY published_at DESC
	`)
	if err != nil {
		slog.Error("devlog list: query failed", "error", err)
		http.Error(w, "Failed to load the devlog.", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var s devlogSummary
		var body string
		var publishedAt time.Time
		if err := rows.Scan(&s.Slug, &s.Title, &body, &publishedAt); err == nil {
			s.Snippet = snippetOf(body)
			s.PublishedAt = publishedAt.Format("2 January 2006")
			data.Posts = append(data.Posts, s)
		}
	}

	d.Render.Render(w, "devlog_list.html", data)
}

type devlogPostData struct {
	Base
	Body        string
	AuthorName  string
	PublishedAt string
}

// DevlogPost shows one published post by slug -- public. A draft (published_at
// IS NULL) 404s here exactly like a nonexistent slug would, rather than
// leaking that a draft with that slug exists.
func (d *Deps) DevlogPost(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var p devlogPostData
	var title string
	var publishedAt time.Time
	err := d.Pool.QueryRow(r.Context(), `
		SELECT dp.title, dp.body, COALESCE(NULLIF(pl.name, ''), 'Staff'), dp.published_at
		FROM devlog_posts dp
		LEFT JOIN players pl ON pl.id = dp.author_player_id
		WHERE dp.slug = $1 AND dp.published_at IS NOT NULL
	`, slug).Scan(&title, &p.Body, &p.AuthorName, &publishedAt)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("devlog post: query failed", "error", err)
		http.Error(w, "Failed to load this post.", http.StatusInternalServerError)
		return
	}
	p.PublishedAt = publishedAt.Format("2 January 2006")
	p.Base = baseFrom(r, title)

	d.Render.Render(w, "devlog_post.html", p)
}

type devlogNewData struct {
	Base
	AdminShell
}

// DevlogNewForm renders the staff post-composer -- gated on Admin Panel
// access (Deps.Auth.RequireAdminPanel, wired in main.go) same as the rest of
// /admin/*. A dedicated devlog.write permission would be more precise, but
// admin-panel-or-nothing is a reasonable v1 given every current admin has
// full trust anyway -- see docs/WEBSITE.md for the broader permission model
// this can graduate into later.
func (d *Deps) DevlogNewForm(w http.ResponseWriter, r *http.Request) {
	d.Render.Render(w, "devlog_new.html", devlogNewData{Base: baseFrom(r, "New Devlog Post"), AdminShell: d.adminShell(r, "devlog")})
}

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(title string) string {
	s := slugNonAlnum.ReplaceAllString(strings.ToLower(title), "-")
	return strings.Trim(s, "-")
}

// DevlogCreate inserts a new post and publishes it immediately -- there's
// no draft/edit UI yet, so a draft would be a dead end with no way to ever
// publish or change it later; not building that half is deliberate, not an
// oversight (see the schema comment on devlog_posts.published_at). body is
// stored as plain text -- see that same comment for why there's no
// markdown/HTML pipeline. A slug collision (two posts titled the same
// thing) gets a numeric suffix rather than a form-rejecting error, since
// staff shouldn't have to think about slugs at all in the common case.
func (d *Deps) DevlogCreate(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/devlog/new?error="+errMsg("Invalid form submission."), http.StatusSeeOther)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	body := strings.TrimSpace(r.FormValue("body"))
	if title == "" || body == "" {
		http.Redirect(w, r, "/admin/devlog/new?error="+errMsg("Title and body are required."), http.StatusSeeOther)
		return
	}

	base := slugify(title)
	if base == "" {
		base = "post"
	}
	slug := base
	for i := 2; ; i++ {
		var exists bool
		if err := d.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM devlog_posts WHERE slug = $1)`, slug).Scan(&exists); err != nil {
			slog.Error("devlog create: slug check failed", "error", err)
			http.Redirect(w, r, "/admin/devlog/new?error="+errMsg("Something went wrong."), http.StatusSeeOther)
			return
		}
		if !exists {
			break
		}
		slug = base + "-" + strconv.Itoa(i)
	}

	if _, err := d.Pool.Exec(r.Context(), `
		INSERT INTO devlog_posts (slug, title, body, author_player_id, published_at)
		VALUES ($1, $2, $3, $4, now())
	`, slug, title, body, sess.PlayerID); err != nil {
		slog.Error("devlog create: insert failed", "error", err)
		http.Redirect(w, r, "/admin/devlog/new?error="+errMsg("Something went wrong."), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/devlog/"+slug, http.StatusSeeOther)
}
