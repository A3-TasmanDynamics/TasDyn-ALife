// Package profile backs the player's own profile page: their website
// display name, linked accounts (Steam, Discord, TeamSpeak) and their
// moderation history -- bans, kicks, punishment points, cases they were
// part of and the support tickets they opened.
package profile

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NameCooldown is how long a player waits between display name changes.
const NameCooldown = 7 * 24 * time.Hour

// UserError is a message safe to show the player as-is.
type UserError struct{ Msg string }

func (e *UserError) Error() string { return e.Msg }

func userErr(format string, a ...any) error { return &UserError{Msg: fmt.Sprintf(format, a...)} }

// IsUserError reports whether err is a message for the player.
func IsUserError(err error) bool {
	var u *UserError
	return errors.As(err, &u)
}

// Profile is the player's own account.
type Profile struct {
	ID              int64
	UID             string
	GameName        string
	DisplayName     string
	NameChangedAt   *time.Time
	SteamName       string
	SteamAvatar     string
	SteamRefreshed  *time.Time
	DiscordID       string
	DiscordUsername string
	TeamSpeakUID    string
	TeamSpeakLinked *time.Time
	Created         time.Time
	LastSeen        *time.Time
}

// Name is what the site shows: the display name, else the game name, else
// the Steam name.
func (p Profile) Name() string {
	for _, n := range []string{p.DisplayName, p.GameName, p.SteamName} {
		if n != "" {
			return n
		}
	}
	return fmt.Sprintf("Player #%d", p.ID)
}

// Initials for the avatar placeholder.
func (p Profile) Initials() string {
	var out []rune
	for _, w := range strings.Fields(p.Name()) {
		for _, r := range w {
			out = append(out, r)
			break
		}
		if len(out) == 2 {
			break
		}
	}
	return strings.ToUpper(string(out))
}

// NextNameChange is when the display name can next change; zero when now.
func (p Profile) NextNameChange() time.Time {
	if p.NameChangedAt == nil {
		return time.Time{}
	}
	next := p.NameChangedAt.Add(NameCooldown)
	if next.Before(time.Now()) {
		return time.Time{}
	}
	return next
}

// Load reads a player's profile.
func Load(ctx context.Context, pool *pgxpool.Pool, playerID int64) (Profile, error) {
	p := Profile{ID: playerID}
	err := pool.QueryRow(ctx, `
		SELECT uid, name, COALESCE(display_name, ''), display_name_changed_at,
		       COALESCE(steam_name, ''), COALESCE(steam_avatar_url, ''), steam_refreshed_at,
		       COALESCE(discord_id, ''), COALESCE(discord_username, ''),
		       COALESCE(teamspeak_uid, ''), teamspeak_linked_at, created_at, last_seen
		FROM players WHERE id = $1`, playerID).Scan(&p.UID, &p.GameName, &p.DisplayName, &p.NameChangedAt,
		&p.SteamName, &p.SteamAvatar, &p.SteamRefreshed, &p.DiscordID, &p.DiscordUsername,
		&p.TeamSpeakUID, &p.TeamSpeakLinked, &p.Created, &p.LastSeen)
	return p, err
}

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _.'-]{1,22}[A-Za-z0-9.]$`)

// SetDisplayName changes (or, when name is empty, clears) the player's
// website display name. Names are 3-24 characters, unique ignoring case,
// and can change once every NameCooldown.
func SetDisplayName(ctx context.Context, pool *pgxpool.Pool, playerID int64, name string) error {
	name = strings.Join(strings.Fields(name), " ")
	p, err := Load(ctx, pool, playerID)
	if err != nil {
		return err
	}
	if name == p.DisplayName {
		return userErr("That's already your username.")
	}
	if name != "" && !validName.MatchString(name) {
		return userErr("Usernames are 3 to 24 characters: letters, numbers, spaces and . _ ' - (starting with a letter or number).")
	}
	if next := p.NextNameChange(); !next.IsZero() {
		return userErr("You can change your username again on %s.", next.Format("2 Jan 2006 at 15:04"))
	}
	if name != "" {
		var taken bool
		pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM players WHERE id <> $1 AND (lower(display_name) = lower($2)
			OR (display_name IS NULL AND lower(name) = lower($2))))`, playerID, name).Scan(&taken)
		if taken {
			return userErr("Someone already goes by %q.", name)
		}
	}
	_, err = pool.Exec(ctx, `UPDATE players SET display_name = NULLIF($2, ''), display_name_changed_at = now(), updated_at = now() WHERE id = $1`, playerID, name)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return userErr("Someone already goes by %q.", name)
	}
	return err
}

// A TeamSpeak unique identifier: 20 bytes, base64 encoded.
var validTSUID = regexp.MustCompile(`^[A-Za-z0-9+/]{27}=$`)

// SetTeamSpeak links (or, when uid is empty, unlinks) the player's
// TeamSpeak identity.
func SetTeamSpeak(ctx context.Context, pool *pgxpool.Pool, playerID int64, uid string) error {
	uid = strings.TrimSpace(uid)
	if uid != "" && !validTSUID.MatchString(uid) {
		return userErr("That doesn't look like a TeamSpeak unique ID. In TeamSpeak open Tools, Identities and copy the Unique ID (28 characters ending in =).")
	}
	_, err := pool.Exec(ctx, `UPDATE players SET teamspeak_uid = NULLIF($2, ''),
		teamspeak_linked_at = CASE WHEN $2 = '' THEN NULL ELSE now() END, updated_at = now() WHERE id = $1`, playerID, uid)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return userErr("That TeamSpeak identity is already linked to another player. Open a support ticket if it's yours.")
	}
	return err
}

// Ban is one entry from the banlist. Staff-only notes are never loaded.
type Ban struct {
	ID         int64
	Reason     string
	Scope      string
	By         string
	CaseID     int64
	Created    time.Time
	Expires    *time.Time
	Lifted     *time.Time
	LiftReason string
	Appeal     string // latest appeal's status, "" = none
}

// State is Active, Lifted or Expired.
func (b Ban) State() string {
	switch {
	case b.Lifted != nil:
		return "Lifted"
	case b.Expires != nil && b.Expires.Before(time.Now()):
		return "Expired"
	}
	return "Active"
}

// Length describes how long the ban was set for.
func (b Ban) Length() string {
	if b.Expires == nil {
		return "Permanent"
	}
	return humanDuration(b.Expires.Sub(b.Created))
}

// Kick is one kick_log entry.
type Kick struct {
	Type    string
	Reason  string
	By      string
	Created time.Time
}

// TypeLabel names the kind of kick.
func (k Kick) TypeLabel() string {
	switch k.Type {
	case "manual":
		return "Staff"
	case "vote":
		return "Vote"
	case "anticheat":
		return "Anti-cheat"
	case "battleye":
		return "BattlEye"
	}
	return k.Type
}

// Points is a punishment points entry.
type Points struct {
	Points  int
	Rules   string
	Comment string
	By      string
	CaseID  int64
	Created time.Time
	Expires *time.Time
	Revoked *time.Time
}

// State is Active, Expired or Revoked.
func (p Points) State() string {
	switch {
	case p.Revoked != nil:
		return "Revoked"
	case p.Expires != nil && p.Expires.Before(time.Now()):
		return "Expired"
	}
	return "Active"
}

// Case is a staff case the player was part of. The case summary and notes
// are staff-only; the player sees the type, their role and the outcome.
type Case struct {
	ID      int64
	Type    string
	Role    string
	Status  string
	Outcome string
	Created time.Time
	Closed  *time.Time
}

// RoleLabel describes the player's part in the case.
func (c Case) RoleLabel() string {
	switch c.Role {
	case "subject":
		return "Subject"
	case "reporter":
		return "Reporter"
	case "witness":
		return "Witness"
	case "related":
		return "Involved"
	}
	return c.Role
}

// Ticket is a support ticket the player opened.
type Ticket struct {
	ID       int64
	Subject  string
	Status   string
	Category string
	Replies  int
	Created  time.Time
	Updated  time.Time
}

// History is the player's moderation record.
type History struct {
	Bans         []Ban
	Kicks        []Kick
	Points       []Points
	ActivePoints int
	Cases        []Case
	Tickets      []Ticket
	OpenTickets  int
}

// Empty reports whether there's no moderation history at all.
func (h History) Empty() bool {
	return len(h.Bans)+len(h.Kicks)+len(h.Points)+len(h.Cases) == 0
}

const staffName = `COALESCE(NULLIF(%[1]s.display_name, ''), NULLIF(%[1]s.name, ''), NULLIF(%[1]s.steam_name, ''), 'Player #' || %[1]s.id)`

func nameOf(alias string) string { return fmt.Sprintf(staffName, alias) }

// Moderation loads the player's moderation history, newest first.
func Moderation(ctx context.Context, pool *pgxpool.Pool, playerID int64) (History, error) {
	var h History
	err := collect(ctx, pool, &h.Bans, `
		SELECT b.id, b.reason, b.scope, COALESCE(`+nameOf("s")+`, 'Staff'), COALESCE(b.case_id, 0),
		       b.created_at, b.expires_at, b.lifted_at, COALESCE(b.lift_reason, ''),
		       COALESCE((SELECT status FROM ban_appeals a WHERE a.ban_id = b.id AND a.player_id = $1 ORDER BY a.id DESC LIMIT 1), '')
		FROM banlist b LEFT JOIN players s ON s.id = b.banned_by
		WHERE b.player_id = $1 OR b.uid = (SELECT uid FROM players WHERE id = $1)
		ORDER BY b.created_at DESC`, playerID, func(r pgx.Rows, b *Ban) error {
		return r.Scan(&b.ID, &b.Reason, &b.Scope, &b.By, &b.CaseID, &b.Created, &b.Expires, &b.Lifted, &b.LiftReason, &b.Appeal)
	})
	if err != nil {
		return h, fmt.Errorf("profile: bans: %w", err)
	}
	err = collect(ctx, pool, &h.Kicks, `
		SELECT k.kick_type, COALESCE(k.reason, ''), CASE WHEN k.kicked_by IS NULL THEN 'System' ELSE `+nameOf("s")+` END, k.created_at
		FROM kick_log k LEFT JOIN players s ON s.id = k.kicked_by
		WHERE k.target_player_id = $1 ORDER BY k.created_at DESC LIMIT 200`, playerID, func(r pgx.Rows, k *Kick) error {
		return r.Scan(&k.Type, &k.Reason, &k.By, &k.Created)
	})
	if err != nil {
		return h, fmt.Errorf("profile: kicks: %w", err)
	}
	err = collect(ctx, pool, &h.Points, `
		SELECT pp.points, pp.rules, COALESCE(pp.comment, ''), COALESCE(`+nameOf("s")+`, 'Staff'), pp.case_id,
		       pp.created_at, pp.expires_at, pp.revoked_at
		FROM punishment_points pp LEFT JOIN players s ON s.id = pp.issued_by
		WHERE pp.player_id = $1 ORDER BY pp.created_at DESC`, playerID, func(r pgx.Rows, p *Points) error {
		return r.Scan(&p.Points, &p.Rules, &p.Comment, &p.By, &p.CaseID, &p.Created, &p.Expires, &p.Revoked)
	})
	if err != nil {
		return h, fmt.Errorf("profile: points: %w", err)
	}
	for _, p := range h.Points {
		if p.State() == "Active" {
			h.ActivePoints += p.Points
		}
	}
	err = collect(ctx, pool, &h.Cases, `
		SELECT DISTINCT ON (c.id) c.id, c.case_type, cp.role, c.status, COALESCE(c.outcome, ''), c.created_at, c.closed_at
		FROM staff_case_participants cp JOIN staff_cases c ON c.id = cp.case_id
		WHERE cp.player_id = $1 AND cp.role IN ('subject', 'related', 'reporter', 'witness')
		ORDER BY c.id DESC, array_position(ARRAY['subject', 'related', 'reporter', 'witness'], cp.role)`, playerID, func(r pgx.Rows, c *Case) error {
		return r.Scan(&c.ID, &c.Type, &c.Role, &c.Status, &c.Outcome, &c.Created, &c.Closed)
	})
	if err != nil {
		return h, fmt.Errorf("profile: cases: %w", err)
	}
	err = collect(ctx, pool, &h.Tickets, `
		SELECT t.id, t.subject, t.status, COALESCE(sc.label, c.label, ''),
		       (SELECT count(*) FROM support_ticket_messages m WHERE m.ticket_id = t.id AND NOT m.internal),
		       t.created_at, t.updated_at
		FROM support_tickets t LEFT JOIN ticket_categories c ON c.id = t.category_id
		LEFT JOIN ticket_categories sc ON sc.id = t.subcategory_id
		WHERE t.player_id = $1 ORDER BY t.updated_at DESC LIMIT 100`, playerID, func(r pgx.Rows, t *Ticket) error {
		return r.Scan(&t.ID, &t.Subject, &t.Status, &t.Category, &t.Replies, &t.Created, &t.Updated)
	})
	if err != nil {
		return h, fmt.Errorf("profile: tickets: %w", err)
	}
	for _, t := range h.Tickets {
		if t.Status != "closed" {
			h.OpenTickets++
		}
	}
	return h, nil
}

func collect[T any](ctx context.Context, pool *pgxpool.Pool, out *[]T, sql string, id int64, scan func(pgx.Rows, *T) error) error {
	rows, err := pool.Query(ctx, sql, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var v T
		if err := scan(rows, &v); err != nil {
			return err
		}
		*out = append(*out, v)
	}
	return rows.Err()
}

func humanDuration(d time.Duration) string {
	days := int(d.Hours()/24 + 0.5)
	switch {
	case days >= 365 && days%365 == 0:
		return plural(days/365, "year")
	case days >= 30 && days%30 == 0:
		return plural(days/30, "month")
	case days >= 7 && days%7 == 0:
		return plural(days/7, "week")
	case days >= 1:
		return plural(days, "day")
	}
	h := int(d.Hours() + 0.5)
	if h < 1 {
		h = 1
	}
	return plural(h, "hour")
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
