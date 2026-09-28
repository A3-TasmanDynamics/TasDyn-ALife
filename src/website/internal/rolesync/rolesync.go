// Package rolesync keeps Discord roles (and, later, TeamSpeak server
// groups) in step with the website's database -- docs/INTEGRATIONS.md
// §2.3 and §3.
//
//  1. Entitlements: a player's database state becomes platform-neutral
//     strings such as "linked", "staff_rank:admin", "faction_rank:police:3".
//  2. platform_group_map ties entitlements to a platform's groups.
//  3. Reconcile: for each of the player's identities on each platform,
//     add missing mapped groups and remove mapped groups they shouldn't
//     have. Groups that aren't in platform_group_map are NEVER touched --
//     that's what makes sync safe on a server with hand-given roles.
//
// Every add/remove is written to sync_log. Sync runs when the database's
// change trigger sends NOTIFY rank_changed, and in a full pass every 15
// minutes that catches anything missed while a platform was unreachable.
package rolesync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Adapter is one platform (Discord, TeamSpeak).
type Adapter interface {
	Platform() string // matches platform_group_map.platform
	// Identities returns the player's account IDs on this platform (none if
	// they haven't linked it).
	Identities(ctx context.Context, playerID int64) ([]string, error)
	// CurrentGroups returns the identity's current groups; present=false
	// means the identity isn't on the server (e.g. left the Discord), which
	// is skipped, not an error.
	CurrentGroups(ctx context.Context, identity string) (groups []string, present bool, err error)
	AddGroup(ctx context.Context, identity, group string) error
	RemoveGroup(ctx context.Context, identity, group string) error
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slug normalises free text (team names) for entitlement keys.
func Slug(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "_"), "_")
}

// Entitlements computes a player's entitlements from the database.
//   - "linked" for every identity being synced (they linked an account);
//   - staff: "staff" + "staff_rank:<key>" when active; "staff_loa"
//     instead of the rank while on LOA; nothing while suspended; plus
//     "staff_team:<slug>" when active or on LOA;
//   - "faction:police" + "faction_rank:police:<level>" when cop_level > 0,
//     and the same for EMS ("ems");
//   - "gang:<id>" for gang members.
func Entitlements(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, playerID int64) ([]string, error) {
	var rankKey, status, team *string
	var cop, medic int
	var gangID *int64
	err := q.QueryRow(ctx, `
		SELECT sr.key, p.staff_status, p.staff_team, COALESCE(p.cop_level, 0), COALESCE(p.medic_level, 0),
		       (SELECT gang_id FROM gang_members WHERE player_id = p.id LIMIT 1)
		FROM players p LEFT JOIN staff_ranks sr ON sr.id = p.staff_rank_id
		WHERE p.id = $1`, playerID).Scan(&rankKey, &status, &team, &cop, &medic, &gangID)
	if err != nil {
		return nil, err
	}
	out := []string{"linked"}
	if rankKey != nil && status != nil {
		switch *status {
		case "active":
			out = append(out, "staff", "staff_rank:"+*rankKey)
		case "loa":
			out = append(out, "staff_loa")
		}
		if team != nil && *team != "" && *status != "suspended" {
			out = append(out, "staff_team:"+Slug(*team))
		}
	}
	if cop > 0 {
		out = append(out, "faction:police", "faction_rank:police:"+strconv.Itoa(cop))
	}
	if medic > 0 {
		out = append(out, "faction:ems", "faction_rank:ems:"+strconv.Itoa(medic))
	}
	if gangID != nil {
		out = append(out, "gang:"+strconv.FormatInt(*gangID, 10))
	}
	return out, nil
}

// Mapping is one platform_group_map row.
type Mapping struct {
	Entitlement string
	GroupID     string
}

// Mappings returns the platform's mappings.
func Mappings(ctx context.Context, pool *pgxpool.Pool, platform string) ([]Mapping, error) {
	rows, err := pool.Query(ctx, `SELECT entitlement, group_id FROM platform_group_map WHERE platform = $1 ORDER BY 1, 2`, platform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Mapping
	for rows.Next() {
		var m Mapping
		if err := rows.Scan(&m.Entitlement, &m.GroupID); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Plan works out what to add and remove, given the mappings, a player's
// entitlements and their current groups. Only mapped groups are ever
// added or removed. Pure, so it's unit-tested directly.
func Plan(mappings []Mapping, entitlements, current []string) (add, remove []string) {
	managed := map[string]bool{}
	want := map[string]bool{}
	has := map[string]bool{}
	ent := map[string]bool{}
	for _, e := range entitlements {
		ent[e] = true
	}
	for _, m := range mappings {
		managed[m.GroupID] = true
		if ent[m.Entitlement] {
			want[m.GroupID] = true
		}
	}
	for _, g := range current {
		has[g] = true
	}
	for g := range want {
		if !has[g] {
			add = append(add, g)
		}
	}
	for g := range has {
		if managed[g] && !want[g] {
			remove = append(remove, g)
		}
	}
	sort.Strings(add)
	sort.Strings(remove)
	return add, remove
}

// Engine runs sync for every adapter.
type Engine struct {
	Pool     *pgxpool.Pool
	Adapters []Adapter
	// FullPassEvery is the periodic full-sync interval (default 15 min).
	FullPassEvery time.Duration
}

// Result summarises one player's sync on one platform.
type Result struct {
	Platform string
	Identity string
	Added    []string
	Removed  []string
	Errors   []string
}

// SyncPlayer reconciles one player on every platform. cause is the
// rank_changes row that triggered it (0 = periodic or manual).
func (e *Engine) SyncPlayer(ctx context.Context, playerID int64, cause int64) ([]Result, error) {
	ents, err := Entitlements(ctx, e.Pool, playerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // player deleted since the notification
	}
	if err != nil {
		return nil, err
	}
	var results []Result
	for _, a := range e.Adapters {
		maps, err := Mappings(ctx, e.Pool, a.Platform())
		if err != nil {
			return results, err
		}
		if len(maps) == 0 {
			continue // nothing mapped on this platform -> nothing to do
		}
		ids, err := a.Identities(ctx, playerID)
		if err != nil {
			return results, err
		}
		for _, id := range ids {
			res := Result{Platform: a.Platform(), Identity: id}
			current, present, err := a.CurrentGroups(ctx, id)
			if err == nil && !present {
				continue
			}
			if err != nil {
				res.Errors = append(res.Errors, err.Error())
				e.log(ctx, playerID, a.Platform(), "add", "", cause, err)
				results = append(results, res)
				continue
			}
			add, remove := Plan(maps, ents, current)
			for _, g := range add {
				err := a.AddGroup(ctx, id, g)
				e.log(ctx, playerID, a.Platform(), "add", g, cause, err)
				if err != nil {
					res.Errors = append(res.Errors, fmt.Sprintf("add %s: %v", g, err))
				} else {
					res.Added = append(res.Added, g)
				}
			}
			for _, g := range remove {
				err := a.RemoveGroup(ctx, id, g)
				e.log(ctx, playerID, a.Platform(), "remove", g, cause, err)
				if err != nil {
					res.Errors = append(res.Errors, fmt.Sprintf("remove %s: %v", g, err))
				} else {
					res.Removed = append(res.Removed, g)
				}
			}
			if len(res.Added)+len(res.Removed)+len(res.Errors) > 0 {
				results = append(results, res)
			}
		}
	}
	return results, nil
}

func (e *Engine) log(ctx context.Context, playerID int64, platform, action, group string, cause int64, err error) {
	var errText *string
	if err != nil {
		s := err.Error()
		errText = &s
	}
	var causeID *int64
	if cause > 0 {
		causeID = &cause
	}
	if _, lerr := e.Pool.Exec(ctx, `
		INSERT INTO sync_log (player_id, platform, action, group_id, rank_change_id, ok, error)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7)`,
		playerID, platform, action, group, causeID, err == nil, errText); lerr != nil {
		slog.Error("rolesync: writing sync_log failed", "error", lerr)
	}
}

// Strip removes every mapped group from one identity -- used just before an
// account is unlinked, so synced roles don't outlive the link. Unmapped
// groups are left alone, as always. Returns the groups removed.
func (e *Engine) Strip(ctx context.Context, playerID int64, platform, identity string) ([]string, error) {
	for _, a := range e.Adapters {
		if a.Platform() != platform {
			continue
		}
		maps, err := Mappings(ctx, e.Pool, platform)
		if err != nil || len(maps) == 0 {
			return nil, err
		}
		current, present, err := a.CurrentGroups(ctx, identity)
		if err != nil || !present {
			return nil, err
		}
		_, remove := Plan(maps, nil, current)
		var removed []string
		for _, g := range remove {
			err := a.RemoveGroup(ctx, identity, g)
			e.log(ctx, playerID, platform, "remove", g, 0, err)
			if err == nil {
				removed = append(removed, g)
			}
		}
		return removed, nil
	}
	return nil, nil
}

// SyncAll reconciles every player who has linked any platform.
func (e *Engine) SyncAll(ctx context.Context) (players, changes, failures int) {
	rows, err := e.Pool.Query(ctx, `SELECT id FROM players WHERE discord_id IS NOT NULL ORDER BY id`)
	if err != nil {
		slog.Error("rolesync: full pass query failed", "error", err)
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		res, err := e.SyncPlayer(ctx, id, 0)
		players++
		if err != nil {
			failures++
			slog.Warn("rolesync: player sync failed", "player_id", id, "error", err)
			continue
		}
		for _, r := range res {
			changes += len(r.Added) + len(r.Removed)
			failures += len(r.Errors)
		}
	}
	return
}

// Run listens for rank_changed notifications and runs the periodic full
// pass until ctx is cancelled.
func (e *Engine) Run(ctx context.Context) {
	every := e.FullPassEvery
	if every == 0 {
		every = 15 * time.Minute
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			if p, c, f := e.SyncAll(ctx); c+f > 0 {
				slog.Info("rolesync: full pass", "players", p, "changes", c, "failures", f)
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()

	backoff := time.Second
	for ctx.Err() == nil {
		err := e.listen(ctx)
		if ctx.Err() != nil {
			return
		}
		slog.Warn("rolesync: listener stopped, reconnecting", "error", err, "in", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < time.Minute {
			backoff *= 2
		}
	}
}

func (e *Engine) listen(ctx context.Context) error {
	conn, err := e.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `LISTEN rank_changed`); err != nil {
		return err
	}
	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		id, perr := strconv.ParseInt(n.Payload, 10, 64)
		if perr != nil {
			continue
		}
		var cause int64
		_ = e.Pool.QueryRow(ctx, `SELECT COALESCE(max(id), 0) FROM rank_changes WHERE player_id = $1`, id).Scan(&cause)
		if _, err := e.SyncPlayer(ctx, id, cause); err != nil {
			slog.Warn("rolesync: sync after change failed (the full pass will retry)", "player_id", id, "error", err)
		}
	}
}
