package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"website/internal/audit"
	"website/internal/auth"
	"website/internal/cases"
	"website/internal/notify"
	"website/internal/players"
)

type staffLogRow struct {
	StaffName  string
	Action     string
	TargetName string
	Reason     string
	Source     string
	CreatedAt  string
}

// AdminShell is what partial_admin_sidebar.html needs on top of Base --
// embedded into every Admin Panel page's data so the sidebar renders the
// same everywhere. AdminTab picks the highlighted nav item.
type AdminShell struct {
	AdminTab  string
	StaffRank string
	// Sidebar badges (layout plan): open cases, active bans, open flags.
	OpenCases  int
	ActiveBans int
	OpenFlags  int
	OpenApps   int
	// Support section: open tickets, and unassigned ones.
	OpenTickets       int
	UnassignedTickets int
	// Perms are the viewer's permission keys: the sidebar only shows
	// pages they can open.
	Perms map[string]bool
}

// adminShell looks up the signed-in staff member's rank display name for
// the sidebar footer. Cosmetic only -- access was already decided by
// RequireAdminPanel -- so a failed lookup just leaves the rank blank.
func (d *Deps) adminShell(r *http.Request, tab string) AdminShell {
	s := AdminShell{AdminTab: tab}
	sess, ok := auth.FromContext(r.Context())
	if !ok || sess == nil {
		return s
	}
	_ = d.Pool.QueryRow(r.Context(), `
		SELECT COALESCE(sr.display_name, ''),
		       (SELECT count(*) FROM staff_cases WHERE status = 'open'),
		       (SELECT count(*) FROM banlist WHERE expires_at IS NULL OR expires_at > now()),
		       (SELECT count(*) FROM anti_cheat_flags WHERE resolution IS NULL),
		       (SELECT count(*) FROM staff_applications WHERE status IN ('pending', 'interview')),
		       (SELECT count(*) FROM support_tickets WHERE status <> 'closed'),
		       (SELECT count(*) FROM support_tickets WHERE status <> 'closed' AND assigned_staff_id IS NULL)
		FROM players p LEFT JOIN staff_ranks sr ON sr.id = p.staff_rank_id
		WHERE p.id = $1
	`, sess.PlayerID).Scan(&s.StaffRank, &s.OpenCases, &s.ActiveBans, &s.OpenFlags, &s.OpenApps, &s.OpenTickets, &s.UnassignedTickets)
	s.Perms, _ = auth.Effective(r.Context(), d.Pool, sess.PlayerID)
	return s
}

type adminHomeData struct {
	Base
	AdminShell
	OnlineCount  int
	OpenTickets  int
	FlagsLast24h int
	TotalPlayers int
	StaffLog     []staffLogRow

	// Economy & population row and rich list (layout plan dashboard,
	// GAMEPANEL_PARITY §5.4). Staff-only: never shown publicly.
	PoliceCount int
	EMSCount    int
	Money       int64 // whole dollars
	RichList    []richRow

	// Cases (GAMEPANEL_PARITY §3.4): open count, opened per day over the
	// last 14 days, and the viewer's own recent cases.
	OpenCaseCount int
	CaseBars      []caseBar
	CaseTotal     int
	MyCases       []cases.ListRow
	MyActivity    cases.Activity
	CanCases      bool

	Notes  []notify.Item
	Unread int
}

type caseBar struct {
	Day    string
	Tick   bool
	Count  int
	Height int // percent of the chart
}

type richRow struct {
	ID    int64
	Name  string
	Total string
	Flag  bool // has an unreviewed anti-cheat flag
}

func (d adminHomeData) MoneySupply() string { return players.Dollars(d.Money) }

// AdminHome is the Admin Panel landing page: headline counts plus the
// read-only staff log. Player lookup, bans, anti-cheat review, the database
// browser and the arsenal editor are designed (docs/WEBSITE.md §7) but not
// built yet -- the sidebar shows them as "Soon" rather than dead links.
func (d *Deps) AdminHome(w http.ResponseWriter, r *http.Request) {
	if sess, ok := auth.FromContext(r.Context()); ok && !sess.AdminPanelAccess {
		// Support-only staff: their part of the panel is Support.
		http.Redirect(w, r, "/admin/support", http.StatusSeeOther)
		return
	}
	data := adminHomeData{Base: baseFrom(r, "Admin Panel"), AdminShell: d.adminShell(r, "dashboard")}

	// Headline counts are non-essential -- a failed one shows 0 rather than
	// taking the whole panel down, same as the landing page's stats.
	ctx := r.Context()
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM player_sessions WHERE disconnected_at IS NULL`).Scan(&data.OnlineCount); err != nil {
		slog.Error("admin home: online count failed", "error", err)
	}
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM support_tickets WHERE status <> 'closed'`).Scan(&data.OpenTickets); err != nil {
		slog.Error("admin home: open ticket count failed", "error", err)
	}
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM anti_cheat_flags WHERE created_at > now() - interval '24 hours'`).Scan(&data.FlagsLast24h); err != nil {
		slog.Error("admin home: anti-cheat flag count failed", "error", err)
	}
	if err := d.Pool.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE cop_level > 0), count(*) FILTER (WHERE medic_level > 0),
		       COALESCE(sum(COALESCE(civ_cash, 0) + COALESCE(cop_cash, 0) + COALESCE(medic_cash, 0)), 0)
		         + COALESCE((SELECT sum(balance) FROM bank_accounts), 0)
		FROM players`).Scan(&data.TotalPlayers, &data.PoliceCount, &data.EMSCount, &data.Money); err != nil {
		slog.Error("admin home: population/economy counts failed", "error", err)
	}
	if rows, err := d.Pool.Query(ctx, `
		SELECT p.id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id),
		       COALESCE(p.civ_cash, 0) + COALESCE(p.cop_cash, 0) + COALESCE(p.medic_cash, 0)
		         + COALESCE((SELECT sum(balance) FROM bank_accounts b WHERE b.player_id = p.id), 0) AS total,
		       EXISTS (SELECT 1 FROM anti_cheat_flags f WHERE f.player_id = p.id AND f.reviewed_at IS NULL)
		FROM players p ORDER BY total DESC, p.id LIMIT 10`); err != nil {
		slog.Error("admin home: rich list failed", "error", err)
	} else {
		for rows.Next() {
			var rr richRow
			var total int64
			if rows.Scan(&rr.ID, &rr.Name, &total, &rr.Flag) == nil {
				rr.Total = players.Dollars(total)
				data.RichList = append(data.RichList, rr)
			}
		}
		rows.Close()
	}

	rows, err := d.Pool.Query(ctx, `
		SELECT COALESCE(NULLIF(staff.name, ''), staff.steam_name,
		                CASE sl.source WHEN 'game' THEN 'In-game' WHEN 'manual' THEN 'Manual DB change' ELSE 'System' END),
		       sl.action, sl.before_value, sl.after_value, sl.source,
		       COALESCE(NULLIF(target.name, ''), target.steam_name, ''), COALESCE(sl.reason, ''), sl.created_at
		FROM staff_log sl
		LEFT JOIN players staff ON staff.id = sl.staff_player_id
		LEFT JOIN players target ON target.id = sl.target_player_id
		ORDER BY sl.created_at DESC
		LIMIT 10
	`)
	if err != nil {
		slog.Error("admin home: staff log query failed", "error", err)
		http.Error(w, "Failed to load the staff log.", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var row staffLogRow
		var createdAt time.Time
		var action string
		var before, after []byte
		if err := rows.Scan(&row.StaffName, &action, &before, &after, &row.Source, &row.TargetName, &row.Reason, &createdAt); err == nil {
			row.Action = audit.Describe(action, before, after)
			row.CreatedAt = createdAt.Format("2006-01-02 15:04")
			data.StaffLog = append(data.StaffLog, row)
		}
	}

	data.OpenCaseCount = data.OpenCases
	if sess, ok := auth.FromContext(ctx); ok {
		data.Notes, data.Unread, _ = notify.List(ctx, d.Pool, sess.PlayerID, 5)
	}
	if data.CanCases = d.can(r, "cases.view"); data.CanCases {
		if counts, days, err := cases.PerDay(ctx, d.Pool, 14, time.Local); err == nil {
			top := 1
			for _, c := range counts {
				top = max(top, c)
				data.CaseTotal += c
			}
			for i, c := range counts {
				data.CaseBars = append(data.CaseBars, caseBar{Day: days[i].Format("2"), Tick: i%2 == 0 || i == len(counts)-1, Count: c, Height: c * 100 / top})
			}
		}
		sess, _ := auth.FromContext(ctx)
		data.MyCases, _ = cases.List(ctx, d.Pool, cases.Filter{Mine: sess.PlayerID, Limit: 5})
		data.MyActivity, _ = cases.StaffActivity(ctx, d.Pool, sess.PlayerID)
	}

	d.Render.Render(w, "admin_home.html", data)
}
