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

// defaultRulesIntro shows until an introduction is written in the editor.
const defaultRulesIntro = "Welcome to TasDyn-ALife. These rules keep the server fair and fun for everyone, and they apply to every player, including staff. Not knowing a rule isn't an excuse, so please read them before you play. If something isn't covered here, use common sense and ask staff."

type rulesData struct {
	Base
	Intro     []string // the rulebook's introduction, one entry per paragraph
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
	intro := doc.Intro
	if intro == "" {
		intro = defaultRulesIntro
	}
	data.Intro = strings.Split(intro, "\n")
	data.TOC = doc.Sections
	data.Sections = rules.Filter(doc.Sections, data.Q)
	if doc.Recent {
	first:
		for _, s := range doc.Sections {
			for _, rl := range s.Rules {
				if rl.Changed {
					data.FirstHit = "rule-" + rl.Number
					break first
				}
			}
			for _, sub := range s.Subs {
				for _, rl := range sub.Rules {
					if rl.Changed {
						data.FirstHit = "rule-" + rl.Number
						break first
					}
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

// editDoc is the section editor's JSON: rules are numbered on save.
type editDoc struct {
	Intro    string        `json:"intro"`
	Sections []editSection `json:"sections"`
}

type editSection struct {
	Title string        `json:"title"`
	Intro string        `json:"intro"`
	Rules []editRule    `json:"rules"`
	Subs  []editSection `json:"subs,omitempty"` // subsections (one level only)
}

type editRule struct {
	Text   string      `json:"text"`
	Points []editPoint `json:"points"`
}

type editPoint struct {
	Text string   `json:"text"`
	Sub  []string `json:"sub"`
}

func editRules(rs []rules.Rule) []editRule {
	out := []editRule{}
	for _, r := range rs {
		er := editRule{Text: r.Text, Points: []editPoint{}}
		for _, p := range r.Points {
			er.Points = append(er.Points, editPoint{Text: p.Text, Sub: append([]string{}, p.Sub...)})
		}
		out = append(out, er)
	}
	return out
}

func sectionsJSON(intro string, secs []rules.Section) string {
	out := []editSection{}
	for _, s := range secs {
		es := editSection{Title: s.Title, Intro: s.Intro, Rules: editRules(s.Rules), Subs: []editSection{}}
		for _, sub := range s.Subs {
			es.Subs = append(es.Subs, editSection{Title: sub.Title, Intro: sub.Intro, Rules: editRules(sub.Rules)})
		}
		out = append(out, es)
	}
	b, _ := json.Marshal(editDoc{Intro: intro, Sections: out})
	return string(b)
}

// bodyFromSections turns the section editor's JSON into the rules
// document. Blank dot points are dropped; a blank title or rule is an
// error, so nothing is silently lost.
func bodyFromSections(raw string) (string, error) {
	var in editDoc
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		return "", fmt.Errorf("%w: the editor sent something unreadable; reload and try again", rules.ErrInvalid)
	}
	var secs []rules.Section
	for i, es := range in.Sections {
		s := rules.Section{Title: strings.TrimSpace(es.Title), Intro: es.Intro}
		if s.Title == "" {
			return "", fmt.Errorf("%w: section %d needs a heading", rules.ErrInvalid, i+1)
		}
		var err error
		if s.Rules, err = rulesFromEdit(es.Rules, s.Title); err != nil {
			return "", err
		}
		for k, esub := range es.Subs {
			sub := rules.Subsection{Title: strings.TrimSpace(esub.Title), Intro: esub.Intro}
			if sub.Title == "" {
				return "", fmt.Errorf("%w: subsection %d in %q needs a heading", rules.ErrInvalid, k+1, s.Title)
			}
			if sub.Rules, err = rulesFromEdit(esub.Rules, sub.Title); err != nil {
				return "", err
			}
			s.Subs = append(s.Subs, sub)
		}
		secs = append(secs, s)
	}
	return rules.FormatDoc(in.Intro, secs), nil
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
	data.Sections = sectionsJSON(doc.Intro, doc.Sections)
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
		labels := changed
		if doc, ok, _ := rules.Current(r.Context(), d.Pool); ok {
			labels = doc.ChangedLabels()
		}
		if len(labels) > 0 {
			msg = fmt.Sprintf("Rules published. Highlighted as changed: %s.", strings.Join(labels, ", "))
		}
		// Optionally make every staff member acknowledge it (GAMEPANEL_PARITY §7.3).
		if r.FormValue("essential") == "on" {
			title := "Server rules updated"
			if len(labels) > 0 {
				title = "Rules updated: " + strings.Join(labels, ", ")
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
		} else if intro, secs, perr := rules.ParseDoc(body); perr == nil {
			data.Sections = sectionsJSON(intro, secs)
		} else {
			data.Sections, data.TextMode = `{"intro":"","sections":[]}`, true
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

// rulesFromEdit checks and converts one heading's rules from the editor.
// Blank dot points are dropped; a blank rule is an error, so nothing is
// silently lost.
func rulesFromEdit(in []editRule, where string) ([]rules.Rule, error) {
	var out []rules.Rule
	for j, er := range in {
		r := rules.Rule{Text: strings.TrimSpace(er.Text)}
		if r.Text == "" {
			return nil, fmt.Errorf("%w: rule %d under %q is empty; write it or delete it", rules.ErrInvalid, j+1, where)
		}
		for _, ep := range er.Points {
			pt := rules.Point{Text: strings.TrimSpace(ep.Text)}
			for _, sp := range ep.Sub {
				if sp = strings.TrimSpace(sp); sp != "" {
					pt.Sub = append(pt.Sub, sp)
				}
			}
			if pt.Text == "" && len(pt.Sub) > 0 {
				return nil, fmt.Errorf("%w: rule %d under %q has sub-points under an empty dot point", rules.ErrInvalid, j+1, where)
			}
			if pt.Text != "" {
				r.Points = append(r.Points, pt)
			}
		}
		out = append(out, r)
	}
	return out, nil
}
