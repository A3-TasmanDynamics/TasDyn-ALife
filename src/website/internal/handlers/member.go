package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"website/internal/auth"
	"website/internal/bank"
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
	TransferToken   string
}

func (d *Deps) Dashboard(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	transferToken, _ := auth.RandomState()
	data := dashboardData{Base: baseFrom(r, "Dashboard"), TransferToken: transferToken}

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
	transferToken, _ := auth.RandomState()
	data := dashboardData{Base: baseFrom(r, "Dashboard"), DiscordLinkCode: code, TransferToken: transferToken}
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

// Transfer handles both "Send Money" forms on the dashboard (move between
// my own faction accounts, or send to another player) -- dispatched on the
// "mode" field rather than two separate routes, since both end up calling
// the same bank package underneath. See internal/bank/transfer.go for why
// this was safe to build (fn_save.sqf never writes *_bank).
func (d *Deps) Transfer(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/dashboard?error="+errMsg("Invalid form submission."), http.StatusSeeOther)
		return
	}

	amountDollars, err := strconv.ParseInt(r.FormValue("amount"), 10, 64)
	if err != nil || amountDollars <= 0 {
		http.Redirect(w, r, "/dashboard?error="+errMsg("Enter a whole dollar amount greater than zero."), http.StatusSeeOther)
		return
	}
	token := r.FormValue("transfer_token")
	fromFaction := r.FormValue("from_faction")

	switch r.FormValue("mode") {
	case "own":
		toFaction := r.FormValue("to_faction")
		err = bank.TransferOwnAccounts(r.Context(), d.Pool, sess.PlayerID, fromFaction, toFaction, amountDollars, token)
	case "player":
		recipientName := r.FormValue("recipient_name")
		toFaction := r.FormValue("recipient_faction")
		err = bank.TransferToPlayer(r.Context(), d.Pool, sess.PlayerID, fromFaction, recipientName, toFaction, amountDollars, token)
	default:
		http.Redirect(w, r, "/dashboard?error="+errMsg("Unknown transfer type."), http.StatusSeeOther)
		return
	}

	if err != nil {
		if isKnownBankError(err) {
			http.Redirect(w, r, "/dashboard?error="+errMsg(err.Error()), http.StatusSeeOther)
			return
		}
		slog.Error("transfer failed", "error", err)
		http.Redirect(w, r, "/dashboard?error="+errMsg("Something went wrong processing that transfer."), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/dashboard?notice="+errMsg("Transfer complete."), http.StatusSeeOther)
}

// isKnownBankError distinguishes a bank package sentinel (safe, specific,
// user-facing message -- e.g. "insufficient funds") from an unexpected
// error (DB connectivity, a bug) that should show a generic message and
// get logged instead of echoed to the browser.
func isKnownBankError(err error) bool {
	switch err {
	case bank.ErrInvalidAmount, bank.ErrInvalidFaction, bank.ErrSameAccount, bank.ErrInsufficientFunds,
		bank.ErrDuplicateRequest, bank.ErrRecipientNotFound, bank.ErrRecipientAmbiguous, bank.ErrSelfTransfer:
		return true
	default:
		return false
	}
}
