// Package applications is recruitment (docs/GAMEPANEL_PARITY.md §4, §6.2):
// staff applications with interviews, reviewed by staff with
// applications.view / applications.decide, and faction applications,
// reviewed by that faction's command.
//
// Applicants are Steam-verified players (they apply while signed in). One
// open application at a time per player (per faction for faction
// applications), and a cooldown after a rejection. Accepting a staff
// application goes through internal/staff and a faction application through
// internal/factions, so the usual rules, logs and role sync all apply.
// Decisions are DMed through the Discord outbox when the player has linked.
package applications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
	"website/internal/discord"
	"website/internal/factions"
	"website/internal/notify"
	"website/internal/staff"
)

// ErrNotAllowed wraps a rule violation with a human-readable reason.
var ErrNotAllowed = errors.New("not allowed")

func notAllowed(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrNotAllowed, fmt.Sprintf(format, args...))
}

// Cooldowns after a rejection before the player can apply again.
const (
	StaffCooldown   = 14 * 24 * time.Hour
	FactionCooldown = 7 * 24 * time.Hour
)

// Question is one application question.
type Question struct {
	Key      string
	Label    string
	Hint     string
	Long     bool // textarea
	Optional bool
	Max      int
}

// StaffQuestions are the staff application form (layout plan "Join the staff team").
var StaffQuestions = []Question{
	{Key: "age", Label: "Age", Max: 3},
	{Key: "tz", Label: "Timezone and usual hours", Hint: "e.g. AEST, weeknights 7–11pm", Max: 100},
	{Key: "about", Label: "About you", Hint: "Who you are and how you play here.", Long: true, Max: 1000},
	{Key: "why", Label: "Why you?", Hint: "What would you bring to the staff team?", Long: true, Max: 1000},
	{Key: "exp", Label: "Experience", Hint: `Moderation, support or community roles, on Arma or elsewhere. "None" is a fine answer.`, Long: true, Max: 1000},
}

// InterviewQuestions are asked in the Discord interview (Gamepanel's set).
var InterviewQuestions = []Question{
	{Key: "experience", Label: "Prior experience", Max: 500},
	{Key: "banned", Label: "Ever banned (why)", Max: 500},
	{Key: "hours", Label: "Hours per week", Max: 100},
	{Key: "away", Label: "Planned time away", Max: 300},
	{Key: "flex", Label: "Flexibility", Max: 300},
}

// FactionQuestions returns the faction application form.
func FactionQuestions(faction string) []Question {
	return []Question{
		{Key: "why", Label: "Why " + factions.Name(faction) + "?", Hint: "What draws you to it, in a few sentences.", Long: true, Max: 1000},
		{Key: "hours", Label: "When will you be on?", Hint: "Timezone and usual hours.", Max: 200},
		{Key: "extra", Label: "Anything command should know? (optional)", Hint: "Experience in other communities, a mate who'll join with you, etc.", Long: true, Optional: true, Max: 1000},
	}
}

// Answer is a question and its answer, for display.
type Answer struct{ Q, A string }

func collect(qs []Question, form map[string]string) (map[string]string, error) {
	out := map[string]string{}
	for _, q := range qs {
		v := strings.TrimSpace(form[q.Key])
		if v == "" && !q.Optional {
			return nil, notAllowed("answer %q", q.Label)
		}
		if len(v) > q.Max {
			return nil, notAllowed("%q is too long (%d characters max)", q.Label, q.Max)
		}
		out[q.Key] = v
	}
	return out, nil
}

func answersFor(qs []Question, raw []byte) []Answer {
	var m map[string]string
	_ = json.Unmarshal(raw, &m)
	out := make([]Answer, 0, len(qs))
	for _, q := range qs {
		if v := m[q.Key]; v != "" || !q.Optional {
			out = append(out, Answer{q.Label, v})
		}
	}
	return out
}

// Actor is the reviewer.
type Actor struct {
	PlayerID int64
	Source   string
}

const nameExpr = `COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id)`

// dm queues a Discord DM for the player, if they've linked Discord.
func dm(ctx context.Context, tx pgx.Tx, playerID int64, key, content string) error {
	var discordID *string
	if err := tx.QueryRow(ctx, `SELECT discord_id FROM players WHERE id = $1`, playerID).Scan(&discordID); err != nil {
		return err
	}
	if discordID == nil || *discordID == "" {
		return nil
	}
	return discord.Enqueue(ctx, tx, discord.OutboxMessage{Kind: discord.KindDM, Target: *discordID, Content: content, DedupeKey: key})
}

// ---- Staff applications ----

// StaffApp is one staff application.
type StaffApp struct {
	ID         int64
	PlayerID   int64
	Name       string
	Status     string
	Answers    []Answer
	TZ         string
	Discord    string
	ReviewedBy string
	ReviewNote string
	CreatedAt  time.Time
	DecidedAt  *time.Time
	PlaytimeH  int64
	Points     int
	PastBans   int
	Interview  *Interview
}

// StatusLabel is the status for display.
func StatusLabel(s string) string {
	return map[string]string{"pending": "Under review", "interview": "Interview", "accepted": "Accepted",
		"rejected": "Not accepted", "withdrawn": "Withdrawn"}[s]
}

// StatusText is the status for display.
func (a StaffApp) StatusText() string { return StatusLabel(a.Status) }

// Interview is the interview record for an application.
type Interview struct {
	Interviewer string
	Answers     map[string]string
	Outcome     string
}

// Eligibility says whether a player can apply for staff right now.
type Eligibility struct {
	CanApply    bool
	Reason      string // why not
	DiscordName string
	PlaytimeH   int64
	Open        *StaffApp // their open application, if any
	Latest      *StaffApp // their most recent application
}

// StaffEligibility checks the "Before you apply" list: signed in with
// Steam (always), Discord linked, not already staff, not banned, no open
// application, not in a cooldown.
func StaffEligibility(ctx context.Context, pool *pgxpool.Pool, playerID int64) (Eligibility, error) {
	var e Eligibility
	var discordName *string
	var staffRank *int
	var banned bool
	err := pool.QueryRow(ctx, `
		SELECT p.discord_username, p.staff_rank_id,
		       (COALESCE(p.civ_playtime_seconds, 0) + COALESCE(p.cop_playtime_seconds, 0) + COALESCE(p.medic_playtime_seconds, 0)) / 3600,
		       EXISTS (SELECT 1 FROM banlist b WHERE b.uid = p.uid AND (b.expires_at IS NULL OR b.expires_at > now()))
		FROM players p WHERE p.id = $1`, playerID).Scan(&discordName, &staffRank, &e.PlaytimeH, &banned)
	if err != nil {
		return e, err
	}
	if discordName != nil {
		e.DiscordName = *discordName
	}
	if latest, err := latestStaffApp(ctx, pool, playerID); err != nil {
		return e, err
	} else if latest != nil {
		e.Latest = latest
		if latest.Status == "pending" || latest.Status == "interview" {
			e.Open = latest
		}
	}
	switch {
	case staffRank != nil:
		e.Reason = "You're already on the staff team."
	case banned:
		e.Reason = "You can't apply while banned."
	case e.DiscordName == "":
		e.Reason = "Link your Discord first (Dashboard → Connect Discord) so we can reach you for an interview."
	case e.Open != nil:
		e.Reason = "You already have an application in progress."
	case e.Latest != nil && e.Latest.Status == "rejected" && e.Latest.DecidedAt != nil && time.Since(*e.Latest.DecidedAt) < StaffCooldown:
		e.Reason = "You can apply again from " + e.Latest.DecidedAt.Add(StaffCooldown).Format("2 Jan 2006") + "."
	default:
		e.CanApply = true
	}
	return e, nil
}

func latestStaffApp(ctx context.Context, pool *pgxpool.Pool, playerID int64) (*StaffApp, error) {
	var id int64
	err := pool.QueryRow(ctx, `SELECT id FROM staff_applications WHERE player_id = $1 ORDER BY id DESC LIMIT 1`, playerID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a, err := GetStaffApp(ctx, pool, id)
	return &a, err
}

// SubmitStaff files a staff application.
func SubmitStaff(ctx context.Context, pool *pgxpool.Pool, playerID int64, form map[string]string, agreed bool) (int64, error) {
	e, err := StaffEligibility(ctx, pool, playerID)
	if err != nil {
		return 0, err
	}
	if !e.CanApply {
		return 0, notAllowed("%s", strings.TrimSuffix(e.Reason, "."))
	}
	answers, err := collect(StaffQuestions, form)
	if err != nil {
		return 0, err
	}
	if age, err := strconv.Atoi(answers["age"]); err != nil || age < 13 || age > 99 {
		return 0, notAllowed("enter your age as a number")
	}
	if !agreed {
		return 0, notAllowed("tick the box to confirm you've read the server rules")
	}
	raw, _ := json.Marshal(answers)
	var id int64
	err = pool.QueryRow(ctx, `INSERT INTO staff_applications (player_id, answers) VALUES ($1, $2) RETURNING id`, playerID, raw).Scan(&id)
	if err != nil && strings.Contains(err.Error(), "idx_staff_applications_open") {
		return 0, notAllowed("you already have an application in progress")
	}
	return id, err
}

// WithdrawStaff lets the applicant withdraw their open application.
func WithdrawStaff(ctx context.Context, pool *pgxpool.Pool, playerID int64) error {
	tag, err := pool.Exec(ctx, `
		UPDATE staff_applications SET status = 'withdrawn', decided_at = now()
		WHERE player_id = $1 AND status IN ('pending', 'interview')`, playerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return notAllowed("you don't have an application in progress")
	}
	return nil
}

// GetStaffApp loads one application with the applicant's record.
func GetStaffApp(ctx context.Context, pool *pgxpool.Pool, id int64) (StaffApp, error) {
	var a StaffApp
	var raw []byte
	var discordName *string
	err := pool.QueryRow(ctx, `
		SELECT a.id, a.player_id, `+nameExpr+`, a.status, a.answers, p.discord_username,
		       COALESCE(NULLIF(r.name, ''), NULLIF(r.steam_name, ''), ''), COALESCE(a.review_note, ''), a.created_at, a.decided_at,
		       (COALESCE(p.civ_playtime_seconds, 0) + COALESCE(p.cop_playtime_seconds, 0) + COALESCE(p.medic_playtime_seconds, 0)) / 3600,
		       COALESCE((SELECT sum(points) FROM punishment_points pp WHERE pp.player_id = p.id AND pp.revoked_at IS NULL
		                 AND (pp.expires_at IS NULL OR pp.expires_at > now())), 0),
		       (SELECT count(*) FROM banlist b WHERE b.uid = p.uid)
		FROM staff_applications a JOIN players p ON p.id = a.player_id LEFT JOIN players r ON r.id = a.reviewed_by
		WHERE a.id = $1`, id).Scan(&a.ID, &a.PlayerID, &a.Name, &a.Status, &raw, &discordName, &a.ReviewedBy, &a.ReviewNote,
		&a.CreatedAt, &a.DecidedAt, &a.PlaytimeH, &a.Points, &a.PastBans)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, notAllowed("that application doesn't exist")
	}
	if err != nil {
		return a, err
	}
	if discordName != nil {
		a.Discord = *discordName
	}
	a.Answers = answersFor(StaffQuestions, raw)
	var m map[string]string
	_ = json.Unmarshal(raw, &m)
	a.TZ = m["tz"]
	var iv Interview
	var ivRaw []byte
	var outcome *string
	err = pool.QueryRow(ctx, `
		SELECT COALESCE(NULLIF(i.name, ''), NULLIF(i.steam_name, ''), ''), iv.answers, iv.outcome
		FROM staff_interviews iv LEFT JOIN players i ON i.id = iv.interviewer_id WHERE iv.application_id = $1`, id).
		Scan(&iv.Interviewer, &ivRaw, &outcome)
	if err == nil {
		_ = json.Unmarshal(ivRaw, &iv.Answers)
		if outcome != nil {
			iv.Outcome = *outcome
		}
		a.Interview = &iv
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return a, err
	}
	return a, nil
}

// ListStaff lists applications with the given status ("" = all), oldest
// open first.
func ListStaff(ctx context.Context, pool *pgxpool.Pool, status string) ([]StaffApp, error) {
	rows, err := pool.Query(ctx, `
		SELECT a.id FROM staff_applications a
		WHERE ($1 = '' OR a.status = $1)
		ORDER BY CASE WHEN a.status IN ('pending', 'interview') THEN 0 ELSE 1 END, a.id DESC LIMIT 200`, status)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	out := make([]StaffApp, 0, len(ids))
	for _, id := range ids {
		a, err := GetStaffApp(ctx, pool, id)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// StaffCounts is the number of applications per status.
func StaffCounts(ctx context.Context, pool *pgxpool.Pool) (map[string]int, error) {
	rows, err := pool.Query(ctx, `SELECT status, count(*) FROM staff_applications GROUP BY 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		out[s] = n
		out[""] += n
	}
	return out, rows.Err()
}

func lockOpenStaff(ctx context.Context, tx pgx.Tx, id int64) (playerID int64, status string, err error) {
	err = tx.QueryRow(ctx, `SELECT player_id, status FROM staff_applications WHERE id = $1 FOR UPDATE`, id).Scan(&playerID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", notAllowed("that application doesn't exist")
	}
	if err == nil && status != "pending" && status != "interview" {
		err = notAllowed("that application was already %s", strings.ToLower(StatusLabel(status)))
	}
	return
}

// ToInterview moves an application to the interview stage.
func ToInterview(ctx context.Context, pool *pgxpool.Pool, actor Actor, id int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	playerID, status, err := lockOpenStaff(ctx, tx, id)
	if err != nil {
		return err
	}
	if status == "interview" {
		return notAllowed("it's already at the interview stage")
	}
	if playerID == actor.PlayerID {
		return notAllowed("you can't review your own application")
	}
	if _, err := tx.Exec(ctx, `UPDATE staff_applications SET status = 'interview', reviewed_by = $2 WHERE id = $1`, id, actor.PlayerID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO staff_interviews (application_id, interviewer_id) VALUES ($1, $2) ON CONFLICT (application_id) DO NOTHING`, id, actor.PlayerID); err != nil {
		return err
	}
	if err := notify.Send(ctx, tx, playerID, "Staff application: interview", "A staff member will contact you on Discord to arrange a time.", "/staff/apply"); err != nil {
		return err
	}
	if err := dm(ctx, tx, playerID, fmt.Sprintf("staff-app:%d:interview", id),
		"Your TasDyn-ALife staff application has moved to **interview**. A staff member will contact you on Discord to arrange a time."); err != nil {
		return err
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{StaffID: actor.PlayerID, TargetID: playerID, Action: "application.interview",
		Source: actor.Source, Reason: fmt.Sprintf("Staff application #%d moved to interview", id)}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SaveInterview records interview answers and (optionally) the outcome.
func SaveInterview(ctx context.Context, pool *pgxpool.Pool, actor Actor, id int64, form map[string]string, outcome string) error {
	if outcome != "" && outcome != "pass" && outcome != "fail" {
		return notAllowed("unknown outcome")
	}
	answers := map[string]string{}
	for _, q := range InterviewQuestions {
		v := strings.TrimSpace(form[q.Key])
		if len(v) > q.Max {
			return notAllowed("%q is too long (%d characters max)", q.Label, q.Max)
		}
		answers[q.Key] = v
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, status, err := lockOpenStaff(ctx, tx, id); err != nil {
		return err
	} else if status != "interview" {
		return notAllowed("move it to interview first")
	}
	raw, _ := json.Marshal(answers)
	var out *string
	if outcome != "" {
		out = &outcome
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO staff_interviews (application_id, interviewer_id, answers, outcome) VALUES ($1, $2, $3, $4)
		ON CONFLICT (application_id) DO UPDATE SET interviewer_id = EXCLUDED.interviewer_id, answers = EXCLUDED.answers,
		       outcome = EXCLUDED.outcome, updated_at = now()`, id, actor.PlayerID, raw, out); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// AcceptStaff accepts an application: the applicant joins staff at rankID
// (through internal/staff, so its seniority rules apply) in team.
func AcceptStaff(ctx context.Context, pool *pgxpool.Pool, actor Actor, id int64, rankID int, team, note string) error {
	a, err := GetStaffApp(ctx, pool, id)
	if err != nil {
		return err
	}
	if a.Status != "pending" && a.Status != "interview" {
		return notAllowed("that application was already %s", strings.ToLower(StatusLabel(a.Status)))
	}
	if a.PlayerID == actor.PlayerID {
		return notAllowed("you can't review your own application")
	}
	sa := staff.Actor{PlayerID: actor.PlayerID, Source: actor.Source}
	if err := staff.SetRank(ctx, pool, sa, a.PlayerID, rankID, fmt.Sprintf("Staff application #%d accepted", id)); err != nil {
		if errors.Is(err, staff.ErrNotAllowed) {
			return notAllowed("%s", strings.TrimPrefix(err.Error(), staff.ErrNotAllowed.Error()+": "))
		}
		return err
	}
	if team = strings.TrimSpace(team); team != "" {
		if err := staff.SetTeam(ctx, pool, sa, a.PlayerID, team, ""); err != nil {
			return err
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE staff_applications SET status = 'accepted', reviewed_by = $2, review_note = NULLIF($3, ''), decided_at = now()
		WHERE id = $1`, id, actor.PlayerID, strings.TrimSpace(note)); err != nil {
		return err
	}
	if err := notify.Send(ctx, tx, a.PlayerID, "Staff application accepted", "Welcome to the staff team. The Admin Panel link is in the header.", "/admin"); err != nil {
		return err
	}
	if err := dm(ctx, tx, a.PlayerID, fmt.Sprintf("staff-app:%d:decision", id),
		"Congratulations! Your TasDyn-ALife **staff application was accepted**. Sign in to the website to find the Admin Panel, and check the staff channels in Discord."); err != nil {
		return err
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{StaffID: actor.PlayerID, TargetID: a.PlayerID, Action: "application.accept",
		Source: actor.Source, Reason: fmt.Sprintf("Staff application #%d accepted", id)}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RejectStaff rejects an application. note (required) is for reviewers only.
func RejectStaff(ctx context.Context, pool *pgxpool.Pool, actor Actor, id int64, note string) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return notAllowed("add a review note saying why")
	}
	if len(note) > 1000 {
		return notAllowed("the note is too long (1000 characters max)")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	playerID, _, err := lockOpenStaff(ctx, tx, id)
	if err != nil {
		return err
	}
	if playerID == actor.PlayerID {
		return notAllowed("you can't review your own application")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE staff_applications SET status = 'rejected', reviewed_by = $2, review_note = $3, decided_at = now() WHERE id = $1`,
		id, actor.PlayerID, note); err != nil {
		return err
	}
	if err := notify.Send(ctx, tx, playerID, "Staff application not accepted",
		"You can apply again from "+time.Now().Add(StaffCooldown).Format("2 Jan 2006")+".", "/staff/apply"); err != nil {
		return err
	}
	if err := dm(ctx, tx, playerID, fmt.Sprintf("staff-app:%d:decision", id),
		fmt.Sprintf("Thanks for applying to the TasDyn-ALife staff team. This time your application **wasn't accepted**. You're welcome to apply again from %s.",
			time.Now().Add(StaffCooldown).Format("2 Jan 2006"))); err != nil {
		return err
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{StaffID: actor.PlayerID, TargetID: playerID, Action: "application.reject",
		Source: actor.Source, Reason: fmt.Sprintf("Staff application #%d: %s", id, note)}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ---- Faction applications ----

// FactionApp is one faction application.
type FactionApp struct {
	ID        int64
	PlayerID  int64
	Name      string
	Faction   string
	Status    string
	Answers   []Answer
	Note      string
	DecidedBy string
	CreatedAt time.Time
	DecidedAt *time.Time
	PlaytimeH int64
	Points    int
	Banned    bool
}

const factionAppSelect = `
	SELECT a.id, a.player_id, ` + nameExpr + `, a.faction, a.status, a.answers, COALESCE(a.decision_note, ''),
	       COALESCE(NULLIF(d.name, ''), NULLIF(d.steam_name, ''), ''), a.created_at, a.decided_at,
	       (COALESCE(p.civ_playtime_seconds, 0) + COALESCE(p.cop_playtime_seconds, 0) + COALESCE(p.medic_playtime_seconds, 0)) / 3600,
	       COALESCE((SELECT sum(points) FROM punishment_points pp WHERE pp.player_id = p.id AND pp.revoked_at IS NULL
	                 AND (pp.expires_at IS NULL OR pp.expires_at > now())), 0),
	       EXISTS (SELECT 1 FROM banlist b WHERE b.uid = p.uid AND (b.expires_at IS NULL OR b.expires_at > now()))
	FROM faction_applications a JOIN players p ON p.id = a.player_id LEFT JOIN players d ON d.id = a.decided_by`

func scanFactionApps(rows pgx.Rows) ([]FactionApp, error) {
	defer rows.Close()
	var out []FactionApp
	for rows.Next() {
		var a FactionApp
		var raw []byte
		if err := rows.Scan(&a.ID, &a.PlayerID, &a.Name, &a.Faction, &a.Status, &raw, &a.Note, &a.DecidedBy,
			&a.CreatedAt, &a.DecidedAt, &a.PlaytimeH, &a.Points, &a.Banned); err != nil {
			return nil, err
		}
		a.Answers = answersFor(FactionQuestions(a.Faction), raw)
		out = append(out, a)
	}
	return out, rows.Err()
}

// PlayerFactionApps lists a player's faction applications, newest first.
func PlayerFactionApps(ctx context.Context, pool *pgxpool.Pool, playerID int64) ([]FactionApp, error) {
	rows, err := pool.Query(ctx, factionAppSelect+` WHERE a.player_id = $1 ORDER BY a.id DESC`, playerID)
	if err != nil {
		return nil, err
	}
	return scanFactionApps(rows)
}

// FactionQueue lists a faction's applications: open ones first (oldest
// first), then the latest decided ones.
func FactionQueue(ctx context.Context, pool *pgxpool.Pool, faction string) ([]FactionApp, error) {
	rows, err := pool.Query(ctx, factionAppSelect+`
		WHERE a.faction = $1 AND (a.status = 'pending' OR a.decided_at > now() - interval '30 days')
		ORDER BY a.status <> 'pending', CASE WHEN a.status = 'pending' THEN a.id ELSE -a.id END LIMIT 100`, faction)
	if err != nil {
		return nil, err
	}
	return scanFactionApps(rows)
}

// OpenFactionCount counts pending applications for a faction.
func OpenFactionCount(ctx context.Context, pool *pgxpool.Pool, faction string) int {
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM faction_applications WHERE faction = $1 AND status = 'pending'`, faction).Scan(&n)
	return n
}

// SubmitFaction files a faction application.
func SubmitFaction(ctx context.Context, pool *pgxpool.Pool, playerID int64, faction string, form map[string]string) error {
	if !factions.Valid(faction) {
		return notAllowed("pick Police or EMS")
	}
	answers, err := collect(FactionQuestions(faction), form)
	if err != nil {
		return err
	}
	col := map[string]string{"police": "cop_level", "ems": "medic_level"}[faction]
	var level int
	var banned bool
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(`+col+`, 0), EXISTS (SELECT 1 FROM banlist b WHERE b.uid = p.uid AND (b.expires_at IS NULL OR b.expires_at > now()))
		FROM players p WHERE p.id = $1`, playerID).Scan(&level, &banned); err != nil {
		return err
	}
	if level > 0 {
		return notAllowed("you're already in %s", factions.Name(faction))
	}
	if banned {
		return notAllowed("you can't apply while banned")
	}
	if bl, err := factions.ActiveBlacklist(ctx, pool, faction, playerID); err != nil {
		return err
	} else if bl != nil {
		return notAllowed("you're blacklisted from %s %s", factions.Name(faction), bl.UntilText())
	}
	var lastRejected *time.Time
	_ = pool.QueryRow(ctx, `
		SELECT max(decided_at) FROM faction_applications WHERE player_id = $1 AND faction = $2 AND status = 'rejected'`,
		playerID, faction).Scan(&lastRejected)
	if lastRejected != nil && time.Since(*lastRejected) < FactionCooldown {
		return notAllowed("you can apply to %s again from %s", factions.Name(faction), lastRejected.Add(FactionCooldown).Format("2 Jan 2006"))
	}
	raw, _ := json.Marshal(answers)
	_, err = pool.Exec(ctx, `INSERT INTO faction_applications (player_id, faction, answers) VALUES ($1, $2, $3)`, playerID, faction, raw)
	if err != nil && strings.Contains(err.Error(), "idx_faction_applications_open") {
		return notAllowed("you already have an open application to %s", factions.Name(faction))
	}
	return err
}

// WithdrawFaction withdraws the player's open application to faction.
func WithdrawFaction(ctx context.Context, pool *pgxpool.Pool, playerID int64, faction string) error {
	tag, err := pool.Exec(ctx, `
		UPDATE faction_applications SET status = 'withdrawn', decided_at = now()
		WHERE player_id = $1 AND faction = $2 AND status = 'pending'`, playerID, faction)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return notAllowed("you don't have an open application there")
	}
	return nil
}

// DecideFaction accepts (recruiting at level 1 through factions.SetLevel,
// so command authority, slots and the Command log all apply) or rejects a
// faction application. Only that faction's command can decide.
func DecideFaction(ctx context.Context, pool *pgxpool.Pool, actor Actor, id int64, accept bool, note string) error {
	note = strings.TrimSpace(note)
	if !accept && note == "" {
		return notAllowed("add a short reason")
	}
	if len(note) > 1000 {
		return notAllowed("the note is too long (1000 characters max)")
	}
	var playerID int64
	var faction, status string
	err := pool.QueryRow(ctx, `SELECT player_id, faction, status FROM faction_applications WHERE id = $1`, id).Scan(&playerID, &faction, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return notAllowed("that application doesn't exist")
	}
	if err != nil {
		return err
	}
	if status != "pending" {
		return notAllowed("that application was already decided")
	}
	if _, isCmd, err := factions.CommandIn(ctx, pool, actor.PlayerID, faction); err != nil {
		return err
	} else if !isCmd {
		return notAllowed("only %s command can decide %s applications", factions.Name(faction), factions.Name(faction))
	}
	if accept {
		reason := fmt.Sprintf("Faction application #%d accepted", id)
		if note != "" {
			reason += ": " + note
		}
		if _, err := factions.SetLevel(ctx, pool, factions.Actor{PlayerID: actor.PlayerID, Source: actor.Source, Via: factions.ViaCommand},
			faction, playerID, 1, reason); err != nil {
			if errors.Is(err, factions.ErrNotAllowed) {
				return notAllowed("%s", strings.TrimPrefix(err.Error(), factions.ErrNotAllowed.Error()+": "))
			}
			return err
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	newStatus := map[bool]string{true: "accepted", false: "rejected"}[accept]
	tag, err := tx.Exec(ctx, `
		UPDATE faction_applications SET status = $2, decided_by = $3, decision_note = NULLIF($4, ''), decided_at = now()
		WHERE id = $1 AND status = 'pending'`, id, newStatus, actor.PlayerID, note)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return notAllowed("that application was already decided")
	}
	msg := fmt.Sprintf("Your application to **%s** on TasDyn-ALife was accepted. Welcome aboard! Your new rank applies the next time you load in.", factions.Name(faction))
	if !accept {
		msg = fmt.Sprintf("Your application to **%s** on TasDyn-ALife wasn't accepted this time: %s. You can apply again from %s.",
			factions.Name(faction), note, time.Now().Add(FactionCooldown).Format("2 Jan 2006"))
	}
	title := factions.Name(faction) + " application accepted"
	if !accept {
		title = factions.Name(faction) + " application not accepted"
	}
	if err := notify.Send(ctx, tx, playerID, title, note, "/factions?faction="+faction); err != nil {
		return err
	}
	if err := dm(ctx, tx, playerID, fmt.Sprintf("faction-app:%d:decision", id), msg); err != nil {
		return err
	}
	if !accept {
		if err := audit.LogStaffAction(ctx, tx, audit.Entry{StaffID: actor.PlayerID, TargetID: playerID, Action: "faction.application_reject",
			Source: actor.Source, Reason: fmt.Sprintf("%s application #%d: %s", factions.Name(faction), id, note)}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
