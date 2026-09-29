// Package render loads and executes html/template pages. html/template,
// not text/template -- it auto-escapes by context (HTML body, attribute,
// URL, ...), which matters here because ticket bodies and player-chosen
// names are real user input reaching these pages, per docs/WEBSITE.md §10.
package render

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

type Renderer struct {
	dir       string
	templates map[string]*template.Template
}

// New parses every "<dir>/*.html" page against the shared layout.html plus
// every "partial_*.html" fragment, once, at startup -- not per-request. A
// template error is a startup-time failure, not something a player can
// trigger by hitting a route.
//
// Partials are separate from pages because a page defines a "content"
// block (what layout.html renders into) -- parsing every page together in
// one set would mean N different files all defining "content", silently
// overriding each other. A partial defines its own uniquely-named block
// (e.g. "support_sidebar") instead, so it can be parsed alongside every
// page without that collision, and any page can reference it with
// {{template "name" .}}.
// funcs are the small helpers templates may use -- kept deliberately few.
var funcs = template.FuncMap{
	"inc":       func(i int) int { return i + 1 },
	"hasPrefix": strings.HasPrefix,
	// money formats whole in-game dollars: 1234567 -> "1,234,567".
	"money": func(n int64) string {
		neg := n < 0
		if neg {
			n = -n
		}
		d := strconv.FormatInt(n, 10)
		var b strings.Builder
		if neg {
			b.WriteByte('-')
		}
		for i, c := range d {
			if i > 0 && (len(d)-i)%3 == 0 {
				b.WriteByte(',')
			}
			b.WriteRune(c)
		}
		return b.String()
	},
	"dec":  func(i int) int { return i - 1 },
	"list": func(s ...string) []string { return s },
	// qset returns "?query" with the given key/value pairs set on a copy of
	// v (an empty value removes the key) -- for filter and paging links
	// that keep the other filters.
	"qset": func(v url.Values, kv ...string) string {
		out := url.Values{}
		for k, vals := range v {
			out[k] = append([]string(nil), vals...)
		}
		for i := 0; i+1 < len(kv); i += 2 {
			if kv[i+1] == "" {
				out.Del(kv[i])
			} else {
				out.Set(kv[i], kv[i+1])
			}
		}
		if len(out) == 0 {
			return "?"
		}
		return "?" + out.Encode()
	},
	"i64": func(i int64) string { return strconv.FormatInt(i, 10) },
	"deref": func(p *int) int {
		if p == nil {
			return 0
		}
		return *p
	},
	// initialsOf: two-letter avatar text; unnamed "Player #35" shows "#35".
	"initialsOf": func(name string) string {
		if rest, ok := strings.CutPrefix(name, "Player #"); ok {
			return "#" + rest
		}
		r := []rune(strings.TrimSpace(name))
		if len(r) > 2 {
			r = r[:2]
		}
		return strings.ToUpper(string(r))
	},
}

func New(dir string) (*Renderer, error) {
	layout := filepath.Join(dir, "layout.html")

	partials, err := filepath.Glob(filepath.Join(dir, "partial_*.html"))
	if err != nil {
		return nil, err
	}

	pages, err := filepath.Glob(filepath.Join(dir, "*.html"))
	if err != nil {
		return nil, err
	}

	r := &Renderer{dir: dir, templates: map[string]*template.Template{}}
	for _, page := range pages {
		name := filepath.Base(page)
		if name == "layout.html" || strings.HasPrefix(name, "partial_") {
			continue
		}
		files := append([]string{layout, page}, partials...)
		t, err := template.New(filepath.Base(layout)).Funcs(funcs).ParseFiles(files...)
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
	r.RenderStatus(w, http.StatusOK, page, data)
}

// RenderStatus renders page with an HTTP status other than 200 (e.g. 403
// for the access-denied pages). The page is rendered to a buffer first, so
// a template error still produces a clean 500.
func (r *Renderer) RenderStatus(w http.ResponseWriter, status int, page string, data any) {
	t, ok := r.templates[page]
	if !ok {
		http.Error(w, fmt.Sprintf("render: unknown page %q", page), http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		http.Error(w, "render: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}
