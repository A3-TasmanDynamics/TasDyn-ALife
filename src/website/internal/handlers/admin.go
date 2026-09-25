package handlers

import (
	"log/slog"
	"net/http"
	"time"
)

type staffLogRow struct {
	StaffName  string
	Action     string
	TargetName string
	Reason     string
	CreatedAt  string
}

type adminHomeData struct {
	Base
	StaffLog []staffLogRow
}

// AdminHome is the Admin Panel landing page. Currently a read-only staff
// log viewer -- player lookup, ban/unban, rank management, anti-cheat flag
// review, and the arsenal editor are designed in docs/WEBSITE.md §7 but not
// yet built (see the template's "Coming soon" note); this establishes the
// access-gated route and the panel switcher, which the rest builds on top
// of incrementally.
func (d *Deps) AdminHome(w http.ResponseWriter, r *http.Request) {
	data := adminHomeData{Base: baseFrom(r, "Admin Panel")}

	rows, err := d.Pool.Query(r.Context(), `
		SELECT COALESCE(staff.name, 'System'), sl.action, COALESCE(target.name, ''), COALESCE(sl.reason, ''), sl.created_at
		FROM staff_log sl
		LEFT JOIN players staff ON staff.id = sl.staff_player_id
		LEFT JOIN players target ON target.id = sl.target_player_id
		ORDER BY sl.created_at DESC
		LIMIT 50
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
		if err := rows.Scan(&row.StaffName, &row.Action, &row.TargetName, &row.Reason, &createdAt); err == nil {
			row.CreatedAt = createdAt.Format("2006-01-02 15:04")
			data.StaffLog = append(data.StaffLog, row)
		}
	}

	d.Render.Render(w, "admin_home.html", data)
}
