package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"website/internal/auth"
	"website/internal/quicksearch"
)

// QuickSearch is the Ctrl+K search box's JSON (/admin/search?q=): pages,
// players, vehicles, gangs, cases and flags, each only if the viewer may
// see them.
func (d *Deps) QuickSearch(w http.ResponseWriter, r *http.Request) {
	sess, ok := auth.FromContext(r.Context())
	if !ok || sess == nil {
		http.Error(w, "Sign in first.", http.StatusUnauthorized)
		return
	}
	perms, err := auth.Effective(r.Context(), d.Pool, sess.PlayerID)
	if err != nil {
		slog.Error("quick search: permissions failed", "error", err)
		http.Error(w, "Search failed.", http.StatusInternalServerError)
		return
	}
	q := r.URL.Query().Get("q")
	if len(q) > 100 {
		q = q[:100]
	}
	groups, err := quicksearch.Search(r.Context(), d.Pool, q, perms)
	if err != nil {
		slog.Error("quick search failed", "error", err)
		http.Error(w, "Search failed.", http.StatusInternalServerError)
		return
	}
	if groups == nil {
		groups = []quicksearch.Group{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"q": q, "groups": groups})
}
