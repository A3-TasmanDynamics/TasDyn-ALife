package auth

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// DeniedInfo says why a staff area refused a request, for the layout plan's
// access-denied pages: a suspended or on-leave staff member sees why and
// until when, not a generic 403.
type DeniedInfo struct {
	Kind       string // "suspended", "loa", "awaiting", "not_staff", "no_permission", "not_command"
	Area       string // "admin" or "support"
	Since      *time.Time
	Until      *time.Time
	Reason     string
	By         string
	Permission string // no_permission: the missing permission's label

	// awaiting: their open staff application.
	AppID     int64
	AppStatus string // pending / interview
}

// deny answers a refused request with a.Denied (the rendered page), or
// plain text if no renderer is wired in.
func (a *Authenticator) deny(w http.ResponseWriter, r *http.Request, playerID int64, area, perm string) {
	info := a.denialInfo(r.Context(), playerID)
	info.Area = area
	if info.Kind == "" && perm == "" {
		// Not staff, but an application is in progress: say so instead
		// of the generic page (layout plan "Denied — Application in review").
		var created time.Time
		err := a.Pool.QueryRow(r.Context(), `
			SELECT id, status, created_at FROM staff_applications
			WHERE player_id = $1 AND status IN ('pending', 'interview') ORDER BY id DESC LIMIT 1`, playerID).
			Scan(&info.AppID, &info.AppStatus, &created)
		if err == nil {
			info.Kind, info.Since = "awaiting", &created
		}
	}
	if info.Kind == "" {
		info.Kind = "not_staff"
		if perm != "" {
			info.Kind = "no_permission"
			info.Permission = perm
			for _, p := range Catalogue {
				if p.Key == perm {
					info.Permission = p.Label
				}
			}
		}
	}
	if a.Denied != nil {
		a.Denied(w, r, info)
		return
	}
	msg := "403 Forbidden: you don't have access to this page"
	switch info.Kind {
	case "suspended":
		msg = "Your staff access is suspended."
	case "loa":
		msg = "You're on leave, so staff tools are paused."
	}
	http.Error(w, msg, http.StatusForbidden)
}

// denialInfo fills Kind for suspended/LOA staff, with when, until, why and
// who (from the latest staff_status change). Kind stays "" otherwise.
// Best-effort: a lookup failure falls back to the generic page.
func (a *Authenticator) denialInfo(ctx context.Context, playerID int64) DeniedInfo {
	var info DeniedInfo
	var status string
	var reason *string
	if err := a.Pool.QueryRow(ctx, `
		SELECT p.staff_status, p.staff_status_reason, p.staff_status_until
		FROM players p WHERE p.id = $1 AND p.staff_rank_id IS NOT NULL`, playerID,
	).Scan(&status, &reason, &info.Until); err != nil || status == "active" {
		return DeniedInfo{}
	}
	info.Kind = status
	if reason != nil {
		info.Reason = *reason
	}
	var since time.Time
	var by string
	err := a.Pool.QueryRow(ctx, `
		SELECT rc.changed_at, COALESCE(NULLIF(s.name, ''), NULLIF(s.steam_name, ''), '')
		FROM rank_changes rc LEFT JOIN players s ON s.id = rc.actor_id
		WHERE rc.player_id = $1 AND rc.field = 'staff_status' AND rc.new_value = $2
		ORDER BY rc.id DESC LIMIT 1`, playerID, status).Scan(&since, &by)
	if err == nil {
		info.Since, info.By = &since, by
	}
	return info
}

// livePanelAccess re-resolves panel access from the database, so a
// suspension, LOA or removal takes effect on the very next request rather
// than at the next sign-in.
func (a *Authenticator) livePanelAccess(ctx context.Context, sess *Session) (admin, support bool) {
	admin, support, err := resolvePanelAccess(ctx, a.Pool, sess.PlayerID)
	if err != nil {
		slog.Error("auth: live panel access check failed, using the session's", "error", err)
		return sess.AdminPanelAccess, sess.SupportPanelAccess
	}
	return admin, support
}
