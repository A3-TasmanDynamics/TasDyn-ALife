package handlers

import (
	"context"
	"encoding/xml"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/config"
)

func TestRobots(t *testing.T) {
	d := &Deps{Cfg: config.Config{SiteBaseURL: "https://alife.example/"}}
	w := httptest.NewRecorder()
	d.Robots(w, httptest.NewRequest("GET", "/robots.txt", nil))
	body := w.Body.String()
	for _, want := range []string{"User-agent: *", "Disallow: /admin", "Disallow: /dashboard", "Disallow: /tickets", "Sitemap: https://alife.example/sitemap.xml"} {
		if !strings.Contains(body, want) {
			t.Errorf("robots.txt missing %q:\n%s", want, body)
		}
	}
	for _, public := range publicPages {
		for _, p := range privatePrefixes {
			if strings.HasPrefix(public, p) {
				t.Errorf("public page %s is disallowed by %s", public, p)
			}
		}
	}
}

func TestMetaDescription(t *testing.T) {
	if (Base{}).MetaDescription() != defaultDescription {
		t.Error("an empty Description should fall back to the site-wide one")
	}
	if got := (Base{Description: "Own words"}).MetaDescription(); got != "Own words" {
		t.Errorf("own description: %q", got)
	}
}

func TestSitemap(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	pool.Exec(ctx, `INSERT INTO devlog_posts (slug, title, body, published_at) VALUES ('sitemap-live', 'Live', 'x', now()), ('sitemap-draft', 'Draft', 'x', NULL)`)
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM devlog_posts WHERE slug IN ('sitemap-live', 'sitemap-draft')`) })

	d := &Deps{Pool: pool, Cfg: config.Config{SiteBaseURL: "https://alife.example"}}
	w := httptest.NewRecorder()
	d.Sitemap(w, httptest.NewRequest("GET", "/sitemap.xml", nil))
	var set struct {
		URLs []struct {
			Loc     string `xml:"loc"`
			LastMod string `xml:"lastmod"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &set); err != nil {
		t.Fatalf("not valid XML: %v\n%s", err, w.Body.String())
	}
	locs := map[string]string{}
	for _, u := range set.URLs {
		locs[u.Loc] = u.LastMod
	}
	for _, p := range []string{"/", "/leaderboards", "/roster/ems"} {
		if _, ok := locs["https://alife.example"+p]; !ok {
			t.Errorf("sitemap is missing %s", p)
		}
	}
	if m, ok := locs["https://alife.example/devlog/sitemap-live"]; !ok || len(m) != 10 {
		t.Errorf("published post missing or undated: %q %v", m, ok)
	}
	if _, ok := locs["https://alife.example/devlog/sitemap-draft"]; ok {
		t.Error("an unpublished draft is in the sitemap")
	}
}
