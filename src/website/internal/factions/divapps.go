package factions

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/notify"
)

// Division applications: a faction member applies to join a division; the
// division's reviewers accept (posting them at the entry role) or turn it
// down with a note.

// DivisionCooldown is how long after being turned down before applying to
// the same division again.
const DivisionCooldown = 7 * 24 * time.Hour

// GeneralDuties is the key used for the general duties "division": members
// in no specialist division. It has no postings or applications.
const GeneralDuties = "GD"

// LeadRoles are the roles that make up the division's command: the top two
// (Commander and Second in Command), or just the Commander in a division
// with fewer than three roles.
func (d Division) LeadRoles() []string {
	if len(d.Roles) >= 3 {
		return d.Roles[:2]
	}
	return d.Roles[:1]
}

// DivisionApp is one division application.
type DivisionApp struct {
	ID           int64
	Division     string
	DivisionName string
	PlayerID     int64
	Name         string
	Level        int
	Rank         string
	Why          string
	Availability string
	Experience   string
	Status       string
	DecidedBy    string
	Note         string
	Created      time.Time
	Decided      *time.Time
	HoursInFac   int64
	Points       int
	Quals        []string
}

const divAppCols = `
	SELECT a.id, a.division_key, dv.name, a.player_id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id),
	       COALESCE(p.%[1]s, 0), a.why, a.availability, a.experience, a.status,
	       COALESCE(NULLIF(b.name, ''), NULLIF(b.steam_name, ''), ''), a.decision_note, a.created_at, a.decided_at,
	       COALESCE(p.%[2]s, 0) / 3600,
	       COALESCE((SELECT sum(points)::int FROM faction_discipline d WHERE d.faction = a.faction AND d.player_id = a.player_id AND d.expires_at > now()), 0),
	       COALESCE((SELECT array_agg(qual_key ORDER BY qual_key) FROM faction_member_quals q WHERE q.faction = a.faction AND q.player_id = a.player_id), '{}')
	FROM faction_division_applications a JOIN players p ON p.id = a.player_id LEFT JOIN players b ON b.id = a.decided_by
	JOIN faction_divisions dv ON dv.faction = a.faction AND dv.key = a.division_key`

func scanDivApps(ctx context.Context, q dbtx, faction string, rows pgx.Rows) ([]DivisionApp, error) {
	defer rows.Close()
	var out []DivisionApp
	for rows.Next() {
		var x DivisionApp
		if err := rows.Scan(&x.ID, &x.Division, &x.DivisionName, &x.PlayerID, &x.Name, &x.Level, &x.Why, &x.Availability, &x.Experience, &x.Status,
			&x.DecidedBy, &x.Note, &x.Created, &x.Decided, &x.HoursInFac, &x.Points, &x.Quals); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ranks, err := Ranks(ctx, q, faction)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Rank = RankFor(ranks, out[i].Level).Label()
	}
	return out, nil
}

func divAppQuery(faction string) string {
	play := map[string]string{"police": "cop_playtime_seconds", "ems": "medic_playtime_seconds"}[faction]
	return fmt.Sprintf(divAppCols, column[faction], play)
}

// DivisionQueue returns a division's applications: pending ones, or recent
// decided ones when pending is false.
func DivisionQueue(ctx context.Context, q dbtx, faction, divKey string, pending bool) ([]DivisionApp, error) {
	rows, err := q.Query(ctx, divAppQuery(faction)+`
		WHERE a.faction = $1 AND a.division_key = $2 AND (a.status = 'pending') = $3
		ORDER BY CASE WHEN $3 THEN a.id END, a.decided_at DESC NULLS LAST LIMIT 50`, faction, divKey, pending)
	if err != nil {
		return nil, err
	}
	return scanDivApps(ctx, q, faction, rows)
}

// MyDivisionApps returns a player's applications in faction, newest first.
func MyDivisionApps(ctx context.Context, q dbtx, faction string, playerID int64) ([]DivisionApp, error) {
	rows, err := q.Query(ctx, divAppQuery(faction)+`
		WHERE a.faction = $1 AND a.player_id = $2 ORDER BY a.id DESC LIMIT 30`, faction, playerID)
	if err != nil {
		return nil, err
	}
	return scanDivApps(ctx, q, faction, rows)
}

// PendingDivisionApps counts pending applications per division.
func PendingDivisionApps(ctx context.Context, q dbtx, faction string) (map[string]int, error) {
	rows, err := q.Query(ctx, `SELECT division_key, count(*) FROM faction_division_applications WHERE faction = $1 AND status = 'pending' GROUP BY 1`, faction)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, rows.Err()
}

// ApplyToDivision sends a member's application to a division.
func ApplyToDivision(ctx context.Context, pool *pgxpool.Pool, playerID int64, faction, divKey, why, availability, experience string) error {
	var err error
	if why, err = cleanText(why, 1000, "why you want to join", true); err != nil {
		return err
	}
	if availability, err = cleanText(availability, 300, "your availability", true); err != nil {
		return err
	}
	if experience, err = cleanText(experience, 1000, "your experience", false); err != nil {
		return err
	}
	if !Valid(faction) {
		return notAllowed("unknown faction")
	}
	lvl, err := levelOf(ctx, pool, faction, playerID)
	if err != nil {
		return err
	}
	if lvl == 0 {
		return notAllowed("you need to be in %s to apply to its divisions", Name(faction))
	}
	divs, err := Divisions(ctx, pool, faction)
	if err != nil {
		return err
	}
	var d Division
	for _, x := range divs {
		if x.Key == divKey {
			d = x
		}
	}
	if d.Key == "" {
		return notAllowed("unknown division")
	}
	var in bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM faction_member_divisions WHERE faction = $1 AND player_id = $2 AND division_key = $3)`,
		faction, playerID, divKey).Scan(&in); err != nil {
		return err
	}
	if in {
		return notAllowed("you're already in %s", d.Name)
	}
	if d.IsAdmin {
		var cab bool
		_ = pool.QueryRow(ctx, `SELECT COALESCE((SELECT is_cabinet AND is_command FROM faction_rank_names WHERE faction = $1 AND level = $2), false)`, faction, lvl).Scan(&cab)
		if cab {
			return notAllowed("cabinet is part of %s automatically", d.Name)
		}
	}
	if lvl < d.MinLevel {
		ranks, _ := Ranks(ctx, pool, faction)
		return notAllowed("%s needs %s or above", d.Name, RankFor(ranks, d.MinLevel).Label())
	}
	var last *time.Time
	_ = pool.QueryRow(ctx, `SELECT max(decided_at) FROM faction_division_applications WHERE faction = $1 AND player_id = $2 AND division_key = $3 AND status = 'rejected'`,
		faction, playerID, divKey).Scan(&last)
	if last != nil && time.Since(*last) < DivisionCooldown {
		return notAllowed("you can apply to %s again from %s", d.Name, last.Add(DivisionCooldown).Local().Format("2 Jan"))
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO faction_division_applications (faction, division_key, player_id, why, availability, experience) VALUES ($1, $2, $3, $4, $5, $6)`,
		faction, divKey, playerID, why, availability, experience)
	if err != nil && isUnique(err) {
		return notAllowed("you already have an application to %s waiting", d.Name)
	}
	return err
}

// WithdrawDivisionApp withdraws the player's own pending application.
func WithdrawDivisionApp(ctx context.Context, pool *pgxpool.Pool, playerID, id int64) error {
	tag, err := pool.Exec(ctx, `UPDATE faction_division_applications SET status = 'withdrawn', decided_at = now()
		WHERE id = $1 AND player_id = $2 AND status = 'pending'`, id, playerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return notAllowed("that application isn't waiting any more")
	}
	return nil
}

// Reviewer is who may review a division's applications, as a viewer.
// Management is checked by the caller (staff override).
func (c Command) CanReview(d Division) bool {
	switch {
	case c.Cabinet:
		return true
	case d.IsAdmin:
		return c.AdminCommander
	case c.Admin:
		return true
	}
	return slices.Contains(c.Leads, d.Key)
}

// reviewer checks the actor may review applications to d.
func reviewer(ctx context.Context, q dbtx, actor Actor, faction string, d Division) (int, error) {
	if actor.Via == ViaStaffOverride {
		return 0, nil
	}
	c, ok, err := CommandIn(ctx, q, actor.PlayerID, faction)
	if err != nil {
		return 0, err
	}
	if !ok || !c.CanReview(d) {
		if d.IsAdmin {
			return 0, notAllowed("only cabinet, Management or the %s %s review %s applications", d.Name, d.CommanderRole(), d.Name)
		}
		return 0, notAllowed("only Administration, cabinet or %s command review its applications", d.Name)
	}
	return c.Level, nil
}

// DecideDivisionApp accepts (posting them at the entry role) or turns down
// an application. Turning one down needs a note, which the applicant sees.
func DecideDivisionApp(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, id int64, accept bool, note string) error {
	var err error
	if note, err = cleanText(note, 500, "a note", !accept); err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var playerID int64
	var divKey, status string
	err = tx.QueryRow(ctx, `SELECT player_id, division_key, status FROM faction_division_applications WHERE id = $1 AND faction = $2 FOR UPDATE`,
		id, faction).Scan(&playerID, &divKey, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return notAllowed("that application doesn't exist")
	} else if err != nil {
		return err
	}
	if status != "pending" {
		return notAllowed("that application has already been decided")
	}
	divs, err := Divisions(ctx, tx, faction)
	if err != nil {
		return err
	}
	var d Division
	for _, x := range divs {
		if x.Key == divKey {
			d = x
		}
	}
	actorLevel, err := reviewer(ctx, tx, actor, faction, d)
	if err != nil {
		return err
	}
	if playerID == actor.PlayerID {
		return notAllowed("you can't decide your own application")
	}
	lvl, err := levelOf(ctx, tx, faction, playerID)
	if err != nil {
		return err
	}
	name := playerName(ctx, tx, playerID)
	newStatus, detail := "rejected", name+"'s application to "+d.Name+" turned down"
	if accept {
		if lvl == 0 {
			return notAllowed("they're no longer in %s", Name(faction))
		}
		if lvl < d.MinLevel {
			ranks, _ := Ranks(ctx, tx, faction)
			return notAllowed("%s needs %s or above", d.Name, RankFor(ranks, d.MinLevel).Label())
		}
		if d.RequiredQual != "" {
			held, err := memberQuals(ctx, tx, faction, playerID)
			if err != nil {
				return err
			}
			if !held[d.RequiredQual] {
				return notAllowed("%s needs the %s qualification; record their pass first", d.Name, d.RequiredQual)
			}
		}
		if !d.IsAdmin { // one specialist division at a time
			if _, err := tx.Exec(ctx, `
				DELETE FROM faction_member_divisions md USING faction_divisions fd
				WHERE md.faction = $1 AND md.player_id = $2 AND fd.faction = md.faction AND fd.key = md.division_key AND NOT fd.is_admin`,
				faction, playerID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO faction_member_divisions (faction, player_id, division_key, role) VALUES ($1, $2, $3, $4)
			ON CONFLICT (faction, player_id, division_key) DO NOTHING`, faction, playerID, divKey, d.EntryRole()); err != nil {
			return err
		}
		newStatus, detail = "accepted", name+" joined "+d.Name+" as "+d.EntryRole()+" (application accepted)"
	}
	if _, err := tx.Exec(ctx, `UPDATE faction_division_applications SET status = $2, decided_by = $3, decided_at = now(), decision_note = $4 WHERE id = $1`,
		id, newStatus, actor.PlayerID, note); err != nil {
		return err
	}
	if err := logEvent(ctx, tx, faction, actor, playerID, lvl, "division", detail, note); err != nil {
		return err
	}
	title, body := d.Name+" application accepted", "You've joined "+d.Name+" as "+d.EntryRole()+"."
	if !accept {
		title, body = d.Name+" application not accepted", note
	}
	if err := notify.Send(ctx, tx, playerID, title, body, "/factions?faction="+faction+"#divisions"); err != nil {
		return err
	}
	_ = actorLevel
	return tx.Commit(ctx)
}
