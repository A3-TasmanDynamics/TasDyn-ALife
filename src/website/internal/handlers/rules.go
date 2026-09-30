package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"website/internal/auth"
	"website/internal/notify"
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
	Body     string
	Sections string // the rulebook as JSON, for the section editor
	TextMode bool   // open on the text editor (unsaved text that didn't parse)
	Note     string
	History  []rules.Version
	HasLive  bool
}

// editSection is the section editor's JSON: rules are numbered on save.
type editSection struct {
	Title string     `json:"title"`
	Intro string     `json:"intro"`
	Rules []editRule `json:"rules"`
}

type editRule struct {
	Text   string      `json:"text"`
	Points []editPoint `json:"points"`
}

type editPoint struct {
	Text string   `json:"text"`
	Sub  []string `json:"sub"`
}

func sectionsJSON(secs []rules.Section) string {
	out := []editSection{}
	for _, s := range secs {
		es := editSection{Title: s.Title, Intro: s.Intro, Rules: []editRule{}}
		for _, r := range s.Rules {
			er := editRule{Text: r.Text, Points: []editPoint{}}
			for _, p := range r.Points {
				er.Points = append(er.Points, editPoint{Text: p.Text, Sub: append([]string{}, p.Sub...)})
			}
			es.Rules = append(es.Rules, er)
		}
		out = append(out, es)
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// bodyFromSections turns the section editor's JSON into the rules
// document. Blank dot points are dropped; a blank title or rule is an
// error, so nothing is silently lost.
func bodyFromSections(raw string) (string, error) {
	var in []editSection
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		return "", fmt.Errorf("%w: the editor sent something unreadable; reload and try again", rules.ErrInvalid)
	}
	var secs []rules.Section
	for i, es := range in {
		s := rules.Section{Title: strings.TrimSpace(es.Title), Intro: es.Intro}
		if s.Title == "" {
			return "", fmt.Errorf("%w: section %d needs a title", rules.ErrInvalid, i+1)
		}
		for j, er := range es.Rules {
			r := rules.Rule{Text: strings.TrimSpace(er.Text)}
			if r.Text == "" {
				return "", fmt.Errorf("%w: rule %d.%d is empty; write it or delete it", rules.ErrInvalid, i+1, j+1)
			}
			for _, ep := range er.Points {
				pt := rules.Point{Text: strings.TrimSpace(ep.Text)}
				for _, sp := range ep.Sub {
					if sp = strings.TrimSpace(sp); sp != "" {
						pt.Sub = append(pt.Sub, sp)
					}
				}
				if pt.Text == "" && len(pt.Sub) > 0 {
					return "", fmt.Errorf("%w: rule %d.%d has sub-points under an empty dot point", rules.ErrInvalid, i+1, j+1)
				}
				if pt.Text != "" {
					r.Points = append(r.Points, pt)
				}
			}
			s.Rules = append(s.Rules, r)
		}
		secs = append(secs, s)
	}
	return rules.Format(secs), nil
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
	data.Sections = sectionsJSON(doc.Sections)
	if data.History, err = rules.History(r.Context(), d.Pool, 15); err != nil {
		slog.Error("rules: history failed", "error", err)
	}
	d.Render.Render(w, "rules_edit.html", data)
}

func (d *Deps) RulesSave(w http.ResponseWriter, r *http.Request) {
	const back = "/admin/rules"
	body, note := r.FormValue("body"), r.FormValue("note")
	sess, _ := auth.FromContext(r.Context())
	var err error
	fromSections := r.FormValue("mode") == "sections"
	if fromSections {
		body, err = bodyFromSections(r.FormValue("sections"))
	}
	var changed []string
	if err == nil {
		changed, err = rules.Save(r.Context(), d.Pool, sess.PlayerID, body, note)
	}
	switch {
	case err == nil:
		msg := "Rules published."
		if len(changed) > 0 {
			msg = fmt.Sprintf("Rules published. Highlighted as changed: %s.", strings.Join(changed, ", "))
		}
		// Optionally make every staff member acknowledge it (GAMEPANEL_PARITY §7.3).
		if r.FormValue("essential") == "on" {
			title := "Server rules updated"
			if len(changed) > 0 {
				title = "Rules updated: " + strings.Join(changed, ", ")
			}
			if tx, terr := d.Pool.Begin(r.Context()); terr == nil {
				if _, n, berr := notify.Broadcast(r.Context(), tx, sess.PlayerID, title, strings.TrimSpace(note), "/rules", "staff"); berr == nil && tx.Commit(r.Context()) == nil {
					msg += fmt.Sprintf(" Essential notice sent to %d staff.", n)
				} else {
					tx.Rollback(r.Context())
					slog.Error("rules: essential notice failed", "error", berr)
					msg += " The essential notice couldn't be sent; post it from Essential Notices."
				}
			}
		}
		http.Redirect(w, r, back+"?notice="+errMsg(msg), http.StatusSeeOther)
	case errors.Is(err, rules.ErrUnchanged):
		http.Redirect(w, r, back+"?notice="+errMsg("Nothing changed, so nothing was published."), http.StatusSeeOther)
	case errors.Is(err, rules.ErrInvalid):
		// Keep the unsaved text: re-render the editor instead of redirecting.
		data := rulesEditData{Base: baseFrom(r, "Server Rules"), AdminShell: d.adminShell(r, "rules"), Body: body, Note: note, HasLive: true}
		if fromSections {
			data.Sections = r.FormValue("sections") // keep their unsaved sections
		} else if secs, perr := rules.Parse(body); perr == nil {
			data.Sections = sectionsJSON(secs)
		} else {
			data.Sections, data.TextMode = "[]", true
		}
		msg := strings.TrimPrefix(err.Error(), rules.ErrInvalid.Error()+": ")
		data.Error = "Not published: " + msg + "."
		data.History, _ = rules.History(r.Context(), d.Pool, 15)
		d.Render.RenderStatus(w, http.StatusUnprocessableEntity, "rules_edit.html", data)
	default:
		slog.Error("rules: save failed", "error", err)
		http.Redirect(w, r, back+"?error="+errMsg("Something went wrong. Nothing was published."), http.StatusSeeOther)
	}
}
