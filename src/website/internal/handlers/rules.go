package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"website/internal/auth"
	"website/internal/rules"
)

type rulesData struct {
	Base
	Doc       rules.Doc
	Published bool
	TOC       []rules.Section
	Sections  []rules.Section
	Q         string
	FirstHit  string // anchor of the first changed rule's section, for the banner link
	CanEdit   bool
}

// Rules is the public /rules page (layout plan "Public — Server rules"):
// a table of contents and search on the left, the rulebook on the right,
// with recently changed rules highlighted under a "CHANGED" banner.
func (d *Deps) Rules(w http.ResponseWriter, r *http.Request) {
	data := rulesData{Base: baseFrom(r, "Server rules"), Q: strings.TrimSpace(r.URL.Query().Get("q"))}
	doc, ok, err := rules.Current(r.Context(), d.Pool)
	if err != nil {
		slog.Error("rules: loading failed", "error", err)
		http.Error(w, "Failed to load the rules.", http.StatusInternalServerError)
		return
	}
	data.Doc, data.Published = doc, ok
	data.TOC = doc.Sections
	data.Sections = rules.Filter(doc.Sections, data.Q)
	if doc.Recent {
		for _, s := range doc.Sections {
			for _, rl := range s.Rules {
				if rl.Changed && data.FirstHit == "" {
					data.FirstHit = s.Anchor
				}
			}
		}
	}
	if data.Session != nil {
		data.CanEdit = d.can(r, "rules.edit")
	}
	d.Render.Render(w, "rules.html", data)
}

type rulesEditData struct {
	Base
	AdminShell
	Body    string
	Note    string
	History []rules.Version
	HasLive bool
}

// RulesEdit is /admin/rules: the rulebook as one text document, plus the
// version history. Needs rules.edit.
func (d *Deps) RulesEdit(w http.ResponseWriter, r *http.Request) {
	data := rulesEditData{Base: baseFrom(r, "Server Rules"), AdminShell: d.adminShell(r, "rules")}
	doc, ok, err := rules.Current(r.Context(), d.Pool)
	if err != nil {
		slog.Error("rules: loading failed", "error", err)
		http.Error(w, "Failed to load the rules.", http.StatusInternalServerError)
		return
	}
	data.Body, data.HasLive = doc.Body, ok
	if data.History, err = rules.History(r.Context(), d.Pool, 15); err != nil {
		slog.Error("rules: history failed", "error", err)
	}
	d.Render.Render(w, "rules_edit.html", data)
}

func (d *Deps) RulesSave(w http.ResponseWriter, r *http.Request) {
	const back = "/admin/rules"
	body, note := r.FormValue("body"), r.FormValue("note")
	sess, _ := auth.FromContext(r.Context())
	changed, err := rules.Save(r.Context(), d.Pool, sess.PlayerID, body, note)
	switch {
	case err == nil:
		msg := "Rules published."
		if len(changed) > 0 {
			msg = fmt.Sprintf("Rules published. Highlighted as changed: %s.", strings.Join(changed, ", "))
		}
		http.Redirect(w, r, back+"?notice="+errMsg(msg), http.StatusSeeOther)
	case errors.Is(err, rules.ErrUnchanged):
		http.Redirect(w, r, back+"?notice="+errMsg("Nothing changed, so nothing was published."), http.StatusSeeOther)
	case errors.Is(err, rules.ErrInvalid):
		// Keep the unsaved text: re-render the editor instead of redirecting.
		data := rulesEditData{Base: baseFrom(r, "Server Rules"), AdminShell: d.adminShell(r, "rules"), Body: body, Note: note, HasLive: true}
		msg := strings.TrimPrefix(err.Error(), rules.ErrInvalid.Error()+": ")
		data.Error = "Not published: " + msg + "."
		data.History, _ = rules.History(r.Context(), d.Pool, 15)
		d.Render.RenderStatus(w, http.StatusUnprocessableEntity, "rules_edit.html", data)
	default:
		slog.Error("rules: save failed", "error", err)
		http.Redirect(w, r, back+"?error="+errMsg("Something went wrong. Nothing was published."), http.StatusSeeOther)
	}
}
