// Package notify is in-site notifications (docs/GAMEPANEL_PARITY.md §7.3):
// a bell with the unread count, notification lists on the dashboards, and
// essential notices (a policy or rule change) that show a banner on every
// panel page until the person acknowledges them.
//
// Links are same-site paths only, so a notification can never send anyone
// off-site. Sending never fails the action that caused it: callers inside a
// transaction use Send with that tx; best-effort callers use SendNow.
package notify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
)

// ErrNotAllowed wraps a rule violation with a human-readable reason.
var ErrNotAllowed = errors.New("not allowed")

// Execer is a pool or a transaction.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// SafeLink returns link if it's a same-site path, otherwise "".
func SafeLink(link string) string {
	link = strings.TrimSpace(link)
	if !strings.HasPrefix(link, "/") || strings.HasPrefix(link, "//") || strings.ContainsAny(link, "\\\r\n") {
		return ""
	}
	return link
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}

// Send notifies one player (inside the caller's transaction, if q is one).
func Send(ctx context.Context, q Execer, playerID int64, title, body, link string) error {
	if playerID == 0 {
		return nil
	}
	var linkPtr *string
	if l := SafeLink(link); l != "" {
		linkPtr = &l
	}
	_, err := q.Exec(ctx, `INSERT INTO notifications (player_id, title, body, link) VALUES ($1, $2, $3, $4)`,
		playerID, clip(title, 160), clip(body, 400), linkPtr)
	return err
}

// Item is one notification.
type Item struct {
	ID           int64
	Title        string
	Body         string
	Link         string
	Essential    bool
	Read         bool
	Acknowledged bool
	CreatedAt    time.Time
}

// When is a short time label: "10:12" today, "Yesterday", or "27 Sep".
func (i Item) When() string {
	now := time.Now()
	t := i.CreatedAt.In(now.Location())
	switch {
	case t.YearDay() == now.YearDay() && t.Year() == now.Year():
		return t.Format("15:04")
	case now.Sub(t) < 48*time.Hour && t.YearDay() == now.Add(-24*time.Hour).YearDay():
		return "Yesterday"
	}
	return t.Format("2 Jan")
}

// List returns the player's latest notifications and their unread count.
func List(ctx context.Context, pool *pgxpool.Pool, playerID int64, limit int) ([]Item, int, error) {
	var unread int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE player_id = $1 AND read_at IS NULL`, playerID).Scan(&unread); err != nil {
		return nil, 0, err
	}
	rows, err := pool.Query(ctx, `
		SELECT id, title, body, COALESCE(link, ''), essential, read_at IS NOT NULL, acknowledged_at IS NOT NULL, created_at
		FROM notifications WHERE player_id = $1 ORDER BY id DESC LIMIT $2`, playerID, limit)
	if err != nil {
		return nil, unread, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var i Item
		if err := rows.Scan(&i.ID, &i.Title, &i.Body, &i.Link, &i.Essential, &i.Read, &i.Acknowledged, &i.CreatedAt); err != nil {
			return nil, unread, err
		}
		out = append(out, i)
	}
	return out, unread, rows.Err()
}

// Open marks one notification read and returns where it points ("" = none).
func Open(ctx context.Context, pool *pgxpool.Pool, playerID, id int64) (string, error) {
	var link string
	err := pool.QueryRow(ctx, `
		UPDATE notifications SET read_at = COALESCE(read_at, now()) WHERE id = $1 AND player_id = $2
		RETURNING COALESCE(link, '')`, id, playerID).Scan(&link)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return SafeLink(link), err
}

// MarkAllRead marks every notification read (essential ones still need
// acknowledging).
func MarkAllRead(ctx context.Context, pool *pgxpool.Pool, playerID int64) error {
	_, err := pool.Exec(ctx, `UPDATE notifications SET read_at = now() WHERE player_id = $1 AND read_at IS NULL`, playerID)
	return err
}

// Pending is an essential notice the viewer hasn't acknowledged, for the
// panel banner.
type Pending struct {
	ID    int64 // notification id
	Title string
	Body  string
	Link  string
	Acked int
	Total int
}

// PendingEssential returns the oldest unacknowledged essential notice.
func PendingEssential(ctx context.Context, pool *pgxpool.Pool, playerID int64) (*Pending, error) {
	var p Pending
	var noticeID *int64
	err := pool.QueryRow(ctx, `
		SELECT id, title, body, COALESCE(link, ''), notice_id FROM notifications
		WHERE player_id = $1 AND essential AND acknowledged_at IS NULL ORDER BY id LIMIT 1`, playerID).
		Scan(&p.ID, &p.Title, &p.Body, &p.Link, &noticeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if noticeID != nil {
		_ = pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE acknowledged_at IS NOT NULL), count(*) FROM notifications WHERE notice_id = $1`, *noticeID).
			Scan(&p.Acked, &p.Total)
	}
	return &p, nil
}

// Acknowledge records "I've read this" on an essential notification.
func Acknowledge(ctx context.Context, pool *pgxpool.Pool, playerID, id int64) error {
	_, err := pool.Exec(ctx, `
		UPDATE notifications SET acknowledged_at = COALESCE(acknowledged_at, now()), read_at = COALESCE(read_at, now())
		WHERE id = $1 AND player_id = $2 AND essential`, id, playerID)
	return err
}

// Broadcast sends an essential notice to all active staff ("staff") or to
// every player who has used the website ("everyone"), inside tx. Returns
// the notice id and how many people it went to.
func Broadcast(ctx context.Context, tx pgx.Tx, actorID int64, title, body, link, audience string) (int64, int64, error) {
	title, body = strings.TrimSpace(title), strings.TrimSpace(body)
	if len(title) < 3 || len(title) > 120 {
		return 0, 0, fmt.Errorf("%w: the title must be 3–120 characters", ErrNotAllowed)
	}
	if len(body) > 2000 {
		return 0, 0, fmt.Errorf("%w: the message is too long (2000 characters max)", ErrNotAllowed)
	}
	if audience != "staff" && audience != "everyone" {
		return 0, 0, fmt.Errorf("%w: unknown audience", ErrNotAllowed)
	}
	var linkPtr *string
	if l := SafeLink(link); l != "" {
		linkPtr = &l
	} else if strings.TrimSpace(link) != "" {
		return 0, 0, fmt.Errorf("%w: links must be a path on this website, like /rules", ErrNotAllowed)
	}
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO staff_notices (title, body, link, audience, created_by) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		title, body, linkPtr, audience, actorID).Scan(&id); err != nil {
		return 0, 0, err
	}
	who := `SELECT id FROM players WHERE staff_rank_id IS NOT NULL AND staff_status <> 'suspended'`
	if audience == "everyone" {
		who = `SELECT DISTINCT player_id FROM web_sessions`
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO notifications (player_id, title, body, link, essential, notice_id)
		SELECT r.id, $1, $2, $3, true, $4 FROM (`+who+`) r(id)`, title, clip(body, 400), linkPtr, id)
	if err != nil {
		return 0, 0, err
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{StaffID: actorID, Action: "notice.post",
		Reason: title, After: map[string]any{"notice": id, "audience": audience, "recipients": tag.RowsAffected()}}); err != nil {
		return 0, 0, err
	}
	return id, tag.RowsAffected(), nil
}

// Notice is a posted essential notice with its acknowledgement progress.
type Notice struct {
	ID        int64
	Title     string
	Body      string
	Link      string
	Audience  string
	By        string
	CreatedAt time.Time
	Acked     int
	Total     int
	Waiting   []string // names that haven't acknowledged (first 20)
}

// Notices lists recent essential notices, newest first.
func Notices(ctx context.Context, pool *pgxpool.Pool, limit int) ([]Notice, error) {
	rows, err := pool.Query(ctx, `
		SELECT n.id, n.title, n.body, COALESCE(n.link, ''), n.audience,
		       COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Unknown'), n.created_at,
		       (SELECT count(*) FILTER (WHERE acknowledged_at IS NOT NULL) FROM notifications WHERE notice_id = n.id),
		       (SELECT count(*) FROM notifications WHERE notice_id = n.id),
		       COALESCE((SELECT array_agg(nm) FROM (
		           SELECT COALESCE(NULLIF(w.name, ''), NULLIF(w.steam_name, ''), 'Player #' || w.id) AS nm
		           FROM notifications x JOIN players w ON w.id = x.player_id
		           WHERE x.notice_id = n.id AND x.acknowledged_at IS NULL ORDER BY 1 LIMIT 20) s), '{}')
		FROM staff_notices n LEFT JOIN players p ON p.id = n.created_by
		ORDER BY n.id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Notice
	for rows.Next() {
		var n Notice
		if err := rows.Scan(&n.ID, &n.Title, &n.Body, &n.Link, &n.Audience, &n.By, &n.CreatedAt, &n.Acked, &n.Total, &n.Waiting); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
