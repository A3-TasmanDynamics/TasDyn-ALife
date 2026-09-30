package factions

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The personnel roster: the Roster page as one sheet (personnel,
// statistical, certification and miscellaneous information), with the
// details command keeps per member and the monthly roll call.

// Regions a member can be listed in.
var Regions = []string{"AU", "NZ", "Asia", "EU", "NA", "SA", "Other"}

// Statuses command can set. Probation and Suspended are shown instead when
// they apply; they come from Recruits & training and Discipline.
var Statuses = []struct{ Key, Label string }{
	{"active", "Active"}, {"semi_active", "Semi-active"}, {"loa", "LOA"}, {"reserve", "Reserve"},
}

func statusLabel(k string) string {
	for _, s := range Statuses {
		if s.Key == k {
			return s.Label
		}
	}
	return k
}

// Personnel is one roster row.
type Personnel struct {
	Member
	Badge      string
	Region     string
	Status     string // stored status key
	Shown      string // what the Status column shows (Probation / Suspended win)
	Enrolled   *time.Time
	Promoted   *time.Time // last rank change
	Department string
	Quals      map[string]bool
	Warnings   int
	Points     int
	Notes      string
	RollCall   string // this month: "present", "excused" or ""
}

// TiG is days in grade: since the last rank change, else since enrolment.
func (p Personnel) TiG() int {
	since := p.Promoted
	if since == nil {
		since = p.Enrolled
	}
	if since == nil {
		return -1
	}
	return int(time.Since(*since).Hours() / 24)
}

// MonthStart returns the first day of t's month.
func MonthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// PersonnelRoster returns every member, highest rank first, with roll call
// for month.
func PersonnelRoster(ctx context.Context, pool *pgxpool.Pool, faction string, month time.Time) ([]Personnel, error) {
	members, err := Roster(ctx, pool, faction)
	if err != nil {
		return nil, err
	}
	held, err := MemberQuals(ctx, pool, faction)
	if err != nil {
		return nil, err
	}
	posts, err := MemberDivisions(ctx, pool, faction)
	if err != nil {
		return nil, err
	}
	divs, err := Divisions(ctx, pool, faction)
	if err != nil {
		return nil, err
	}
	divName := map[string]string{}
	for _, d := range divs {
		divName[d.Key] = d.Name
	}
	standing, err := Standings(ctx, pool, faction)
	if err != nil {
		return nil, err
	}
	probation := map[int64]bool{}
	rows, err := pool.Query(ctx, `SELECT player_id FROM faction_probations WHERE faction = $1 AND status = 'active'`, faction)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		probation[id] = true
	}
	rows.Close()
	warnings := map[int64]int{}
	rows, err = pool.Query(ctx, `
		SELECT d.player_id, count(*) FROM faction_discipline d
		WHERE d.faction = $1 AND d.kind = 'mvw' AND d.expires_at > now()
		  AND NOT EXISTS (SELECT 1 FROM faction_discipline c WHERE c.corrects_id = d.id)
		  AND d.id > COALESCE((SELECT max(id) FROM faction_discipline v WHERE v.faction = $1 AND v.player_id = d.player_id AND v.kind = 'mvw_conversion'), 0)
		GROUP BY 1`, faction)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			rows.Close()
			return nil, err
		}
		warnings[id] = n
	}
	rows.Close()
	type info struct {
		badge, region, status, notes string
		enrolled                     *time.Time
	}
	infos := map[int64]info{}
	rows, err = pool.Query(ctx, `SELECT player_id, badge, region, status, notes, enrolled_on FROM faction_members WHERE faction = $1`, faction)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var x info
		if err := rows.Scan(&id, &x.badge, &x.region, &x.status, &x.notes, &x.enrolled); err != nil {
			rows.Close()
			return nil, err
		}
		infos[id] = x
	}
	rows.Close()
	rc := map[int64]string{}
	rows, err = pool.Query(ctx, `SELECT player_id, mark FROM faction_roll_call WHERE faction = $1 AND month = $2`, faction, MonthStart(month))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var m string
		if err := rows.Scan(&id, &m); err != nil {
			rows.Close()
			return nil, err
		}
		rc[id] = m
	}
	rows.Close()

	out := make([]Personnel, 0, len(members))
	for _, m := range members {
		x, ok := infos[m.ID]
		if !ok {
			x.status = "active"
		}
		p := Personnel{Member: m, Badge: x.badge, Region: x.region, Status: x.status, Notes: x.notes, Enrolled: x.enrolled,
			Promoted: m.RankSince, Quals: held[m.ID], Warnings: warnings[m.ID], Points: standing[m.ID].Points, RollCall: rc[m.ID],
			Department: "General Duties"}
		if post, ok := posts[m.ID]; ok {
			p.Department = divName[post.Key] + " · " + post.Role
		}
		switch {
		case standing[m.ID].Suspended != nil:
			p.Shown = "Suspended"
		case probation[m.ID]:
			p.Shown = "Probation"
		default:
			p.Shown = statusLabel(p.Status)
		}
		out = append(out, p)
	}
	return out, nil
}

// MemberDetails is the personnel file command edits on a member's profile.
type MemberDetails struct {
	Badge, Region, Status, Notes string
	Enrolled                     string // YYYY-MM-DD or ""
}

// UpdateMember saves a member's personnel file. Command, for members ranked
// below them. It returns what changed (empty when nothing did).
func UpdateMember(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, targetID int64, md MemberDetails) ([]string, error) {
	var err error
	if md.Badge, err = cleanText(md.Badge, 12, "the badge number", false); err != nil {
		return nil, err
	}
	if md.Notes, err = cleanText(md.Notes, 300, "the notes", false); err != nil {
		return nil, err
	}
	okRegion := md.Region == ""
	for _, r := range Regions {
		okRegion = okRegion || r == md.Region
	}
	if !okRegion {
		return nil, notAllowed("unknown region")
	}
	if statusLabel(md.Status) == md.Status {
		return nil, notAllowed("unknown status")
	}
	var enrolled *time.Time
	if md.Enrolled != "" {
		t, err := time.Parse("2006-01-02", md.Enrolled)
		if err != nil || t.After(time.Now()) || t.Year() < 2000 {
			return nil, notAllowed("the enrollment date isn't a valid past date")
		}
		enrolled = &t
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	a, err := commandOver(ctx, tx, actor, faction, targetID)
	if err != nil {
		return nil, err
	}
	if a.TargetLvl == 0 {
		return nil, notAllowed("they aren't in %s", Name(faction))
	}
	var old MemberDetails
	var oldEnrolled *time.Time
	err = tx.QueryRow(ctx, `SELECT badge, region, status, notes, enrolled_on FROM faction_members WHERE faction = $1 AND player_id = $2 FOR UPDATE`,
		faction, targetID).Scan(&old.Badge, &old.Region, &old.Status, &old.Notes, &oldEnrolled)
	if errors.Is(err, pgx.ErrNoRows) {
		old.Status = "active"
	} else if err != nil {
		return nil, err
	}
	if oldEnrolled != nil {
		old.Enrolled = oldEnrolled.Format("2006-01-02")
	}

	var ch []string
	diff := func(what, a, b string) {
		if a != b {
			if a == "" {
				a = "—"
			}
			if b == "" {
				b = "—"
			}
			ch = append(ch, fmt.Sprintf("%s %s → %s", what, a, b))
		}
	}
	diff("badge", old.Badge, md.Badge)
	diff("region", old.Region, md.Region)
	diff("status", statusLabel(old.Status), statusLabel(md.Status))
	diff("enrolled", old.Enrolled, md.Enrolled)
	if old.Notes != md.Notes {
		ch = append(ch, "notes updated")
	}
	if len(ch) == 0 {
		return nil, nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO faction_members (faction, player_id, badge, region, status, notes, enrolled_on, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())
		ON CONFLICT (faction, player_id) DO UPDATE SET badge = $3, region = $4, status = $5, notes = $6, enrolled_on = $7, updated_at = now()`,
		faction, targetID, md.Badge, md.Region, md.Status, md.Notes, enrolled); err != nil {
		return nil, err
	}
	if err := logEvent(ctx, tx, faction, actor, targetID, a.TargetLvl, "roster", playerName(ctx, tx, targetID)+": "+strings.Join(ch, "; "), ""); err != nil {
		return nil, err
	}
	return ch, tx.Commit(ctx)
}

// RollCallMonth is one month of a member's roll call.
type RollCallMonth struct {
	Month time.Time
	Mark  string // present / excused / ""
	By    string
}

// Value is the month as YYYY-MM, for forms.
func (m RollCallMonth) Value() string { return m.Month.Format("2006-01") }

// RollCallHistory returns the last n months (newest first) for a member.
func RollCallHistory(ctx context.Context, q dbtx, faction string, playerID int64, n int) ([]RollCallMonth, error) {
	marks := map[string]RollCallMonth{}
	rows, err := q.Query(ctx, `
		SELECT r.month, r.mark, COALESCE(NULLIF(b.name, ''), NULLIF(b.steam_name, ''), '')
		FROM faction_roll_call r LEFT JOIN players b ON b.id = r.by_id
		WHERE r.faction = $1 AND r.player_id = $2 AND r.month >= $3`,
		faction, playerID, MonthStart(time.Now()).AddDate(0, -(n-1), 0))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m RollCallMonth
		if err := rows.Scan(&m.Month, &m.Mark, &m.By); err != nil {
			return nil, err
		}
		marks[m.Month.Format("2006-01")] = m
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]RollCallMonth, 0, n)
	for i := 0; i < n; i++ {
		mo := MonthStart(time.Now()).AddDate(0, -i, 0)
		m, ok := marks[mo.Format("2006-01")]
		if !ok {
			m = RollCallMonth{Month: mo}
		}
		out = append(out, m)
	}
	return out, nil
}

// SetRollCall marks a member present, excused or unmarked ("") for a
// month. Command, for members ranked below them.
func SetRollCall(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, targetID int64, month time.Time, mark string) error {
	if mark != "" && mark != "present" && mark != "excused" {
		return notAllowed("unknown roll call mark")
	}
	month = MonthStart(month)
	if month.After(time.Now()) {
		return notAllowed("roll call can't be marked for a future month")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	a, err := commandOver(ctx, tx, actor, faction, targetID)
	if err != nil {
		return err
	}
	if a.TargetLvl == 0 {
		return notAllowed("they aren't in %s", Name(faction))
	}
	var old string
	if err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT mark FROM faction_roll_call WHERE faction = $1 AND player_id = $2 AND month = $3), '')`,
		faction, targetID, month).Scan(&old); err != nil {
		return err
	}
	if old == mark {
		return notAllowed("it's already marked that way")
	}
	if mark == "" {
		_, err = tx.Exec(ctx, `DELETE FROM faction_roll_call WHERE faction = $1 AND player_id = $2 AND month = $3`, faction, targetID, month)
	} else {
		_, err = tx.Exec(ctx, `
			INSERT INTO faction_roll_call (faction, player_id, month, mark, by_id) VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (faction, player_id, month) DO UPDATE SET mark = $4, by_id = $5, marked_at = now()`,
			faction, targetID, month, mark, actor.PlayerID)
	}
	if err != nil {
		return err
	}
	word := map[string]string{"present": "present", "excused": "excused", "": "not marked"}[mark]
	detail := fmt.Sprintf("%s: %s roll call %s", playerName(ctx, tx, targetID), month.Format("January 2006"), word)
	if err := logEvent(ctx, tx, faction, actor, targetID, a.TargetLvl, "roll_call", detail, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
