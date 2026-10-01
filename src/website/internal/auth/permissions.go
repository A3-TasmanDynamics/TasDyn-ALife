package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Permission is one gated website/bot action. The catalogue below is the
// only list of keys that exist: the role editor can only offer these, and
// Can refuses any key not in it, so a typo'd key grants nothing instead of
// silently checking a permission nobody can hold (docs/GAMEPANEL_PARITY.md
// §10).
//
// Resolution: a per-player override row wins (docs/ADMIN_TOOLS.md §7);
// otherwise the player's rank must have the key ticked in rank_permissions
// (edited on /admin/roles, the layout plan's Roles & Permissions board).
type Permission struct {
	Key   string
	Label string
	Group string
	// SeedLevel is only a starting point: the schema seeds each built-in
	// rank with every key whose SeedLevel is at or below the rank's level.
	// After that, rank_permissions is the source of truth.
	SeedLevel int
	// NoOverride keys can only come from a rank, never from a one-off
	// per-player override.
	NoOverride bool
}

// Levels of the seeded staff_ranks rows.
const (
	LevelTrialMod  = 10
	LevelModerator = 20
	LevelAdmin     = 40
	LevelHeadAdmin = 100
)

// Catalogue groups and keys follow the layout plan's Roles board; the
// extra keys at the end of some groups are ones the website already uses.
var Catalogue = []Permission{
	{Key: "staff.view", Label: "View staff directory", Group: "Staff", SeedLevel: LevelTrialMod},
	{Key: "staff.edit", Label: "Edit rank, team, region", Group: "Staff", SeedLevel: LevelHeadAdmin, NoOverride: true},
	{Key: "staff.loa", Label: "Put staff on LOA", Group: "Staff", SeedLevel: LevelAdmin},
	{Key: "staff.suspend", Label: "Suspend staff", Group: "Staff", SeedLevel: LevelHeadAdmin, NoOverride: true},
	{Key: "staff.remove", Label: "Remove from staff", Group: "Staff", SeedLevel: LevelHeadAdmin, NoOverride: true},
	{Key: "roles.manage", Label: "Edit roles & permissions", Group: "Staff", SeedLevel: LevelHeadAdmin, NoOverride: true},
	{Key: "staff.notes", Label: "Read and add staff notes", Group: "Staff", SeedLevel: LevelAdmin},

	{Key: "cases.view", Label: "View cases", Group: "Moderation", SeedLevel: LevelTrialMod},
	{Key: "cases.lead", Label: "Open and lead cases", Group: "Moderation", SeedLevel: LevelTrialMod},
	{Key: "cases.close", Label: "Close cases", Group: "Moderation", SeedLevel: LevelModerator},
	{Key: "bans.issue", Label: "Issue timed bans", Group: "Moderation", SeedLevel: LevelModerator},
	{Key: "bans.permanent", Label: "Issue permanent bans", Group: "Moderation", SeedLevel: LevelAdmin},
	{Key: "bans.revoke", Label: "Lift bans", Group: "Moderation", SeedLevel: LevelAdmin},
	{Key: "bans.appeal_review", Label: "Review ban appeals", Group: "Moderation", SeedLevel: LevelAdmin},
	{Key: "anticheat.review", Label: "Review anti-cheat flags", Group: "Moderation", SeedLevel: LevelModerator},

	{Key: "players.view", Label: "Look up players", Group: "Players", SeedLevel: LevelTrialMod},
	{Key: "players.vehicles", Label: "View vehicles tab", Group: "Players", SeedLevel: LevelModerator},
	// Faction levels are faction command's job; these are the Management-only
	// staff override (GAMEPANEL_PARITY §5.2), never granted per-player.
	{Key: "players.edit_police", Label: "Override police rank (Management)", Group: "Players", SeedLevel: LevelHeadAdmin, NoOverride: true},
	{Key: "players.edit_medic", Label: "Override EMS rank (Management)", Group: "Players", SeedLevel: LevelHeadAdmin, NoOverride: true},
	{Key: "players.compensate", Label: "Compensate players", Group: "Players", SeedLevel: LevelAdmin},
	{Key: "players.compensate_large", Label: "Compensate above threshold", Group: "Players", SeedLevel: LevelHeadAdmin},

	{Key: "applications.view", Label: "View staff applications", Group: "Recruitment", SeedLevel: LevelModerator},
	{Key: "applications.decide", Label: "Accept or reject applications", Group: "Recruitment", SeedLevel: LevelAdmin},

	{Key: "factions.audit", Label: "View faction command logs", Group: "Factions", SeedLevel: LevelModerator},
	{Key: "factions.configure", Label: "Edit faction rank names", Group: "Factions", SeedLevel: LevelHeadAdmin},
	{Key: "records.review", Label: "Review faction records", Group: "Factions", SeedLevel: LevelAdmin},

	{Key: "server.logs", Label: "View live server logs", Group: "Server", SeedLevel: LevelAdmin},
	{Key: "server.control", Label: "Restart, stop and start servers", Group: "Server", SeedLevel: LevelHeadAdmin, NoOverride: true},
	{Key: "database.query", Label: "Read-only database browser", Group: "Server", SeedLevel: LevelHeadAdmin, NoOverride: true},
	{Key: "announce.post", Label: "Post announcements", Group: "Server", SeedLevel: LevelAdmin},
	{Key: "rules.edit", Label: "Edit the server rules", Group: "Server", SeedLevel: LevelAdmin},
	{Key: "bot.admin", Label: "Bot health and forced syncs", Group: "Server", SeedLevel: LevelHeadAdmin},

	{Key: "dev.tools", Label: "Development: system health, website logs, project board", Group: "Development", SeedLevel: LevelHeadAdmin},
}

var catalogueByKey = func() map[string]Permission {
	m := make(map[string]Permission, len(Catalogue))
	for _, p := range Catalogue {
		m[p.Key] = p
	}
	return m
}()

// Known reports whether key is in the Catalogue.
func Known(key string) bool {
	_, ok := catalogueByKey[key]
	return ok
}

// CatalogueGroups returns the catalogue grouped, in catalogue order.
func CatalogueGroups() []PermissionGroup {
	var out []PermissionGroup
	idx := map[string]int{}
	for _, p := range Catalogue {
		i, ok := idx[p.Group]
		if !ok {
			i = len(out)
			idx[p.Group] = i
			out = append(out, PermissionGroup{Name: p.Group})
		}
		out[i].Perms = append(out[i].Perms, p)
	}
	return out
}

type PermissionGroup struct {
	Name  string
	Perms []Permission
}

// ErrUnknownPermission means a caller asked about a key that isn't in the
// Catalogue -- a programming error, surfaced loudly rather than as "no".
var ErrUnknownPermission = errors.New("auth: unknown permission key")

// Denial explains a "no" so callers can show a specific message.
type Denial string

const (
	Allowed      Denial = ""
	DenyNotStaff Denial = "not_staff"
	DenyInactive Denial = "inactive" // suspended / LOA
	DenyLevel    Denial = "level"    // rank doesn't grant it (name kept for callers)
)

// RequirePermission gates a route on one Catalogue key, checked live on
// every request (not cached in the session), so a demotion, suspension or
// permission change takes effect immediately.
func (a *Authenticator) RequirePermission(key string) func(http.Handler) http.Handler {
	if !Known(key) {
		panic("auth: RequirePermission with unknown key " + key) // a wiring bug; fail at startup
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, ok := FromContext(r.Context())
			if !ok {
				http.Redirect(w, r, "/?login_required=1", http.StatusSeeOther)
				return
			}
			denial, err := Can(r.Context(), a.Pool, sess.PlayerID, key)
			if err != nil {
				http.Error(w, "Couldn't check your permissions. Please try again.", http.StatusInternalServerError)
				return
			}
			if denial != Allowed {
				a.deny(w, r, sess.PlayerID, "admin", key)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Can reports whether playerID may perform key right now. Suspended/LOA
// staff are denied everything, overrides included -- same rule as panel
// access (resolvePanelAccess).
func Can(ctx context.Context, pool *pgxpool.Pool, playerID int64, key string) (Denial, error) {
	perm, ok := catalogueByKey[key]
	if !ok {
		return DenyLevel, ErrUnknownPermission
	}

	var rankID *int
	var status string
	var rankHas bool
	err := pool.QueryRow(ctx, `
		SELECT p.staff_rank_id, p.staff_status,
		       EXISTS (SELECT 1 FROM rank_permissions rp WHERE rp.rank_id = p.staff_rank_id AND rp.command_key = $2)
		FROM players p WHERE p.id = $1
	`, playerID, key).Scan(&rankID, &status, &rankHas)
	if errors.Is(err, pgx.ErrNoRows) {
		return DenyNotStaff, nil
	}
	if err != nil {
		return DenyLevel, err
	}
	if rankID == nil {
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

	if rankHas {
		return Allowed, nil
	}
	return DenyLevel, nil
}

// Effective returns every catalogue key the player currently holds (rank
// grants plus per-player overrides), for showing only the pages they can
// open. Empty for non-staff and inactive staff.
func Effective(ctx context.Context, pool *pgxpool.Pool, playerID int64) (map[string]bool, error) {
	out := map[string]bool{}
	var status string
	var rankID *int
	err := pool.QueryRow(ctx, `SELECT staff_rank_id, staff_status FROM players WHERE id = $1`, playerID).Scan(&rankID, &status)
	if errors.Is(err, pgx.ErrNoRows) || rankID == nil || status != "active" {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	rows, err := pool.Query(ctx, `SELECT command_key FROM rank_permissions WHERE rank_id = $1`, *rankID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var k string
		if rows.Scan(&k) == nil {
			out[k] = true
		}
	}
	rows.Close()
	rows, err = pool.Query(ctx, `SELECT command_key, allow FROM staff_permission_overrides WHERE player_id = $1`, playerID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var allow bool
		if rows.Scan(&k, &allow) != nil {
			continue
		}
		if p, ok := catalogueByKey[k]; ok && !p.NoOverride {
			out[k] = allow
		}
	}
	for k := range out {
		if _, ok := catalogueByKey[k]; !ok {
			delete(out, k)
		}
	}
	return out, rows.Err()
}
