package lookup

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/audit"
	"website/internal/gamenames"
)

var ErrNotAllowed = errors.New("not allowed")

// Actor is the staff member acting, and from where.
type Actor struct {
	PlayerID int64
	Source   string
}

// VehicleActions are what staff can do to a vehicle from the website, by
// the state it has to be in. A vehicle that's out in the world is the
// game's (it's saved again when it's stored or impounded), so those go
// through the in-game admin menu ("Their vehicles") instead.
var VehicleActions = map[string]struct {
	From  []string
	To    string
	Label string
}{
	"release": {[]string{"impounded"}, "garaged", "Release from impound"},
	"destroy": {[]string{"garaged", "impounded"}, "destroyed", "Mark destroyed"},
	"restore": {[]string{"destroyed"}, "garaged", "Restore to garage"},
}

// ActionsFor lists the actions that apply to a vehicle in this state.
func ActionsFor(status string) []string {
	var out []string
	for _, a := range []string{"release", "restore", "destroy"} {
		for _, f := range VehicleActions[a].From {
			if f == status {
				out = append(out, a)
			}
		}
	}
	return out
}

// ActionLabel is the button text for an action.
func ActionLabel(a string) string { return VehicleActions[a].Label }

// VehicleAction changes a vehicle's state (release / destroy / restore),
// with the reason in the staff log (which the vehicle's history shows).
// The caller checks players.vehicle_actions.
func VehicleAction(ctx context.Context, pool *pgxpool.Pool, actor Actor, id int64, action, reason string) (string, error) {
	act, ok := VehicleActions[action]
	if !ok {
		return "", fmt.Errorf("%w: unknown action", ErrNotAllowed)
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 300 {
		return "", fmt.Errorf("%w: a reason (up to 300 characters) is required", ErrNotAllowed)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var status, class, plate string
	var owner *int64
	err = tx.QueryRow(ctx, `SELECT status, classname, COALESCE(plate, ''), owner_player_id FROM vehicles WHERE id = $1 FOR UPDATE`, id).
		Scan(&status, &class, &plate, &owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("%w: that vehicle doesn't exist", ErrNotAllowed)
	}
	if err != nil {
		return "", err
	}
	allowed := false
	for _, f := range act.From {
		allowed = allowed || f == status
	}
	if !allowed {
		if status == "spawned" {
			return "", fmt.Errorf("%w: it's out in the world; use the in-game admin menu (Their vehicles) to send it to the garage first", ErrNotAllowed)
		}
		var from []string
		for _, f := range act.From {
			from = append(from, strings.ToLower(StatusLabel(f)))
		}
		return "", fmt.Errorf("%w: %s only works on a vehicle that's %s (this one is %s)", ErrNotAllowed, act.Label, strings.Join(from, " or "), strings.ToLower(StatusLabel(status)))
	}
	if _, err := tx.Exec(ctx, `UPDATE vehicles SET status = $2 WHERE id = $1`, id, act.To); err != nil {
		return "", err
	}
	var target int64
	if owner != nil {
		target = *owner
	}
	name := gamenames.Vehicle(class)
	if plate != "" {
		name += " " + plate
	}
	if err := audit.LogStaffAction(ctx, tx, audit.Entry{
		StaffID: actor.PlayerID, TargetID: target, Action: "vehicle." + action, Source: actor.Source,
		Reason: name + ": " + reason,
		Before: map[string]any{"vehicle": id, "status": status},
		After:  map[string]any{"vehicle": id, "status": act.To},
	}); err != nil {
		return "", err
	}
	return name, tx.Commit(ctx)
}
