package handlers

import (
	"log/slog"
	"net/http"

	"website/internal/auth"
)

type playerStats struct {
	Name             string
	CivCash          int64
	CivBank          int64
	CivPlaytimeHours int64
	CopLevel         int32
	CopCash          int64
	CopBank          int64
	MedicLevel       int32
	MedicCash        int64
	MedicBank        int64
	DiscordUsername  string
}

type gangMember struct {
	Name string
	Rank string
}

type gangInfo struct {
	Name    string
	Tag     string
	Balance int64
	Members []gangMember
}

type dashboardData struct {
	Base
	Player          playerStats
	Gang            *gangInfo
	DiscordLinkCode string
}

func (d *Deps) Dashboard(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	data := dashboardData{Base: baseFrom(r, "Dashboard")}

	var discordUsername *string
	var civPlaytimeSeconds int64
	err := d.Pool.QueryRow(r.Context(), `
		SELECT name, civ_cash, civ_bank, civ_playtime_seconds,
		       cop_level, cop_cash, cop_bank,
		       medic_level, medic_cash, medic_bank,
		       discord_username
		FROM players WHERE id = $1
	`, sess.PlayerID).Scan(
		&data.Player.Name, &data.Player.CivCash, &data.Player.CivBank, &civPlaytimeSeconds,
		&data.Player.CopLevel, &data.Player.CopCash, &data.Player.CopBank,
		&data.Player.MedicLevel, &data.Player.MedicCash, &data.Player.MedicBank,
		&discordUsername,
	)
	if err != nil {
		slog.Error("dashboard: loading player failed", "error", err)
		http.Error(w, "Failed to load your account.", http.StatusInternalServerError)
		return
	}
	data.Player.CivPlaytimeHours = civPlaytimeSeconds / 3600
	if discordUsername != nil {
		data.Player.DiscordUsername = *discordUsername
	}

	var gangID int64
	var g gangInfo
	err = d.Pool.QueryRow(r.Context(), `
		SELECT g.id, g.name, g.tag, COALESCE(ga.balance, 0)
		FROM gang_members gm
		JOIN gangs g ON g.id = gm.gang_id
		LEFT JOIN gang_accounts ga ON ga.gang_id = g.id
		WHERE gm.player_id = $1
	`, sess.PlayerID).Scan(&gangID, &g.Name, &g.Tag, &g.Balance)
	if err == nil {
		rows, rerr := d.Pool.Query(r.Context(), `
			SELECT p.name, gm.rank FROM gang_members gm
			JOIN players p ON p.id = gm.player_id
			WHERE gm.gang_id = $1 ORDER BY gm.rank DESC, p.name
		`, gangID)
		if rerr == nil {
			defer rows.Close()
			for rows.Next() {
				var m gangMember
				if rows.Scan(&m.Name, &m.Rank) == nil {
					g.Members = append(g.Members, m)
				}
			}
		}
		data.Gang = &g
	}

	d.Render.Render(w, "dashboard.html", data)
}

// GenerateDiscordLinkCode issues a one-time code for the Discord `/link`
// command (the mirror-image of DiscordConnect's OAuth path) and re-renders
// the dashboard with it shown. POST, not GET -- it has a side effect (a new
// row in discord_link_codes).
func (d *Deps) GenerateDiscordLinkCode(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())

	code, _, err := auth.GenerateLinkCode(r.Context(), d.Pool, sess.PlayerID)
	if err != nil {
		slog.Error("dashboard: generating discord link code failed", "error", err)
		http.Redirect(w, r, "/dashboard?error="+errMsg("Couldn't generate a code, try again."), http.StatusSeeOther)
		return
	}

	// Re-render inline rather than redirecting -- a redirect would lose the
	// code (it's only meaningful once, shown once), and it's short-lived
	// enough that reflecting it via a query param isn't worth the
	// leak-into-browser-history tradeoff.
	data := dashboardData{Base: baseFrom(r, "Dashboard"), DiscordLinkCode: code}
	if err := d.loadDashboardPlayer(r, sess.PlayerID, &data); err != nil {
		http.Error(w, "Failed to load your account.", http.StatusInternalServerError)
		return
	}
	d.Render.Render(w, "dashboard.html", data)
}

func (d *Deps) loadDashboardPlayer(r *http.Request, playerID int64, data *dashboardData) error {
	var discordUsername *string
	var civPlaytimeSeconds int64
	err := d.Pool.QueryRow(r.Context(), `
		SELECT name, civ_cash, civ_bank, civ_playtime_seconds,
		       cop_level, cop_cash, cop_bank,
		       medic_level, medic_cash, medic_bank,
		       discord_username
		FROM players WHERE id = $1
	`, playerID).Scan(
		&data.Player.Name, &data.Player.CivCash, &data.Player.CivBank, &civPlaytimeSeconds,
		&data.Player.CopLevel, &data.Player.CopCash, &data.Player.CopBank,
		&data.Player.MedicLevel, &data.Player.MedicCash, &data.Player.MedicBank,
		&discordUsername,
	)
	if err != nil {
		return err
	}
	data.Player.CivPlaytimeHours = civPlaytimeSeconds / 3600
	if discordUsername != nil {
		data.Player.DiscordUsername = *discordUsername
	}
	return nil
}
