// Package rules stores the public server rules (/rules, layout plan
// "Public — Server rules") and their edits (/admin/rules).
//
// The rules are stored as one plain-text document. /admin/rules edits it
// section by section; the text form is also editable directly:
//
//	> An optional introduction to the whole rulebook (before any section).
//	# General conduct
//	> An optional intro shown under the section title.
//	1.1 Stay in character while in-game.
//	1.2 Treat other players and staff with respect.
//	    A line that doesn't start with a number continues the rule above.
//	## Chain of command
//	> A subsection (##) groups rules inside a section, with an optional intro.
//	1.3.1 Follow the instructions of senior staff.
//	# Safe zones
//	2.1 These areas are safe zones:
//	- Kavala Markets
//	  - A second-level dot point (indented).
//	- Kavala Hospital
//
// Every save is a new row in rule_versions, so the history is never lost.
// Rules whose text changed since the previous version are highlighted on
// the public page for a while, with the editor's note as a banner.
package rules

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
)

// ChangedFor is how long changed rules stay highlighted.
const ChangedFor = 30 * 24 * time.Hour

// MaxBody caps the document size (the whole rulebook, not one rule).
const MaxBody = 100_000

type Rule struct {
	Number  string // unique number from the document, for anchors and change tracking
	Disp    string // outline number players see: "1.2", or "1.4.1" in a subsection
	Text    string
	Points  []Point
	Changed bool
}

// Point is a dot point under a rule, with optional second-level points.
type Point struct {
	Text string
	Sub  []string
}

type Section struct {
	N      int
	Title  string
	Anchor string
	Intro  string
	Rules  []Rule // rules directly in the section, before any subsection
	Subs   []Subsection
}

// Subsection groups rules inside a section ("1.4 Chain of command").
type Subsection struct {
	Disp   string // "1.4"
	Title  string
	Anchor string
	Intro  string
	Rules  []Rule
}

// rules points at every rule in the section, subsections included.
func (s *Section) rules() []*Rule {
	var out []*Rule
	for i := range s.Rules {
		out = append(out, &s.Rules[i])
	}
	for j := range s.Subs {
		for i := range s.Subs[j].Rules {
			out = append(out, &s.Subs[j].Rules[i])
		}
	}
	return out
}

// Count is how many rules the section holds, subsections included.
func (s Section) Count() int { return len(s.rules()) }

// number fills in display numbers and anchors in outline order: section
// rules first (N.1, N.2...), then subsections continuing the count (N.3),
// whose rules go a level deeper (N.3.1).
func (s *Section) number() {
	for i := range s.Rules {
		s.Rules[i].Disp = fmt.Sprintf("%d.%d", s.N, i+1)
	}
	for j := range s.Subs {
		sub := &s.Subs[j]
		sub.Disp = fmt.Sprintf("%d.%d", s.N, len(s.Rules)+j+1)
		sub.Anchor = "r" + strings.ReplaceAll(sub.Disp, ".", "-")
		for i := range sub.Rules {
			sub.Rules[i].Disp = fmt.Sprintf("%s.%d", sub.Disp, i+1)
		}
	}
}

// content is a rule's text and dot points, for spotting changes whatever
// its number.
func (r Rule) content() string {
	var b strings.Builder
	b.WriteString(r.Text)
	for _, p := range r.Points {
		b.WriteString("\n- " + p.Text)
		for _, sp := range p.Sub {
			b.WriteString("\n  - " + sp)
		}
	}
	return b.String()
}

// Doc is the live rulebook.
type Doc struct {
	Version    int64
	Body       string
	Intro      string // the rulebook's introduction, "" = none written
	Sections   []Section
	UpdatedAt  time.Time
	UpdatedBy  string
	ChangeNote string   // shown as the "CHANGED" banner while recent
	Changed    []string // rule numbers changed in this version
	Recent     bool     // updated within ChangedFor
}

// ErrInvalid wraps a parse/validation error meant for the editor.
var ErrInvalid = errors.New("invalid rules")

var ruleLine = regexp.MustCompile(`^(\d+(?:\.\d+)*)[.)]?\s+(.+)$`)

// pointLine is a dot point: "- text", "* text" or "• text".
var pointLine = regexp.MustCompile(`^[-*\x{2022}]\s+(.+)$`)

// Parse turns the document into sections (see ParseDoc).
func Parse(body string) ([]Section, error) {
	_, secs, err := ParseDoc(body)
	return secs, err
}

// ParseDoc turns the document into its introduction and sections. It
// rejects a rule before the first heading, duplicate rule numbers and an
// empty document.
func ParseDoc(body string) (intro string, out []Section, err error) {
	seen := map[string]bool{}
	inPoint := false // continuation lines extend the last dot point
	// target is where rules go: the current subsection, else the section.
	target := func() *[]Rule {
		s := &out[len(out)-1]
		if n := len(s.Subs); n > 0 {
			return &s.Subs[n-1].Rules
		}
		return &s.Rules
	}
	for i, raw := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, ">"):
			text := strings.TrimSpace(strings.TrimPrefix(line, ">"))
			if len(out) == 0 {
				if intro != "" {
					intro += "\n"
				}
				intro += text
				continue
			}
			s := &out[len(out)-1]
			if len(*target()) > 0 {
				return "", nil, fmt.Errorf("%w: line %d: an intro (>) goes before the first rule under its heading", ErrInvalid, i+1)
			}
			if n := len(s.Subs); n > 0 {
				s.Subs[n-1].Intro = strings.TrimSpace(s.Subs[n-1].Intro + " " + text)
			} else {
				s.Intro = strings.TrimSpace(s.Intro + " " + text)
			}
		case pointLine.MatchString(line) && !ruleLine.MatchString(line):
			if len(out) == 0 || len(*target()) == 0 {
				return "", nil, fmt.Errorf("%w: line %d: a dot point needs a rule above it", ErrInvalid, i+1)
			}
			rs := *target()
			r := &rs[len(rs)-1]
			text := pointLine.FindStringSubmatch(line)[1]
			indented := len(raw)-len(strings.TrimLeft(raw, " \t")) >= 2
			if indented && len(r.Points) > 0 {
				p := &r.Points[len(r.Points)-1]
				p.Sub = append(p.Sub, text)
			} else {
				r.Points = append(r.Points, Point{Text: text})
			}
			inPoint = true
		case strings.HasPrefix(line, "##"):
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if title == "" {
				return "", nil, fmt.Errorf("%w: line %d: a subsection needs a title after the ##", ErrInvalid, i+1)
			}
			if len(out) == 0 {
				return "", nil, fmt.Errorf("%w: line %d: a subsection (##) goes inside a section (#)", ErrInvalid, i+1)
			}
			s := &out[len(out)-1]
			s.Subs = append(s.Subs, Subsection{Title: title})
			inPoint = false
		case strings.HasPrefix(line, "#"):
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if title == "" {
				return "", nil, fmt.Errorf("%w: line %d: a heading needs a title after the #", ErrInvalid, i+1)
			}
			n := len(out) + 1
			out = append(out, Section{N: n, Title: title, Anchor: fmt.Sprintf("r%d", n)})
		default:
			if len(out) == 0 {
				return "", nil, fmt.Errorf("%w: line %d: start with a section heading, e.g. \"# General conduct\"", ErrInvalid, i+1)
			}
			rules := target()
			if m := ruleLine.FindStringSubmatch(line); m != nil {
				if seen[m[1]] {
					return "", nil, fmt.Errorf("%w: line %d: rule %s appears twice", ErrInvalid, i+1, m[1])
				}
				seen[m[1]] = true
				*rules = append(*rules, Rule{Number: m[1], Text: m[2]})
				inPoint = false
				continue
			}
			if len(*rules) == 0 {
				return "", nil, fmt.Errorf("%w: line %d: expected a numbered rule, e.g. \"1.1 Stay in character\"", ErrInvalid, i+1)
			}
			r := &(*rules)[len(*rules)-1]
			if inPoint && len(r.Points) > 0 {
				p := &r.Points[len(r.Points)-1]
				if n := len(p.Sub); n > 0 {
					p.Sub[n-1] += " " + line
				} else {
					p.Text += " " + line
				}
				continue
			}
			r.Text += " " + line
		}
	}
	if len(seen) == 0 {
		return "", nil, fmt.Errorf("%w: there are no rules yet", ErrInvalid)
	}
	for i := range out {
		out[i].number()
	}
	return intro, out, nil
}

// Changed lists the numbers of rules that are new or whose text or dot
// points differ from every rule in the previous version, so renumbering
// alone (a rule inserted above) isn't flagged. Removed rules aren't
// listed: there's nothing to highlight.
func Changed(prev, next []Section) []string {
	old := map[string]bool{}
	for i := range prev {
		for _, r := range prev[i].rules() {
			old[r.content()] = true
		}
	}
	var out []string
	for i := range next {
		for _, r := range next[i].rules() {
			if !old[r.content()] {
				out = append(out, r.Number)
			}
		}
	}
	return out
}

// Format writes sections back as the rules document (see FormatDoc).
func Format(secs []Section) string { return FormatDoc("", secs) }

// FormatDoc writes the introduction and sections back as the rules
// document, numbering rules in outline order (the editor's output).
func FormatDoc(intro string, secs []Section) string {
	var b strings.Builder
	for _, para := range strings.Split(strings.ReplaceAll(intro, "\r\n", "\n"), "\n") {
		if para = oneLine(para); para != "" {
			b.WriteString("> " + para + "\n")
		}
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	for i, s := range secs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("# " + oneLine(s.Title) + "\n")
		if intro := oneLine(s.Intro); intro != "" {
			b.WriteString("> " + intro + "\n")
		}
		for j, r := range s.Rules {
			writeRule(&b, fmt.Sprintf("%d.%d", i+1, j+1), r)
		}
		for k, sub := range s.Subs {
			num := fmt.Sprintf("%d.%d", i+1, len(s.Rules)+k+1)
			b.WriteString("## " + oneLine(sub.Title) + "\n")
			if intro := oneLine(sub.Intro); intro != "" {
				b.WriteString("> " + intro + "\n")
			}
			for j, r := range sub.Rules {
				writeRule(&b, fmt.Sprintf("%s.%d", num, j+1), r)
			}
		}
	}
	return b.String()
}

func writeRule(b *strings.Builder, num string, r Rule) {
	b.WriteString(num + " " + oneLine(r.Text) + "\n")
	for _, p := range r.Points {
		b.WriteString("- " + oneLine(p.Text) + "\n")
		for _, sp := range p.Sub {
			b.WriteString("  - " + oneLine(sp) + "\n")
		}
	}
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// Current returns the live rulebook, or ok=false if none is published.
func Current(ctx context.Context, pool *pgxpool.Pool) (Doc, bool, error) {
	var d Doc
	var note *string
	err := pool.QueryRow(ctx, `
		SELECT v.id, v.body, v.change_note, v.changed, v.created_at,
		       COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), '')
		FROM rule_versions v LEFT JOIN players p ON p.id = v.created_by
		ORDER BY v.id DESC LIMIT 1`).Scan(&d.Version, &d.Body, &note, &d.Changed, &d.UpdatedAt, &d.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return Doc{}, false, nil
	}
	if err != nil {
		return Doc{}, false, err
	}
	if note != nil {
		d.ChangeNote = *note
	}
	d.Recent = time.Since(d.UpdatedAt) < ChangedFor
	d.Intro, d.Sections, err = ParseDoc(d.Body)
	if err != nil {
		return Doc{}, false, err // saved versions are always valid; this is corruption
	}
	if d.Recent {
		changed := map[string]bool{}
		for _, n := range d.Changed {
			changed[n] = true
		}
		for i := range d.Sections {
			for _, r := range d.Sections[i].rules() {
				r.Changed = changed[r.Number]
			}
		}
	}
	return d, true, nil
}

// Filter keeps rules matching q (in the rule text, its number, or the
// title of its section or subsection); empty subsections and sections are
// dropped.
func Filter(secs []Section, q string) []Section {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return secs
	}
	match := func(rs []Rule, all bool) []Rule {
		var keep []Rule
		for _, r := range rs {
			if all || strings.Contains(strings.ToLower(r.content()), q) || strings.HasPrefix(r.Number, q) || strings.HasPrefix(r.Disp, q) {
				keep = append(keep, r)
			}
		}
		return keep
	}
	var out []Section
	for _, s := range secs {
		titleHit := strings.Contains(strings.ToLower(s.Title), q)
		s.Rules = match(s.Rules, titleHit)
		var subs []Subsection
		for _, sub := range s.Subs {
			hit := titleHit || strings.Contains(strings.ToLower(sub.Title), q)
			if sub.Rules = match(sub.Rules, hit); len(sub.Rules) > 0 || hit {
				subs = append(subs, sub)
			}
		}
		s.Subs = subs
		if len(s.Rules) > 0 || len(s.Subs) > 0 {
			out = append(out, s)
		}
	}
	return out
}

// Version is one saved version, for the editor's history list.
type Version struct {
	ID        int64
	CreatedAt time.Time
	By        string
	Note      string
	Changed   []string
}

func History(ctx context.Context, pool *pgxpool.Pool, limit int) ([]Version, error) {
	rows, err := pool.Query(ctx, `
		SELECT v.id, v.created_at, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Unknown'),
		       COALESCE(v.change_note, ''), v.changed
		FROM rule_versions v LEFT JOIN players p ON p.id = v.created_by
		ORDER BY v.id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Version
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.CreatedAt, &v.By, &v.Note, &v.Changed); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ErrUnchanged is returned by Save when the text is identical to the live
// version (nothing is saved or logged).
var ErrUnchanged = errors.New("rules unchanged")

// Save publishes a new version and logs it (rules.update). note is the
// short "what changed" shown to players; it's required once rules exist.
func Save(ctx context.Context, pool *pgxpool.Pool, actorID int64, body, note string) (changed []string, err error) {
	body = strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
	note = strings.TrimSpace(note)
	if len(body) > MaxBody {
		return nil, fmt.Errorf("%w: the rules are too long (%d characters max)", ErrInvalid, MaxBody)
	}
	if len(note) > 300 {
		return nil, fmt.Errorf("%w: the change note is too long (300 characters max)", ErrInvalid)
	}
	next, err := Parse(body)
	if err != nil {
		return nil, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	// Lock so two editors saving at once can't both diff against the same
	// previous version.
	if _, err := tx.Exec(ctx, `LOCK TABLE rule_versions IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return nil, err
	}
	var prevBody string
	var prevID int64
	err = tx.QueryRow(ctx, `SELECT id, body FROM rule_versions ORDER BY id DESC LIMIT 1`).Scan(&prevID, &prevBody)
	first := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !first {
		return nil, err
	}
	if !first && prevBody == body {
		return nil, ErrUnchanged
	}
	if !first && note == "" {
		return nil, fmt.Errorf("%w: say briefly what changed; players see it at the top of the rules", ErrInvalid)
	}
	if !first {
		prev, perr := Parse(prevBody)
		if perr != nil {
			prev = nil
		}
		changed = Changed(prev, next)
	}
	var notePtr *string
	if note != "" {
		notePtr = &note
	}
	if changed == nil {
		changed = []string{}
	}
	var id int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO rule_versions (body, change_note, changed, created_by) VALUES ($1, $2, $3, $4) RETURNING id`,
		body, notePtr, changed, actorID).Scan(&id); err != nil {
		return nil, err
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actorID, Action: "rules.update", Reason: note,
		Before: map[string]any{"version": prevID},
		After:  map[string]any{"version": id, "changed": changed},
	}); err != nil {
		return nil, err
	}
	return changed, tx.Commit(ctx)
}

// ChangedLabels names the changed rules by the numbers players see,
// e.g. "1.4.1".
func (d Doc) ChangedLabels() []string {
	want := map[string]bool{}
	for _, n := range d.Changed {
		want[n] = true
	}
	var out []string
	for i := range d.Sections {
		for _, r := range d.Sections[i].rules() {
			if want[r.Number] {
				out = append(out, r.Disp)
			}
		}
	}
	return out
}
