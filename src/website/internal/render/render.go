// Package render loads and executes html/template pages. html/template,
// not text/template -- it auto-escapes by context (HTML body, attribute,
// URL, ...), which matters here because ticket bodies and player-chosen
// names are real user input reaching these pages, per docs/WEBSITE.md §10.
package render

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
)

type Renderer struct {
	dir       string
	templates map[string]*template.Template
}

// New parses every "<dir>/*.html" page against the shared layout.html,
// once, at startup -- not per-request. A template error is a startup-time
// failure, not something a player can trigger by hitting a route.
func New(dir string) (*Renderer, error) {
	layout := filepath.Join(dir, "layout.html")

	pages, err := filepath.Glob(filepath.Join(dir, "*.html"))
	if err != nil {
		return nil, err
	}

	r := &Renderer{dir: dir, templates: map[string]*template.Template{}}
	for _, page := range pages {
		name := filepath.Base(page)
		if name == "layout.html" {
			continue
		}
		t, err := template.ParseFiles(layout, page)
		if err != nil {
			return nil, fmt.Errorf("render: parsing %s: %w", name, err)
		}
		r.templates[name] = t
	}
	return r, nil
}

// Render executes "<page>.html" (must have been found by New) against the
// shared layout, with data as the layout's top-level template data.
func (r *Renderer) Render(w http.ResponseWriter, page string, data any) {
	t, ok := r.templates[page]
	if !ok {
		http.Error(w, fmt.Sprintf("render: unknown page %q", page), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "render: "+err.Error(), http.StatusInternalServerError)
	}
}
