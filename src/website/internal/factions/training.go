package factions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Probation and training sheets (layout plan "Recruits & training"). A
// probation starts when someone is recruited; command signs off each
// training item, then confirms the recruit (promoted one rank) or ends the
// probation (removed from the faction).

type TrainingItem struct {
	Key, Name   string
	RetakeHours int
}

func TrainingItems(ctx context.Context, q dbtx, faction string) ([]TrainingItem, error) {
	rows, err := q.Query(ctx, `SELECT key, name, retake_hours FROM faction_training_items WHERE faction = $1 ORDER BY sort, key`, faction)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrainingItem
	for rows.Next() {
		var t TrainingItem
		if err := rows.Scan(&t.Key, &t.Name, &t.RetakeHours); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Result is the latest sign-off on one training item.
type Result struct {
	Item     TrainingItem
	Result   string // pass / fail / none
	By       string
	At       time.Time
	RetakeAt time.Time // when a failed item can next be passed (zero = now)
}

func (r Result) CanRetake() bool { return r.RetakeAt.IsZero() || time.Now().After(r.RetakeAt) }

// Probation is one recruit's probation.
type Probation struct {
	ID        int64
	PlayerID  int64
	Name      string
	Level     int
	FTOID     int64
	FTO       string
	Started   time.Time
	Ends      time.Time
	Status    string
	Note      string
	DecidedBy string
	DecidedAt *time.Time
	Results   []Result
	Passed    int
}

// Day is which day of probation they're on (1-based), capped at Days.
func (p Probation) Day() int {
	d := int(time.Since(p.Started).Hours()/24) + 1
	return min(d, p.Days())
}

// Days is the probation's length.
func (p Probation) Days() int { return max(1, int(p.Ends.Sub(p.Started).Hours()/24+0.5)) }

// DayPercent is how far through it is, for the progress bar.
func (p Probation) DayPercent() int { return p.Day() * 100 / p.Days() }

// Ending is true in the last two days.
func (p Probation) Ending() bool { return p.Days()-p.Day() < 2 }

func (p Probation) AllPassed() bool { return len(p.Results) > 0 && p.Passed == len(p.Results) }

// Remaining is how many items are still to pass.
func (p Probation) Remaining() int { return len(p.Results) - p.Passed }

// Probations lists the faction's probations: active ones, or recent
// finished ones when active is false.
func Probations(ctx context.Context, q dbtx, faction string, active bool) ([]Probation, error) {
	rows, err := q.Query(ctx, `
		SELECT fp.id, fp.player_id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id), COALESCE(p.`+column[faction]+`, 0),
		       COALESCE(fp.fto_id, 0), COALESCE(NULLIF(f.name, ''), NULLIF(f.steam_name, ''), ''),
		       fp.started_at, fp.ends_at, fp.status, fp.note,
		       COALESCE(NULLIF(d.name, ''), NULLIF(d.steam_name, ''), ''), fp.decided_at
		FROM faction_probations fp
		JOIN players p ON p.id = fp.player_id
		LEFT JOIN players f ON f.id = fp.fto_id
		LEFT JOIN players d ON d.id = fp.decided_by
		WHERE fp.faction = $1 AND (fp.status = 'active') = $2
		ORDER BY CASE WHEN $2 THEN fp.started_at END, fp.decided_at DESC NULLS LAST
		LIMIT 100`, faction, active)
	if err != nil {
		return nil, err
	}
	var out []Probation
	for rows.Next() {
		var p Probation
		if err := rows.Scan(&p.ID, &p.PlayerID, &p.Name, &p.Level, &p.FTOID, &p.FTO, &p.Started, &p.Ends, &p.Status, &p.Note, &p.DecidedBy, &p.DecidedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items, err := TrainingItems(ctx, q, faction)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if err := loadResults(ctx, q, &out[i], items); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// GetProbation loads one probation in faction.
func GetProbation(ctx context.Context, q dbtx, faction string, id int64) (Probation, error) {
	ps, err := Probations(ctx, q, faction, true)
	if err != nil {
		return Probation{}, err
	}
	for _, p := range ps {
		if p.ID == id {
			return p, nil
		}
	}
	done, err := Probations(ctx, q, faction, false)
	if err != nil {
		return Probation{}, err
	}
	for _, p := range done {
		if p.ID == id {
			return p, nil
		}
	}
	return Probation{}, notAllowed("that probation doesn't exist")
}

func loadResults(ctx context.Context, q dbtx, p *Probation, items []TrainingItem) error {
	rows, err := q.Query(ctx, `
		SELECT DISTINCT ON (r.item_key) r.item_key, r.result, COALESCE(NULLIF(b.name, ''), NULLIF(b.steam_name, ''), ''), r.created_at
		FROM faction_training_results r LEFT JOIN players b ON b.id = r.by_id
		WHERE r.probation_id = $1 ORDER BY r.item_key, r.id DESC`, p.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	latest := map[string]Result{}
	for rows.Next() {
		var k string
		var r Result
		if err := rows.Scan(&k, &r.Result, &r.By, &r.At); err != nil {
			return err
		}
		latest[k] = r
	}
	if err := rows.Err(); err != nil {
		return err
	}
	p.Results, p.Passed = nil, 0
	for _, it := range items {
		r, ok := latest[it.Key]
		if !ok {
			r = Result{Result: "none"}
		}
		r.Item = it
		if r.Result == "fail" && it.RetakeHours > 0 {
			r.RetakeAt = r.At.Add(time.Duration(it.RetakeHours) * time.Hour)
		}
		if r.Result == "pass" {
			p.Passed++
		}
		p.Results = append(p.Results, r)
	}
	return nil
}

// lockProbation loads an active probation for a command action on it.
func lockProbation(ctx context.Context, tx pgx.Tx, actor Actor, faction string, id int64) (Probation, authority, error) {
	var p Probation
	err := tx.QueryRow(ctx, `SELECT player_id, status FROM faction_probations WHERE id = $1 AND faction = $2 FOR UPDATE`, id, faction).Scan(&p.PlayerID, &p.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, authority{}, notAllowed("that probation doesn't exist")
	}
	if err != nil {
		return p, authority{}, err
	}
	if p.Status != "active" {
		return p, authority{}, notAllowed("that probation has already finished")
	}
	a, err := recordsOver(ctx, tx, actor, faction, p.PlayerID)
	if err != nil {
		return p, a, err
	}
	p.ID, p.Name, p.Level = id, playerName(ctx, tx, p.PlayerID), a.TargetLvl
	return p, a, nil
}

// SetTraining records a sign-off on one training item.
func SetTraining(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, probationID int64, itemKey, result string) error {
	if result != "pass" && result != "fail" && result != "none" {
		return notAllowed("unknown result")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	p, a, err := lockProbation(ctx, tx, actor, faction, probationID)
	if err != nil {
		return err
	}
	items, err := TrainingItems(ctx, tx, faction)
	if err != nil {
		return err
	}
	if err := loadResults(ctx, tx, &p, items); err != nil {
		return err
	}
	var cur Result
	for _, r := range p.Results {
		if r.Item.Key == itemKey {
			cur = r
		}
	}
	if cur.Item.Key == "" {
		return notAllowed("unknown training item")
	}
	if cur.Result == result {
		return notAllowed("it's already marked that way")
	}
	if result == "pass" && cur.Result == "fail" && !cur.CanRetake() {
		return notAllowed("%s can be retaken after %d hours, from %s", cur.Item.Name, cur.Item.RetakeHours, cur.RetakeAt.Local().Format("15:04 on 2 Jan"))
	}
	if _, err := tx.Exec(ctx, `INSERT INTO faction_training_results (probation_id, item_key, result, by_id) VALUES ($1, $2, $3, $4)`,
		probationID, itemKey, result, actor.PlayerID); err != nil {
		return err
	}
	word := map[string]string{"pass": "passed", "fail": "needs work on", "none": "reset"}[result]
	detail := fmt.Sprintf("%s %s %s", p.Name, word, cur.Item.Name)
	if result == "none" {
		detail = fmt.Sprintf("%s: %s reset to not done", p.Name, cur.Item.Name)
	}
	if err := logEvent(ctx, tx, faction, actor, p.PlayerID, a.TargetLvl, "training", detail, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UpdateProbation sets the FTO and note.
func UpdateProbation(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, probationID, ftoID int64, note string) error {
	var err error
	if note, err = cleanText(note, 500, "the note", false); err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	p, a, err := lockProbation(ctx, tx, actor, faction, probationID)
	if err != nil {
		return err
	}
	var oldFTO int64
	var oldNote string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(fto_id, 0), note FROM faction_probations WHERE id = $1`, probationID).Scan(&oldFTO, &oldNote); err != nil {
		return err
	}
	if ftoID > 0 {
		if ftoID == p.PlayerID {
			return notAllowed("a recruit can't be their own FTO")
		}
		l, err := levelOf(ctx, tx, faction, ftoID)
		if err != nil {
			return err
		}
		if l <= p.Level {
			return notAllowed("the FTO must be a %s member ranked above the recruit", Name(faction))
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE faction_probations SET fto_id = NULLIF($2, 0), note = $3 WHERE id = $1`, probationID, ftoID, note); err != nil {
		return err
	}
	if oldFTO != ftoID {
		detail := p.Name + ": FTO removed"
		if ftoID > 0 {
			detail = p.Name + ": FTO is now " + playerName(ctx, tx, ftoID)
		}
		if err := logEvent(ctx, tx, faction, actor, p.PlayerID, a.TargetLvl, "probation", detail, ""); err != nil {
			return err
		}
	} else if oldNote == note {
		return notAllowed("nothing changed")
	}
	return tx.Commit(ctx)
}

// ConfirmProbation promotes a recruit who has passed every item one rank.
func ConfirmProbation(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, probationID int64) (Change, error) {
	p, err := GetProbation(ctx, pool, faction, probationID)
	if err != nil {
		return Change{}, err
	}
	if p.Status != "active" {
		return Change{}, notAllowed("that probation has already finished")
	}
	if !p.AllPassed() {
		return Change{}, notAllowed("%d training item%s still to pass", len(p.Results)-p.Passed, map[bool]string{true: "s"}[len(p.Results)-p.Passed != 1])
	}
	ch, err := setLevel(ctx, pool, actor, faction, p.PlayerID, p.Level+1, "Passed probation", true)
	if err != nil {
		return ch, err
	}
	_, err = pool.Exec(ctx, `UPDATE faction_probations SET status = 'confirmed', decided_by = $2, decided_at = now() WHERE id = $1 AND status = 'active'`,
		probationID, actor.PlayerID)
	return ch, err
}

// EndProbation removes a recruit who isn't suitable, with a discharge record.
func EndProbation(ctx context.Context, pool *pgxpool.Pool, actor Actor, faction string, probationID int64, reason string) error {
	var err error
	if reason, err = cleanText(reason, 300, "a reason", true); err != nil {
		return err
	}
	p, err := GetProbation(ctx, pool, faction, probationID)
	if err != nil {
		return err
	}
	if p.Status != "active" {
		return notAllowed("that probation has already finished")
	}
	if _, err := setLevel(ctx, pool, actor, faction, p.PlayerID, 0, "Probation ended: "+reason, false); err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		WITH upd AS (UPDATE faction_probations SET status = 'ended', decided_by = $2, decided_at = now(), note = $3 WHERE id = $1)
		INSERT INTO faction_discharges (faction, player_id, last_level, type, notes, by_id) VALUES ($4, $5, $6, 'probation_ended', $3, $2)`,
		probationID, actor.PlayerID, reason, faction, p.PlayerID, p.Level)
	return err
}
