// Package rules stores the public server rules (/rules, layout plan
// "Public — Server rules") and their edits (/admin/rules).
//
// The rules are one plain-text document, so editing needs no special UI:
//
//	# General conduct
//	1.1 Stay in character while in-game.
//	1.2 Treat other players and staff with respect.
//	    A line that doesn't start with a number continues the rule above.
//	# Combat
//	2.1 No random deathmatch (RDM).
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
	Number  string
	Text    string
	Changed bool
}

type Section struct {
	N      int
	Title  string
	Anchor string
	Rules  []Rule
}

// Doc is the live rulebook.
type Doc struct {
	Version    int64
	Body       string
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

// Parse turns the document into sections. It rejects a rule before the
// first heading, duplicate rule numbers and an empty document.
func Parse(body string) ([]Section, error) {
	var out []Section
	seen := map[string]bool{}
	for i, raw := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "#"):
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if title == "" {
				return nil, fmt.Errorf("%w: line %d: a heading needs a title after the #", ErrInvalid, i+1)
			}
			n := len(out) + 1
			out = append(out, Section{N: n, Title: title, Anchor: fmt.Sprintf("r%d", n)})
		default:
			if len(out) == 0 {
				return nil, fmt.Errorf("%w: line %d: start with a section heading, e.g. \"# General conduct\"", ErrInvalid, i+1)
			}
			s := &out[len(out)-1]
			if m := ruleLine.FindStringSubmatch(line); m != nil {
				if seen[m[1]] {
					return nil, fmt.Errorf("%w: line %d: rule %s appears twice", ErrInvalid, i+1, m[1])
				}
				seen[m[1]] = true
				s.Rules = append(s.Rules, Rule{Number: m[1], Text: m[2]})
				continue
			}
			if len(s.Rules) == 0 {
				return nil, fmt.Errorf("%w: line %d: expected a numbered rule, e.g. \"1.1 Stay in character\"", ErrInvalid, i+1)
			}
			r := &s.Rules[len(s.Rules)-1]
			r.Text += " " + line
		}
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("%w: there are no rules yet", ErrInvalid)
	}
	return out, nil
}

// Changed lists rule numbers that are new or whose text differs between
// two versions (removed rules aren't listed: there's nothing to highlight).
func Changed(prev, next []Section) []string {
	old := map[string]string{}
	for _, s := range prev {
		for _, r := range s.Rules {
			old[r.Number] = r.Text
		}
	}
	var out []string
	for _, s := range next {
		for _, r := range s.Rules {
			if t, ok := old[r.Number]; !ok || t != r.Text {
				out = append(out, r.Number)
			}
		}
	}
	return out
}

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
	d.Sections, err = Parse(d.Body)
	if err != nil {
		return Doc{}, false, err // saved versions are always valid; this is corruption
	}
	if d.Recent {
		changed := map[string]bool{}
		for _, n := range d.Changed {
			changed[n] = true
		}
		for i := range d.Sections {
			for j := range d.Sections[i].Rules {
				d.Sections[i].Rules[j].Changed = changed[d.Sections[i].Rules[j].Number]
			}
		}
	}
	return d, true, nil
}

// Filter keeps rules matching q (in the rule text, its number or its
// section's title); sections left empty are dropped.
func Filter(secs []Section, q string) []Section {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return secs
	}
	var out []Section
	for _, s := range secs {
		titleHit := strings.Contains(strings.ToLower(s.Title), q)
		var keep []Rule
		for _, r := range s.Rules {
			if titleHit || strings.Contains(strings.ToLower(r.Text), q) || strings.HasPrefix(r.Number, q) {
				keep = append(keep, r)
			}
		}
		if len(keep) > 0 {
			s.Rules = keep
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
