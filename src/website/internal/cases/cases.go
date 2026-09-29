// Package cases is moderation (docs/GAMEPANEL_PARITY.md §3, OPERATIONS.md
// §2): cases with an append-only timeline, punishment points, bans issued
// from a case, ban appeals and anti-cheat flag review.
//
// Nothing here is ever edited or deleted in place. A correction is a new
// entry pointing at the one it corrects; revoking points or lifting a ban is
// recorded (revoked_at / lifted_at) with a case entry saying who and why.
// Every write also goes to staff_log, so it shows in the Staff Log and
// #staff-log. Callers check permissions (cases.*, bans.*, anticheat.review).
package cases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
)

// ErrNotAllowed wraps a rule violation with a human-readable reason.
var ErrNotAllowed = errors.New("not allowed")

func notAllowed(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrNotAllowed, fmt.Sprintf(format, args...))
}

// Actor is the staff member acting.
type Actor struct {
	PlayerID int64
	Source   string // audit.SourceWebsite / audit.SourceDiscord
}

// Types are the case types, in menu order.
var Types = []struct{ Key, Label string }{
	{"exploit", "Exploit"}, {"cheating", "Cheating"}, {"rdm_vdm", "RDM / VDM"}, {"conduct", "Conduct"},
	{"chat", "Chat"}, {"compensation", "Compensation"}, {"ban_appeal", "Ban appeal"}, {"other", "Other"},
}

// TypeLabel returns the display name of a case type.
func TypeLabel(key string) string {
	for _, t := range Types {
		if t.Key == key {
			return t.Label
		}
	}
	return key
}

func validType(key string) bool {
	for _, t := range Types {
		if t.Key == key {
			return true
		}
	}
	return false
}

const nameExpr = `COALESCE(NULLIF(%[1]s.name, ''), NULLIF(%[1]s.steam_name, ''), 'Player #' || %[1]s.id)`

func name(alias string) string { return fmt.Sprintf(nameExpr, alias) }

func cleanText(s string, max int, what string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", notAllowed("%s is required", what)
	}
	if len(s) > max {
		return "", notAllowed("%s is too long (%d characters max)", what, max)
	}
	return s, nil
}

func addEntry(ctx context.Context, tx pgx.Tx, caseID, authorID int64, kind, body string, corrects int64, ref string) (int64, error) {
	var id int64
	var correctsPtr *int64
	if corrects > 0 {
		correctsPtr = &corrects
	}
	var refPtr *string
	if ref != "" {
		refPtr = &ref
	}
	err := tx.QueryRow(ctx, `
		INSERT INTO staff_case_entries (case_id, author_id, kind, body, corrects_entry_id, action_ref)
		VALUES ($1, NULLIF($2, 0), $3, $4, $5, $6) RETURNING id`,
		caseID, authorID, kind, body, correctsPtr, refPtr).Scan(&id)
	return id, err
}

// lockCase locks a case row and returns its status.
func lockCase(ctx context.Context, tx pgx.Tx, caseID int64) (string, error) {
	var status string
	err := tx.QueryRow(ctx, `SELECT status FROM staff_cases WHERE id = $1 FOR UPDATE`, caseID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", notAllowed("that case doesn't exist")
	}
	return status, err
}

// Open creates a case led by the actor, with the given subjects. flagID > 0
// links (and resolves) the anti-cheat flag it was opened from.
func Open(ctx context.Context, pool *pgxpool.Pool, actor Actor, caseType, summary string, subjects []int64, flagID int64, note string) (int64, error) {
	if !validType(caseType) {
		return 0, notAllowed("pick a case type")
	}
	summary, err := cleanText(summary, 200, "a summary")
	if err != nil {
		return 0, err
	}
	if len(summary) < 3 {
		return 0, notAllowed("the summary is too short")
	}
	if len(subjects) == 0 {
		return 0, notAllowed("add at least one player")
	}
	note = strings.TrimSpace(note)
	if len(note) > 4000 {
		return 0, notAllowed("the first entry is too long (4000 characters max)")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var flagPtr *int64
	if flagID > 0 {
		flagPtr = &flagID
	}
	var id int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO staff_cases (case_type, summary, lead_staff_id, flag_id) VALUES ($1, $2, $3, $4) RETURNING id`,
		caseType, summary, actor.PlayerID, flagPtr).Scan(&id); err != nil {
		return 0, err
	}
	seen := map[int64]bool{}
	for _, pid := range subjects {
		if seen[pid] {
			continue
		}
		seen[pid] = true
		if pid == actor.PlayerID {
			return 0, notAllowed("you can't open a case about yourself")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO staff_case_participants (case_id, player_id, role) VALUES ($1, $2, 'subject')`, id, pid); err != nil {
			var exists bool
			_ = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM players WHERE id = $1)`, pid).Scan(&exists)
			if !exists {
				return 0, notAllowed("player #%d doesn't exist", pid)
			}
			return 0, err
		}
	}
	body := "Case opened."
	if note != "" {
		body = note
	}
	if _, err := addEntry(ctx, tx, id, actor.PlayerID, "note", body, 0, ""); err != nil {
		return 0, err
	}
	if flagID > 0 {
		tag, err := tx.Exec(ctx, `
			UPDATE anti_cheat_flags SET resolution = 'case', reviewed_by = $2, reviewed_at = now(), resolution_note = $3
			WHERE id = $1`, flagID, actor.PlayerID, fmt.Sprintf("Handled in case #%d", id))
		if err != nil {
			return 0, err
		}
		if tag.RowsAffected() == 0 {
			return 0, notAllowed("anti-cheat flag #%d doesn't exist", flagID)
		}
		if _, err := addEntry(ctx, tx, id, actor.PlayerID, "flag", fmt.Sprintf("Opened from anti-cheat flag #%d.", flagID), 0, fmt.Sprintf("anti_cheat_flags:%d", flagID)); err != nil {
			return 0, err
		}
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actor.PlayerID, TargetID: subjects[0], Action: "case.open", Source: actor.Source,
		Reason: fmt.Sprintf("#%d %s: %s", id, TypeLabel(caseType), summary),
		After:  map[string]any{"case": id, "type": caseType, "players": len(seen)},
	}); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

// Participant is someone involved in a case.
type Participant struct {
	PlayerID     int64
	Name         string
	Role         string
	ActivePoints int
	Banned       bool
	PoliceLevel  int
	EMSLevel     int
}

// Case is one case with its people.
type Case struct {
	ID        int64
	Type      string
	TypeLabel string
	Status    string
	Summary   string
	Outcome   string
	LeadID    int64
	Lead      string
	FlagID    int64
	CreatedAt time.Time
	ClosedAt  *time.Time
	Players   []Participant // subjects and related players
	Assisting []Participant // assisting staff
}

// Get loads one case.
func Get(ctx context.Context, pool *pgxpool.Pool, id int64) (Case, error) {
	var c Case
	err := pool.QueryRow(ctx, `
		SELECT c.id, c.case_type, c.status, c.summary, COALESCE(c.outcome, ''), COALESCE(c.lead_staff_id, 0),
		       COALESCE(`+name("l")+`, 'Unknown'), COALESCE(c.flag_id, 0), c.created_at, c.closed_at
		FROM staff_cases c LEFT JOIN players l ON l.id = c.lead_staff_id
		WHERE c.id = $1`, id).Scan(&c.ID, &c.Type, &c.Status, &c.Summary, &c.Outcome, &c.LeadID, &c.Lead,
		&c.FlagID, &c.CreatedAt, &c.ClosedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, notAllowed("that case doesn't exist")
	}
	if err != nil {
		return c, err
	}
	c.TypeLabel = TypeLabel(c.Type)
	rows, err := pool.Query(ctx, `
		SELECT p.id, `+name("p")+`, cp.role,
		       COALESCE((SELECT sum(points) FROM punishment_points pp WHERE pp.player_id = p.id
		                 AND pp.revoked_at IS NULL AND (pp.expires_at IS NULL OR pp.expires_at > now())), 0),
		       EXISTS (SELECT 1 FROM banlist b WHERE b.uid = p.uid AND (b.expires_at IS NULL OR b.expires_at > now())),
		       COALESCE(p.cop_level, 0), COALESCE(p.medic_level, 0)
		FROM staff_case_participants cp JOIN players p ON p.id = cp.player_id
		WHERE cp.case_id = $1 ORDER BY cp.role = 'assisting_staff', cp.role <> 'subject', cp.id`, id)
	if err != nil {
		return c, err
	}
	defer rows.Close()
	for rows.Next() {
		var p Participant
		if err := rows.Scan(&p.PlayerID, &p.Name, &p.Role, &p.ActivePoints, &p.Banned, &p.PoliceLevel, &p.EMSLevel); err != nil {
			return c, err
		}
		if p.Role == "assisting_staff" {
			c.Assisting = append(c.Assisting, p)
		} else {
			c.Players = append(c.Players, p)
		}
	}
	return c, rows.Err()
}

// Entry is one timeline entry.
type Entry struct {
	ID          int64
	N           int // position in the case, from 1
	Kind        string
	Body        string
	By          string
	When        time.Time
	Corrects    int   // N of the entry this corrects, 0 = none
	CorrectedBy []int // Ns of entries correcting this one
}

// Entries returns the case's timeline, oldest first.
func Entries(ctx context.Context, pool *pgxpool.Pool, caseID int64) ([]Entry, error) {
	rows, err := pool.Query(ctx, `
		SELECT e.id, e.kind, e.body, COALESCE(`+name("a")+`, 'System'), e.created_at, COALESCE(e.corrects_entry_id, 0)
		FROM staff_case_entries e LEFT JOIN players a ON a.id = e.author_id
		WHERE e.case_id = $1 ORDER BY e.id`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	nOf := map[int64]int{}
	corrects := map[int]int64{}
	for rows.Next() {
		var e Entry
		var c int64
		if err := rows.Scan(&e.ID, &e.Kind, &e.Body, &e.By, &e.When, &c); err != nil {
			return nil, err
		}
		e.N = len(out) + 1
		nOf[e.ID] = e.N
		if c > 0 {
			corrects[e.N] = c
		}
		out = append(out, e)
	}
	for n, target := range corrects {
		if tn, ok := nOf[target]; ok {
			out[n-1].Corrects = tn
			out[tn-1].CorrectedBy = append(out[tn-1].CorrectedBy, n)
		}
	}
	return out, rows.Err()
}

// AddNote appends an entry. correctsN > 0 makes it a correction of that
// entry (by its number in this case).
func AddNote(ctx context.Context, pool *pgxpool.Pool, actor Actor, caseID int64, body string, correctsN int) error {
	body, err := cleanText(body, 4000, "the entry")
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := lockCase(ctx, tx, caseID); err != nil {
		return err
	}
	kind, corrects := "note", int64(0)
	if correctsN > 0 {
		err := tx.QueryRow(ctx, `SELECT id FROM staff_case_entries WHERE case_id = $1 ORDER BY id OFFSET $2 LIMIT 1`,
			caseID, correctsN-1).Scan(&corrects)
		if errors.Is(err, pgx.ErrNoRows) {
			return notAllowed("entry #%d doesn't exist in this case", correctsN)
		}
		if err != nil {
			return err
		}
		kind = "correction"
	}
	if _, err := addEntry(ctx, tx, caseID, actor.PlayerID, kind, body, corrects, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetStatus closes or reopens a case. outcome (optional, on close) is the
// short result shown in the case list.
func SetStatus(ctx context.Context, pool *pgxpool.Pool, actor Actor, caseID int64, status, outcome, reason string) error {
	if status != "open" && status != "closed" {
		return notAllowed("unknown status")
	}
	reason, err := cleanText(reason, 1000, "a reason")
	if err != nil {
		return err
	}
	outcome = strings.TrimSpace(outcome)
	if len(outcome) > 80 {
		return notAllowed("the outcome is too long (80 characters max)")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	cur, err := lockCase(ctx, tx, caseID)
	if err != nil {
		return err
	}
	if cur == status {
		return notAllowed("the case is already %s", status)
	}
	if status == "closed" {
		_, err = tx.Exec(ctx, `UPDATE staff_cases SET status = 'closed', closed_at = now(), outcome = COALESCE(NULLIF($2, ''), outcome) WHERE id = $1`, caseID, outcome)
	} else {
		_, err = tx.Exec(ctx, `UPDATE staff_cases SET status = 'open', closed_at = NULL WHERE id = $1`, caseID)
	}
	if err != nil {
		return err
	}
	body := map[string]string{"closed": "Closed: ", "open": "Reopened: "}[status] + reason
	if outcome != "" {
		body += " (outcome: " + outcome + ")"
	}
	if _, err := addEntry(ctx, tx, caseID, actor.PlayerID, "status", body, 0, ""); err != nil {
		return err
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actor.PlayerID, Action: map[string]string{"closed": "case.close", "open": "case.reopen"}[status],
		Source: actor.Source, Reason: fmt.Sprintf("#%d: %s", caseID, reason),
		After: map[string]any{"case": caseID, "outcome": outcome},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// AddParticipant adds a player (role subject/related/reporter/witness) or
// a staff member (assisting_staff) to a case.
func AddParticipant(ctx context.Context, pool *pgxpool.Pool, actor Actor, caseID, playerID int64, role string) error {
	switch role {
	case "subject", "related", "reporter", "witness", "assisting_staff":
	default:
		return notAllowed("unknown role")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := lockCase(ctx, tx, caseID); err != nil {
		return err
	}
	var who string
	var staff bool
	err = tx.QueryRow(ctx, `SELECT `+name("p")+`, p.staff_rank_id IS NOT NULL FROM players p WHERE p.id = $1`, playerID).Scan(&who, &staff)
	if errors.Is(err, pgx.ErrNoRows) {
		return notAllowed("that player doesn't exist")
	}
	if err != nil {
		return err
	}
	if role == "assisting_staff" && !staff {
		return notAllowed("%s isn't staff", who)
	}
	if role != "assisting_staff" && playerID == actor.PlayerID {
		return notAllowed("you can't add yourself as a player in a case")
	}
	tag, err := tx.Exec(ctx, `INSERT INTO staff_case_participants (case_id, player_id, role) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		caseID, playerID, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return notAllowed("%s is already on this case", who)
	}
	label := map[string]string{"subject": "subject", "related": "related player", "reporter": "reporter",
		"witness": "witness", "assisting_staff": "assisting staff"}[role]
	if _, err := addEntry(ctx, tx, caseID, actor.PlayerID, "participant", fmt.Sprintf("Added %s as %s.", who, label), 0, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Points is one punishment points award.
type Points struct {
	ID        int64
	CaseID    int64
	PlayerID  int64
	Player    string
	Points    int
	Rules     string
	Comment   string
	By        string
	CreatedAt time.Time
	ExpiresAt *time.Time
	RevokedAt *time.Time
	Active    bool
}

// IssuePoints awards punishment points to a player on the case.
func IssuePoints(ctx context.Context, pool *pgxpool.Pool, actor Actor, caseID, playerID int64, points int, rules, comment string, expires *time.Time) error {
	if points < 1 || points > 100 {
		return notAllowed("points must be between 1 and 100")
	}
	rules, err := cleanText(rules, 100, "the rules broken")
	if err != nil {
		return err
	}
	comment = strings.TrimSpace(comment)
	if len(comment) > 1000 {
		return notAllowed("the comment is too long (1000 characters max)")
	}
	if expires != nil && !expires.After(time.Now()) {
		return notAllowed("the end date must be in the future")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if st, err := lockCase(ctx, tx, caseID); err != nil {
		return err
	} else if st != "open" {
		return notAllowed("reopen the case first")
	}
	var who string
	err = tx.QueryRow(ctx, `
		SELECT `+name("p")+` FROM staff_case_participants cp JOIN players p ON p.id = cp.player_id
		WHERE cp.case_id = $1 AND cp.player_id = $2 AND cp.role IN ('subject', 'related') LIMIT 1`, caseID, playerID).Scan(&who)
	if errors.Is(err, pgx.ErrNoRows) {
		return notAllowed("add them to the case first")
	}
	if err != nil {
		return err
	}
	var commentPtr *string
	if comment != "" {
		commentPtr = &comment
	}
	var pid int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO punishment_points (case_id, player_id, points, rules, comment, issued_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		caseID, playerID, points, rules, commentPtr, actor.PlayerID, expires).Scan(&pid); err != nil {
		return err
	}
	body := fmt.Sprintf("%d point%s to %s for rule %s.", points, plural(points), who, rules)
	if expires != nil {
		body += " Stops counting " + expires.Format("2 Jan 2006") + "."
	}
	if comment != "" {
		body += " " + comment
	}
	if _, err := addEntry(ctx, tx, caseID, actor.PlayerID, "points", body, 0, fmt.Sprintf("punishment_points:%d", pid)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE staff_cases SET outcome = $2 WHERE id = $1`, caseID, fmt.Sprintf("%d point%s", points, plural(points))); err != nil {
		return err
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actor.PlayerID, TargetID: playerID, Action: "player.warn", Source: actor.Source,
		Reason: fmt.Sprintf("%d points, rule %s (case #%d)", points, rules, caseID),
		After:  map[string]any{"case": caseID, "points": points, "rules": rules},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RevokePoints stops an award counting, recorded in its case.
func RevokePoints(ctx context.Context, pool *pgxpool.Pool, actor Actor, pointsID int64, reason string) error {
	reason, err := cleanText(reason, 1000, "a reason")
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var caseID, playerID int64
	var pts int
	var revoked *time.Time
	err = tx.QueryRow(ctx, `SELECT case_id, player_id, points, revoked_at FROM punishment_points WHERE id = $1 FOR UPDATE`, pointsID).
		Scan(&caseID, &playerID, &pts, &revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return notAllowed("those points don't exist")
	}
	if err != nil {
		return err
	}
	if revoked != nil {
		return notAllowed("those points were already revoked")
	}
	if _, err := tx.Exec(ctx, `UPDATE punishment_points SET revoked_at = now(), revoked_by = $2 WHERE id = $1`, pointsID, actor.PlayerID); err != nil {
		return err
	}
	if _, err := addEntry(ctx, tx, caseID, actor.PlayerID, "points_revoked", fmt.Sprintf("Revoked %d point%s: %s", pts, plural(pts), reason), 0, fmt.Sprintf("punishment_points:%d", pointsID)); err != nil {
		return err
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actor.PlayerID, TargetID: playerID, Action: "player.warn_revoke", Source: actor.Source,
		Reason: fmt.Sprintf("%d points revoked (case #%d): %s", pts, caseID, reason),
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// PlayerPoints lists a player's punishment points, newest first, and their
// active total (not expired, not revoked).
func PlayerPoints(ctx context.Context, pool *pgxpool.Pool, playerID int64) ([]Points, int, error) {
	rows, err := pool.Query(ctx, `
		SELECT pp.id, pp.case_id, pp.points, pp.rules, COALESCE(pp.comment, ''), COALESCE(`+name("i")+`, 'Unknown'),
		       pp.created_at, pp.expires_at, pp.revoked_at
		FROM punishment_points pp LEFT JOIN players i ON i.id = pp.issued_by
		WHERE pp.player_id = $1 ORDER BY pp.id DESC`, playerID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Points
	total := 0
	for rows.Next() {
		p := Points{PlayerID: playerID}
		if err := rows.Scan(&p.ID, &p.CaseID, &p.Points, &p.Rules, &p.Comment, &p.By, &p.CreatedAt, &p.ExpiresAt, &p.RevokedAt); err != nil {
			return nil, 0, err
		}
		p.Active = p.RevokedAt == nil && (p.ExpiresAt == nil || p.ExpiresAt.After(time.Now()))
		if p.Active {
			total += p.Points
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// ListRow is one row of the case list.
type ListRow struct {
	ID        int64
	Status    string
	TypeLabel string
	Summary   string
	Players   string
	Lead      string
	Assist    string
	Outcome   string
	Opened    time.Time
}

// Filter narrows the case list. Empty fields don't filter.
type Filter struct {
	Status string // open / closed
	Type   string
	LeadID int64
	Since  time.Duration // 0 = all time
	Q      string        // participant name, past name, Steam64, staff name, summary or #id
	Mine   int64         // cases this player leads or assists
	Limit  int
}

// List returns cases matching f, newest first.
func List(ctx context.Context, pool *pgxpool.Pool, f Filter) ([]ListRow, error) {
	if f.Limit <= 0 {
		f.Limit = 200
	}
	q := strings.TrimPrefix(strings.TrimSpace(f.Q), "#")
	var since *time.Time
	if f.Since > 0 {
		t := time.Now().Add(-f.Since)
		since = &t
	}
	rows, err := pool.Query(ctx, `
		SELECT c.id, c.status, c.case_type, c.summary,
		       COALESCE((SELECT string_agg(`+name("p")+`, ', ' ORDER BY cp.id) FROM staff_case_participants cp
		                 JOIN players p ON p.id = cp.player_id WHERE cp.case_id = c.id AND cp.role IN ('subject', 'related')), ''),
		       COALESCE(`+name("l")+`, 'Unknown'),
		       COALESCE((SELECT string_agg(`+name("p")+`, ', ') FROM staff_case_participants cp
		                 JOIN players p ON p.id = cp.player_id WHERE cp.case_id = c.id AND cp.role = 'assisting_staff'), ''),
		       COALESCE(c.outcome, ''), c.created_at
		FROM staff_cases c LEFT JOIN players l ON l.id = c.lead_staff_id
		WHERE ($1 = '' OR c.status = $1) AND ($2 = '' OR c.case_type = $2) AND ($3 = 0 OR c.lead_staff_id = $3)
		  AND ($4::timestamptz IS NULL OR c.created_at >= $4)
		  AND ($5 = 0 OR c.lead_staff_id = $5 OR EXISTS (SELECT 1 FROM staff_case_participants cp
		        WHERE cp.case_id = c.id AND cp.player_id = $5 AND cp.role = 'assisting_staff'))
		  AND ($6 = '' OR c.id::text = $6 OR c.summary ILIKE '%' || $6 || '%' OR l.name ILIKE '%' || $6 || '%'
		       OR EXISTS (SELECT 1 FROM staff_case_participants cp JOIN players p ON p.id = cp.player_id
		                  WHERE cp.case_id = c.id AND (p.name ILIKE '%' || $6 || '%' OR p.steam_name ILIKE '%' || $6 || '%'
		                        OR p.uid = $6 OR EXISTS (SELECT 1 FROM player_aliases a WHERE a.player_id = p.id AND a.name ILIKE '%' || $6 || '%'))))
		ORDER BY c.id DESC LIMIT $7`, f.Status, f.Type, f.LeadID, since, f.Mine, q, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ListRow
	for rows.Next() {
		var r ListRow
		var typ string
		if err := rows.Scan(&r.ID, &r.Status, &typ, &r.Summary, &r.Players, &r.Lead, &r.Assist, &r.Outcome, &r.Opened); err != nil {
			return nil, err
		}
		r.TypeLabel = TypeLabel(typ)
		out = append(out, r)
	}
	return out, rows.Err()
}

// PlayerCases lists the cases a player is involved in (as a player, not staff).
func PlayerCases(ctx context.Context, pool *pgxpool.Pool, playerID int64) ([]ListRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT c.id, c.status, c.case_type, c.summary, COALESCE(c.outcome, ''), c.created_at
		FROM staff_cases c JOIN staff_case_participants cp ON cp.case_id = c.id
		WHERE cp.player_id = $1 AND cp.role <> 'assisting_staff' ORDER BY c.id DESC LIMIT 50`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ListRow
	for rows.Next() {
		var r ListRow
		var typ string
		if err := rows.Scan(&r.ID, &r.Status, &typ, &r.Summary, &r.Outcome, &r.Opened); err != nil {
			return nil, err
		}
		r.TypeLabel = TypeLabel(typ)
		out = append(out, r)
	}
	return out, rows.Err()
}

// Activity is a staff member's case counts (led / assisted).
type Activity struct {
	LedWeek, AssistWeek, Led30, Assist30, LedAll, AssistAll int
}

func StaffActivity(ctx context.Context, pool *pgxpool.Pool, staffID int64) (Activity, error) {
	var a Activity
	err := pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE c.lead_staff_id = $1 AND c.created_at > now() - interval '7 days'),
		       count(*) FILTER (WHERE c.lead_staff_id <> $1 AND c.created_at > now() - interval '7 days'),
		       count(*) FILTER (WHERE c.lead_staff_id = $1 AND c.created_at > now() - interval '30 days'),
		       count(*) FILTER (WHERE c.lead_staff_id <> $1 AND c.created_at > now() - interval '30 days'),
		       count(*) FILTER (WHERE c.lead_staff_id = $1),
		       count(*) FILTER (WHERE c.lead_staff_id <> $1)
		FROM staff_cases c
		WHERE c.lead_staff_id = $1 OR EXISTS (SELECT 1 FROM staff_case_participants cp
		      WHERE cp.case_id = c.id AND cp.player_id = $1 AND cp.role = 'assisting_staff')`, staffID).
		Scan(&a.LedWeek, &a.AssistWeek, &a.Led30, &a.Assist30, &a.LedAll, &a.AssistAll)
	return a, err
}

// PerDay returns how many cases were opened on each of the last n days
// (oldest first), in loc.
func PerDay(ctx context.Context, pool *pgxpool.Pool, n int, loc *time.Location) ([]int, []time.Time, error) {
	start := time.Now().In(loc).AddDate(0, 0, -(n - 1))
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
	counts := make([]int, n)
	days := make([]time.Time, n)
	for i := range days {
		days[i] = start.AddDate(0, 0, i)
	}
	rows, err := pool.Query(ctx, `SELECT created_at FROM staff_cases WHERE created_at >= $1`, start)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t time.Time
		if err := rows.Scan(&t); err != nil {
			return nil, nil, err
		}
		t = t.In(loc)
		d := int(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc).Sub(start).Hours() / 24)
		if d >= 0 && d < n {
			counts[d]++
		}
	}
	return counts, days, rows.Err()
}
