package handlers

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"

	"website/internal/auth"
	"website/internal/bank"
	"website/internal/dashboard"
)

type gangMember struct {
	PlayerID int64
	Name     string
	Rank     string
}

type gangInfo struct {
	Name     string
	Tag      string
	Balance  int64
	IsLeader bool
	Members  []gangMember
}

type dashboardData struct {
	Base
	dashboard.Overview
	Boards          []dashboard.Board
	Gang            *gangInfo
	DiscordLinkCode string
	TransferToken   string
	JoinURL         string // steam://connect link, "" when the game address isn't configured
}

// Dashboard is the player dashboard (layout plan "Player — Dashboard").
func (d *Deps) Dashboard(w http.ResponseWriter, r *http.Request) {
	d.renderDashboard(w, r, "")
}

// renderDashboard loads and renders the dashboard; linkCode, if set, is a
// freshly generated Discord /link code shown once.
func (d *Deps) renderDashboard(w http.ResponseWriter, r *http.Request, linkCode string) {
	sess, _ := auth.FromContext(r.Context())
	transferToken, _ := auth.RandomState()
	data := dashboardData{Base: baseFrom(r, "Dashboard"), TransferToken: transferToken, DiscordLinkCode: linkCode}
	ov, err := dashboard.Load(r.Context(), d.Pool, sess.PlayerID)
	if err != nil {
		slog.Error("dashboard: loading player failed", "error", err)
		http.Error(w, "Failed to load your account.", http.StatusInternalServerError)
		return
	}
	data.Overview = ov
	if data.Boards, err = dashboard.Leaderboards(r.Context(), d.Pool, sess.PlayerID, ov.OptIn); err != nil {
		slog.Error("dashboard: leaderboards failed", "error", err)
	}
	data.JoinURL = joinURL(d.Cfg.GameQueryAddr)
	data.Gang = d.loadGang(r, sess.PlayerID)
	d.Render.Render(w, "dashboard.html", data)
}

// joinURL builds a steam://connect link from the game's query address
// (query port = game port + 1).
func joinURL(queryAddr string) string {
	host, port, err := net.SplitHostPort(queryAddr)
	if err != nil || host == "" {
		return ""
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 2 {
		return ""
	}
	return "steam://connect/" + net.JoinHostPort(host, strconv.Itoa(p-1))
}

func (d *Deps) loadGang(r *http.Request, playerID int64) *gangInfo {
	var gangID, leaderPlayerID int64
	var g gangInfo
	err := d.Pool.QueryRow(r.Context(), `
		SELECT g.id, g.name, g.tag, COALESCE(ga.balance, 0), g.leader_player_id
		FROM gang_members gm
		JOIN gangs g ON g.id = gm.gang_id
		LEFT JOIN gang_accounts ga ON ga.gang_id = g.id
		WHERE gm.player_id = $1
	`, playerID).Scan(&gangID, &g.Name, &g.Tag, &g.Balance, &leaderPlayerID)
	if err != nil {
		return nil
	}
	g.IsLeader = leaderPlayerID == playerID
	rows, err := d.Pool.Query(r.Context(), `
		SELECT p.id, p.name, gm.rank FROM gang_members gm
		JOIN players p ON p.id = gm.player_id
		WHERE gm.gang_id = $1 ORDER BY gm.rank DESC, p.name
	`, gangID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var m gangMember
			if rows.Scan(&m.PlayerID, &m.Name, &m.Rank) == nil {
				g.Members = append(g.Members, m)
			}
		}
	}
	return &g
}

// GenerateDiscordLinkCode issues a one-time code for the Discord `/link`
// command (the mirror-image of DiscordConnect's OAuth path) and re-renders
// the dashboard with it shown. POST, not GET -- it has a side effect (a new
// row in discord_link_codes). Re-rendered inline rather than redirected: a
// redirect would put the one-time code in browser history.
func (d *Deps) GenerateDiscordLinkCode(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	code, _, err := auth.GenerateLinkCode(r.Context(), d.Pool, sess.PlayerID)
	if err != nil {
		slog.Error("dashboard: generating discord link code failed", "error", err)
		http.Redirect(w, r, "/dashboard?error="+errMsg("Couldn't generate a code, try again."), http.StatusSeeOther)
		return
	}
	d.renderDashboard(w, r, code)
}

// SetLeaderboardOptIn is the dashboard's "Show me on leaderboards" box.
func (d *Deps) SetLeaderboardOptIn(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	on := r.FormValue("opt_in") == "on"
	if err := dashboard.SetOptIn(r.Context(), d.Pool, sess.PlayerID, on); err != nil {
		slog.Error("dashboard: leaderboard opt-in failed", "error", err)
		http.Redirect(w, r, "/dashboard?error="+errMsg("Couldn't save that. Try again.")+"#leaderboards", http.StatusSeeOther)
		return
	}
	msg := "You're now hidden from the leaderboards."
	if on {
		msg = "You now appear on the leaderboards."
	}
	http.Redirect(w, r, "/dashboard?notice="+errMsg(msg)+"#leaderboards", http.StatusSeeOther)
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
