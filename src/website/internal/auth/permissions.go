package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Permission is one gated website/bot action. The catalogue below is the
// only list of keys that exist: the role editor can only offer these, and
// Can refuses any key not in it, so a typo'd key grants nothing instead of
// silently checking a permission nobody can hold (docs/GAMEPANEL_PARITY.md
// §10). Resolution follows docs/ADMIN_TOOLS.md §7: a per-player override
// row wins; otherwise the player's rank level must be >= MinLevel.
type Permission struct {
	Key      string
	Label    string
	Group    string
	MinLevel int
	// NoOverride keys can only come from rank level -- a Head Admin can't
	// hand one to a lower rank as a one-off (same idea as the in-game debug
	// console exclusion).
	NoOverride bool
}

// Rank levels, matching the seeded staff_ranks rows.
const (
	LevelTrialMod  = 10
	LevelModerator = 20
	LevelAdmin     = 40
	LevelHeadAdmin = 100
)

// Catalogue holds default thresholds; they're starting points to tune, and
// the role editor (Wave 1) will let per-rank grants refine them.
var Catalogue = []Permission{
	{Key: "players.view", Label: "Look up players", Group: "Players", MinLevel: LevelTrialMod},
	{Key: "players.vehicles", Label: "View player vehicles", Group: "Players", MinLevel: LevelModerator},
	{Key: "players.edit_police", Label: "Set police level", Group: "Players", MinLevel: LevelAdmin},
	{Key: "players.edit_medic", Label: "Set EMS level", Group: "Players", MinLevel: LevelAdmin},
	{Key: "players.compensate", Label: "Compensate players", Group: "Players", MinLevel: LevelAdmin},
	{Key: "players.compensate_large", Label: "Compensate large amounts", Group: "Players", MinLevel: LevelHeadAdmin},

	{Key: "cases.view", Label: "View cases", Group: "Discipline", MinLevel: LevelTrialMod},
	{Key: "cases.create", Label: "Open cases, add notes, warn", Group: "Discipline", MinLevel: LevelTrialMod},
	{Key: "cases.close", Label: "Close cases", Group: "Discipline", MinLevel: LevelModerator},
	{Key: "bans.issue", Label: "Issue temporary bans", Group: "Discipline", MinLevel: LevelModerator},
	{Key: "bans.permanent", Label: "Issue permanent bans", Group: "Discipline", MinLevel: LevelAdmin},
	{Key: "bans.revoke", Label: "Lift bans", Group: "Discipline", MinLevel: LevelAdmin},

	{Key: "staff.view", Label: "View staff directory", Group: "Staff", MinLevel: LevelTrialMod},
	{Key: "staff.loa", Label: "Put staff on LOA / reinstate", Group: "Staff", MinLevel: LevelAdmin},
	{Key: "staff.edit", Label: "Promote / demote staff", Group: "Staff", MinLevel: LevelHeadAdmin, NoOverride: true},
	{Key: "staff.suspend", Label: "Suspend staff", Group: "Staff", MinLevel: LevelHeadAdmin, NoOverride: true},
	{Key: "staff.remove", Label: "Remove from staff", Group: "Staff", MinLevel: LevelHeadAdmin, NoOverride: true},
	{Key: "roles.manage", Label: "Edit ranks, permissions and role sync", Group: "Staff", MinLevel: LevelHeadAdmin, NoOverride: true},
	{Key: "applications.view", Label: "View applications", Group: "Staff", MinLevel: LevelModerator},
	{Key: "applications.decide", Label: "Accept / reject applications", Group: "Staff", MinLevel: LevelAdmin},

	{Key: "factions.audit", Label: "View faction audit logs", Group: "Factions", MinLevel: LevelModerator},
	{Key: "factions.configure", Label: "Configure faction ranks", Group: "Factions", MinLevel: LevelHeadAdmin},
	{Key: "records.review", Label: "Review faction records", Group: "Factions", MinLevel: LevelAdmin},

	{Key: "announce.post", Label: "Post announcements", Group: "Community", MinLevel: LevelAdmin},
	{Key: "server.logs", Label: "View live server logs", Group: "Server", MinLevel: LevelAdmin},
	{Key: "server.control", Label: "Restart / stop servers", Group: "Server", MinLevel: LevelHeadAdmin, NoOverride: true},
	{Key: "bot.admin", Label: "Bot health and forced syncs", Group: "Server", MinLevel: LevelHeadAdmin},
}

var catalogueByKey = func() map[string]Permission {
	m := make(map[string]Permission, len(Catalogue))
	for _, p := range Catalogue {
		m[p.Key] = p
	}
	return m
}()

// ErrUnknownPermission means a caller asked about a key that isn't in the
// Catalogue -- a programming error, surfaced loudly rather than as "no".
var ErrUnknownPermission = errors.New("auth: unknown permission key")

// Denial explains a "no" so callers can show a specific message.
type Denial string

const (
	Allowed      Denial = ""
	DenyNotStaff Denial = "not_staff"
	DenyInactive Denial = "inactive" // suspended / LOA
	DenyLevel    Denial = "level"
)

// Can reports whether playerID may perform key right now. Suspended/LOA
// staff are denied everything, overrides included -- same rule as panel
// access (resolvePanelAccess).
func Can(ctx context.Context, pool *pgxpool.Pool, playerID int64, key string) (Denial, error) {
	perm, ok := catalogueByKey[key]
	if !ok {
		return DenyLevel, ErrUnknownPermission
	}

	var level *int
	var status string
	err := pool.QueryRow(ctx, `
		SELECT sr.level, p.staff_status
		FROM players p LEFT JOIN staff_ranks sr ON sr.id = p.staff_rank_id
		WHERE p.id = $1
	`, playerID).Scan(&level, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return DenyNotStaff, nil
	}
	if err != nil {
		return DenyLevel, err
	}
	if level == nil {
		return DenyNotStaff, nil
	}
	if status != "active" {
		return DenyInactive, nil
	}

	if !perm.NoOverride {
		var allow bool
		err := pool.QueryRow(ctx, `
			SELECT allow FROM staff_permission_overrides WHERE player_id = $1 AND command_key = $2
		`, playerID, key).Scan(&allow)
		switch {
		case err == nil && allow:
			return Allowed, nil
		case err == nil && !allow:
			return DenyLevel, nil
		case !errors.Is(err, pgx.ErrNoRows):
			return DenyLevel, err
		}
	}

	if *level >= perm.MinLevel {
		return Allowed, nil
	}
	return DenyLevel, nil
}
