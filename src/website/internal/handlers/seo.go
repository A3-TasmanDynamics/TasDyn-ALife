package handlers

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// privatePrefixes are the signed-in and staff areas: never worth indexing,
// and robots.txt asks search engines to stay out of them.
var privatePrefixes = []string{"/admin", "/auth/", "/command", "/dashboard", "/factions", "/notifications", "/profile", "/staff/", "/support", "/tickets"}

// publicPages are the indexable pages besides devlog posts.
var publicPages = []string{"/", "/leaderboards", "/rules", "/status", "/devlog", "/roster/police", "/roster/ems"}

// Robots serves /robots.txt: public pages open, private areas closed, and
// where the sitemap is.
func (d *Deps) Robots(w http.ResponseWriter, r *http.Request) {
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	for _, p := range privatePrefixes {
		fmt.Fprintf(&b, "Disallow: %s\n", p)
	}
	fmt.Fprintf(&b, "\nSitemap: %s/sitemap.xml\n", strings.TrimRight(d.Cfg.SiteBaseURL, "/"))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(b.String()))
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type sitemapSet struct {
	XMLName xml.Name     `xml:"urlset"`
	NS      string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

// Sitemap serves /sitemap.xml: the public pages and every published devlog
// post (dated by its last edit).
func (d *Deps) Sitemap(w http.ResponseWriter, r *http.Request) {
	base := strings.TrimRight(d.Cfg.SiteBaseURL, "/")
	set := sitemapSet{NS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	for _, p := range publicPages {
		set.URLs = append(set.URLs, sitemapURL{Loc: base + p})
	}
	rows, err := d.Pool.Query(r.Context(), `
		SELECT slug, GREATEST(published_at, COALESCE(updated_at, published_at)) FROM devlog_posts
		WHERE published_at IS NOT NULL ORDER BY published_at DESC`)
	if err != nil {
		// The pages alone are still a useful sitemap.
		slog.Error("sitemap: devlog query failed", "error", err)
	} else {
		for rows.Next() {
			var slug string
			var when time.Time
			if rows.Scan(&slug, &when) == nil {
				set.URLs = append(set.URLs, sitemapURL{Loc: base + "/devlog/" + slug, LastMod: when.UTC().Format("2006-01-02")})
			}
		}
		rows.Close()
	}
	out, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		http.Error(w, "sitemap failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(out)
}
