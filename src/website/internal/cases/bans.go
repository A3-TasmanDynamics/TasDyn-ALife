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
	"website/internal/notify"
)

// Ban reasons offered in the Issue ban dialog (layout plan).
var BanReasons = []string{
	"Exploit / duplication", "RDM", "VDM", "Combat logging", "Hate speech", "Cheating (anti-cheat flag)", "Other",
}

// Durations offered, in days (0 = permanent).
var BanDurations = []struct {
	Days  int
	Label string
}{{1, "1 day"}, {3, "3 days"}, {7, "7 days"}, {30, "30 days"}, {0, "Permanent"}}

// Ban is one banlist row.
type Ban struct {
	ID         int
	PlayerID   int64
	Player     string
	UID        string
	DiscordID  string
	Reason     string
	Note       string
	Scope      string // game / game_discord
	CaseID     int64
	By         string
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	LiftedAt   *time.Time
	LiftedBy   string
	LiftReason string
	Active     bool
	OpenAppeal int64 // id of an open appeal, 0 = none
}

// ScopeLabel is "game" or "game + Discord".
func (b Ban) ScopeLabel() string {
	if b.Scope == "game_discord" {
		return "game + Discord"
	}
	return "game"
}

// DurationLabel is e.g. "7-day ban" or "Permanent ban".
func DurationLabel(days int) string {
	if days == 0 {
		return "Permanent ban"
	}
	return fmt.Sprintf("%d-day ban", days)
}

// IssueBan bans a player on the case from the game (and optionally
// Discord). days = 0 is permanent; the caller checks bans.issue and, for
// permanent bans, bans.permanent. Returns the ban id and the player's linked
// Discord ID (for the bot to apply a Discord ban when scope includes it).
func IssueBan(ctx context.Context, pool *pgxpool.Pool, actor Actor, caseID, playerID int64, reason string, days int, discord bool, note string) (int, string, error) {
	reason, err := cleanText(reason, 300, "a reason")
	if err != nil {
		return 0, "", err
	}
	if days < 0 || days > 3650 {
		return 0, "", notAllowed("pick a ban length")
	}
	note = strings.TrimSpace(note)
	if len(note) > 2000 {
		return 0, "", notAllowed("the staff note is too long (2000 characters max)")
	}
	if playerID == actor.PlayerID {
		return 0, "", notAllowed("you can't ban yourself")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback(ctx)
	if st, err := lockCase(ctx, tx, caseID); err != nil {
		return 0, "", err
	} else if st != "open" {
		return 0, "", notAllowed("reopen the case first")
	}
	var uid, who string
	var discordID *string
	err = tx.QueryRow(ctx, `
		SELECT p.uid, `+name("p")+`, p.discord_id FROM staff_case_participants cp JOIN players p ON p.id = cp.player_id
		WHERE cp.case_id = $1 AND cp.player_id = $2 AND cp.role IN ('subject', 'related') LIMIT 1`, caseID, playerID).
		Scan(&uid, &who, &discordID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", notAllowed("add them to the case first")
	}
	if err != nil {
		return 0, "", err
	}
	var active int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM banlist WHERE uid = $1 AND (expires_at IS NULL OR expires_at > now())`, uid).Scan(&active); err != nil {
		return 0, "", err
	}
	if active > 0 {
		return 0, "", notAllowed("%s is already banned; lift that ban first to change it", who)
	}
	var expires *time.Time
	if days > 0 {
		t := time.Now().Add(time.Duration(days) * 24 * time.Hour)
		expires = &t
	}
	scope := "game"
	if discord {
		scope = "game_discord"
	}
	var notePtr *string
	if note != "" {
		notePtr = &note
	}
	var banID int
	if err := tx.QueryRow(ctx, `
		INSERT INTO banlist (uid, reason, banned_by, expires_at, case_id, player_id, scope, note)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		uid, reason, actor.PlayerID, expires, caseID, playerID, scope, notePtr).Scan(&banID); err != nil {
		return 0, "", err
	}
	label := DurationLabel(days)
	body := fmt.Sprintf("%s for %s: %s.", label, who, reason)
	if discord {
		body += " Also banned from Discord."
	}
	if note != "" {
		body += " Note: " + note
	}
	if _, err := addEntry(ctx, tx, caseID, actor.PlayerID, "ban", body, 0, fmt.Sprintf("banlist:%d", banID)); err != nil {
		return 0, "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE staff_cases SET outcome = $2 WHERE id = $1`, caseID, label); err != nil {
		return 0, "", err
	}
	after := map[string]any{"case": caseID, "ban": banID, "scope": scope, "days": days}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actor.PlayerID, TargetID: playerID, Action: "ban.issue", Source: actor.Source,
		Reason: fmt.Sprintf("%s: %s (case #%d)", label, reason, caseID), After: after,
	}); err != nil {
		return 0, "", err
	}
	d := ""
	if discordID != nil && discord {
		d = *discordID
	}
	return banID, d, tx.Commit(ctx)
}

// LiftBan ends a ban now (expires_at = now, which is what the game checks)
// and records why in its case. The caller checks bans.revoke. Returns the
// player's Discord ID when the ban included Discord, for the bot to unban.
func LiftBan(ctx context.Context, pool *pgxpool.Pool, actor Actor, banID int, reason string) (string, error) {
	reason, err := cleanText(reason, 1000, "a reason")
	if err != nil {
		return "", err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	d, err := liftBanTx(ctx, tx, actor, banID, reason, "ban.lift")
	if err != nil {
		return "", err
	}
	return d, tx.Commit(ctx)
}

func liftBanTx(ctx context.Context, tx pgx.Tx, actor Actor, banID int, reason, action string) (string, error) {
	var caseID *int64
	var playerID *int64
	var scope string
	var expires *time.Time
	var discordID *string
	err := tx.QueryRow(ctx, `
		SELECT b.case_id, b.player_id, b.scope, b.expires_at, p.discord_id
		FROM banlist b LEFT JOIN players p ON p.id = b.player_id WHERE b.id = $1 FOR UPDATE OF b`, banID).
		Scan(&caseID, &playerID, &scope, &expires, &discordID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", notAllowed("that ban doesn't exist")
	}
	if err != nil {
		return "", err
	}
	if expires != nil && !expires.After(time.Now()) {
		return "", notAllowed("that ban has already ended")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE banlist SET expires_at = now(), lifted_at = now(), lifted_by = $2, lift_reason = $3 WHERE id = $1`,
		banID, actor.PlayerID, reason); err != nil {
		return "", err
	}
	if caseID != nil {
		if _, err := addEntry(ctx, tx, *caseID, actor.PlayerID, "unban", "Ban lifted: "+reason, 0, fmt.Sprintf("banlist:%d", banID)); err != nil {
			return "", err
		}
	}
	var target int64
	if playerID != nil {
		target = *playerID
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actor.PlayerID, TargetID: target, Action: action, Source: actor.Source, Reason: reason,
		After: map[string]any{"ban": banID},
	}); err != nil {
		return "", err
	}
	if scope == "game_discord" && discordID != nil {
		return *discordID, nil
	}
	return "", nil
}

// BanStats are the Bans page tiles.
type BanStats struct {
	Active, Permanent, ExpiringWeek, OpenAppeals int
}

func Stats(ctx context.Context, pool *pgxpool.Pool) (BanStats, error) {
	var s BanStats
	err := pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE expires_at IS NULL OR expires_at > now()),
		       count(*) FILTER (WHERE expires_at IS NULL),
		       count(*) FILTER (WHERE expires_at > now() AND expires_at < now() + interval '7 days'),
		       (SELECT count(*) FROM ban_appeals WHERE status = 'open')
		FROM banlist`).Scan(&s.Active, &s.Permanent, &s.ExpiringWeek, &s.OpenAppeals)
	return s, err
}

// Bans lists active (active=true) or ended bans, newest first.
func Bans(ctx context.Context, pool *pgxpool.Pool, active bool, limit int) ([]Ban, error) {
	cond := `b.expires_at IS NOT NULL AND b.expires_at <= now()`
	if active {
		cond = `(b.expires_at IS NULL OR b.expires_at > now())`
	}
	rows, err := pool.Query(ctx, `
		SELECT b.id, COALESCE(p.id, 0), COALESCE(`+name("p")+`, b.uid), b.uid, COALESCE(p.discord_id, ''), b.reason,
		       COALESCE(b.note, ''), b.scope, COALESCE(b.case_id, 0), COALESCE(`+name("s")+`, 'System'), b.created_at,
		       b.expires_at, b.lifted_at, COALESCE(`+name("l")+`, ''), COALESCE(b.lift_reason, ''),
		       COALESCE((SELECT id FROM ban_appeals a WHERE a.ban_id = b.id AND a.status = 'open'), 0)
		FROM banlist b
		LEFT JOIN players p ON p.id = b.player_id OR (b.player_id IS NULL AND p.uid = b.uid)
		LEFT JOIN players s ON s.id = b.banned_by
		LEFT JOIN players l ON l.id = b.lifted_by
		WHERE `+cond+` ORDER BY b.id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Ban
	for rows.Next() {
		var b Ban
		if err := rows.Scan(&b.ID, &b.PlayerID, &b.Player, &b.UID, &b.DiscordID, &b.Reason, &b.Note, &b.Scope, &b.CaseID,
			&b.By, &b.CreatedAt, &b.ExpiresAt, &b.LiftedAt, &b.LiftedBy, &b.LiftReason, &b.OpenAppeal); err != nil {
			return nil, err
		}
		b.Active = active
		out = append(out, b)
	}
	return out, rows.Err()
}

// Appeal is one ban appeal.
type Appeal struct {
	ID        int64
	BanID     int
	PlayerID  int64
	Player    string
	Body      string
	Status    string
	Decision  string
	DecidedBy string
	CreatedAt time.Time
	Ban       Ban
}

// ActiveBan returns the player's current ban, if any, with its open or
// latest appeal.
func ActiveBan(ctx context.Context, pool *pgxpool.Pool, playerID int64) (*Ban, *Appeal, error) {
	var b Ban
	err := pool.QueryRow(ctx, `
		SELECT b.id, b.reason, b.created_at, b.expires_at, COALESCE(b.case_id, 0)
		FROM banlist b JOIN players p ON p.uid = b.uid
		WHERE p.id = $1 AND (b.expires_at IS NULL OR b.expires_at > now())
		ORDER BY b.expires_at DESC NULLS FIRST LIMIT 1`, playerID).Scan(&b.ID, &b.Reason, &b.CreatedAt, &b.ExpiresAt, &b.CaseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	b.Active = true
	var a Appeal
	err = pool.QueryRow(ctx, `
		SELECT id, status, COALESCE(decision, ''), created_at FROM ban_appeals
		WHERE ban_id = $1 AND player_id = $2 ORDER BY id DESC LIMIT 1`, b.ID, playerID).Scan(&a.ID, &a.Status, &a.Decision, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return &b, nil, nil
	}
	if err != nil {
		return &b, nil, err
	}
	return &b, &a, nil
}

// SubmitAppeal lets a banned player appeal their current ban, once per
// ban while an appeal is open, and not again after a rejection.
func SubmitAppeal(ctx context.Context, pool *pgxpool.Pool, playerID int64, body string) error {
	body = strings.TrimSpace(body)
	if len(body) < 20 {
		return notAllowed("please explain in a little more detail (20 characters minimum)")
	}
	if len(body) > 4000 {
		return notAllowed("your appeal is too long (4000 characters max)")
	}
	ban, last, err := ActiveBan(ctx, pool, playerID)
	if err != nil {
		return err
	}
	if ban == nil {
		return notAllowed("you don't have an active ban")
	}
	if last != nil && last.Status == "open" {
		return notAllowed("you already have an appeal waiting for review")
	}
	if last != nil && last.Status == "rejected" {
		return notAllowed("your appeal for this ban was already reviewed")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO ban_appeals (ban_id, player_id, body) VALUES ($1, $2, $3) RETURNING id`, ban.ID, playerID, body).Scan(&id); err != nil {
		return err
	}
	if ban.CaseID > 0 {
		if _, err := addEntry(ctx, tx, ban.CaseID, playerID, "appeal", "Appeal submitted by the player: "+body, 0, fmt.Sprintf("ban_appeals:%d", id)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Appeals lists open appeals, oldest first (the review queue).
func Appeals(ctx context.Context, pool *pgxpool.Pool) ([]Appeal, error) {
	rows, err := pool.Query(ctx, `
		SELECT a.id, a.ban_id, a.player_id, `+name("p")+`, a.body, a.status, a.created_at,
		       b.reason, b.created_at, b.expires_at, COALESCE(b.case_id, 0), COALESCE(`+name("s")+`, 'System')
		FROM ban_appeals a JOIN banlist b ON b.id = a.ban_id JOIN players p ON p.id = a.player_id
		LEFT JOIN players s ON s.id = b.banned_by
		WHERE a.status = 'open' ORDER BY a.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Appeal
	for rows.Next() {
		var a Appeal
		if err := rows.Scan(&a.ID, &a.BanID, &a.PlayerID, &a.Player, &a.Body, &a.Status, &a.CreatedAt,
			&a.Ban.Reason, &a.Ban.CreatedAt, &a.Ban.ExpiresAt, &a.Ban.CaseID, &a.Ban.By); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DecideAppeal accepts (lifting the ban) or rejects an appeal. The caller
// checks bans.appeal_review. Returns the Discord ID to unban, if any.
func DecideAppeal(ctx context.Context, pool *pgxpool.Pool, actor Actor, appealID int64, accept bool, decision string) (string, error) {
	decision, err := cleanText(decision, 1000, "a reason for your decision")
	if err != nil {
		return "", err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var banID int
	var playerID int64
	var status string
	var caseID *int64
	err = tx.QueryRow(ctx, `
		SELECT a.ban_id, a.player_id, a.status, b.case_id FROM ban_appeals a JOIN banlist b ON b.id = a.ban_id
		WHERE a.id = $1 FOR UPDATE OF a`, appealID).Scan(&banID, &playerID, &status, &caseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", notAllowed("that appeal doesn't exist")
	}
	if err != nil {
		return "", err
	}
	if status != "open" {
		return "", notAllowed("that appeal was already decided")
	}
	if playerID == actor.PlayerID {
		return "", notAllowed("you can't decide your own appeal")
	}
	newStatus := "rejected"
	if accept {
		newStatus = "accepted"
	}
	title := "Ban appeal rejected"
	if accept {
		title = "Ban appeal accepted: your ban is lifted"
	}
	if err := notify.Send(ctx, tx, playerID, title, decision, "/dashboard"); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE ban_appeals SET status = $2, decided_by = $3, decision = $4, decided_at = now() WHERE id = $1`,
		appealID, newStatus, actor.PlayerID, decision); err != nil {
		return "", err
	}
	discordID := ""
	if accept {
		if discordID, err = liftBanTx(ctx, tx, actor, banID, "Appeal accepted: "+decision, "ban.appeal_accept"); err != nil {
			return "", err
		}
	} else {
		if caseID != nil {
			if _, err := addEntry(ctx, tx, *caseID, actor.PlayerID, "appeal", "Appeal rejected: "+decision, 0, fmt.Sprintf("ban_appeals:%d", appealID)); err != nil {
				return "", err
			}
		}
		if err := audit.LogStaffAction(ctx, tx, audit.Entry{
			StaffID: actor.PlayerID, TargetID: playerID, Action: "ban.appeal_reject", Source: actor.Source, Reason: decision,
			After: map[string]any{"appeal": appealID, "ban": banID},
		}); err != nil {
			return "", err
		}
	}
	return discordID, tx.Commit(ctx)
}
