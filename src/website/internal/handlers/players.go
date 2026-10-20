package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"website/internal/audit"
	"website/internal/auth"
	"website/internal/cases"
	"website/internal/factions"
	"website/internal/lookup"
	"website/internal/players"
)

type playerLookupData struct {
	Base
	AdminShell
	Q        string
	Results  []players.Result
	P        *players.Profile
	Tab      string
	Initials string
	Query    url.Values

	Vehicles []players.Vehicle
	Houses   []lookup.HouseRow
	Wanted   players.Wanted
	Accounts []players.Account
	BankTx   []players.BankTx
	Inv      []players.Inventory
	Sess     players.SessionInfo
	Notes    []players.Note
	Licences []players.LicenceGroup
	Ban      *cases.Ban

	CanNotes       bool
	CanLicences    bool
	CanVehActions  bool
	CanBan         bool
	CanPermBan     bool
	CanLiftBan     bool
	BanReasons     []string
	BanDurations   []struct {
		Days  int
		Label string
	}
	VehLog   []players.VehicleEvent
	Audit    []players.AuditEvent

	CanVehicles   bool
	CanCompensate bool
	CanLarge      bool
	CanPolice     bool
	CanEMS        bool
	LargeLimit    string

	Points       []cases.Points
	ActivePoints int
	Cases        []cases.ListRow
	CanCases     bool
	CanOpenCase  bool
	CanRevoke    bool
}

func (d playerLookupData) Money(c int64) string { return players.Dollars(c) }

// PlayerLookup is /admin/players -- the layout plan's Player Lookup board:
// search results on the left, the selected player's profile on the right.
func (d *Deps) PlayerLookup(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	data := playerLookupData{
		Base: baseFrom(r, "Player Lookup"), AdminShell: d.adminShell(r, "players"),
		Q: q.Get("q"), Tab: "overview", Query: q, LargeLimit: players.Dollars(players.LargeCompensation),
	}
	var err error
	if data.Results, err = players.Search(r.Context(), d.Pool, data.Q); err != nil {
		slog.Error("player search failed", "error", err)
		http.Error(w, "Search failed.", http.StatusInternalServerError)
		return
	}

	id, _ := strconv.ParseInt(q.Get("id"), 10, 64)
	if id == 0 && len(data.Results) > 0 && data.Q != "" {
		id = data.Results[0].ID
	}
	if id > 0 {
		p, err := players.Get(r.Context(), d.Pool, id)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			slog.Error("player profile failed", "error", err)
		default:
			data.P = &p
			data.Initials = initials(p.Name)
		}
	}
	if data.P != nil {
		data.CanVehicles = d.can(r, "players.vehicles")
		data.CanCompensate = d.can(r, "players.compensate")
		data.CanLarge = d.can(r, "players.compensate_large")
		data.CanPolice = d.can(r, "players.edit_police")
		data.CanEMS = d.can(r, "players.edit_medic")
		data.CanCases = d.can(r, "cases.view")
		data.CanOpenCase = d.can(r, "cases.lead")
		data.CanRevoke = d.can(r, "cases.close")
		data.Points, data.ActivePoints = d.playerPoints(r.Context(), data.P.ID)
		data.Houses, _ = lookup.PlayerHouses(r.Context(), d.Pool, data.P.ID)
		data.Wanted, _ = players.WantedInfo(r.Context(), d.Pool, data.P.ID)
		data.CanNotes = d.can(r, "players.notes")
		data.CanLicences = d.can(r, "players.licences")
		data.CanVehActions = d.can(r, "players.vehicle_actions")
		data.CanBan = d.can(r, "bans.issue")
		data.CanPermBan = d.can(r, "bans.permanent")
		data.CanLiftBan = d.can(r, "bans.revoke")
		data.BanReasons, data.BanDurations = cases.BanReasons, cases.BanDurations
		data.Ban, _, _ = cases.ActiveBan(r.Context(), d.Pool, data.P.ID)
		if data.CanLicences {
			data.Licences, _ = players.LicenceGroups(r.Context(), d.Pool, data.P.ID)
		}
		if (data.CanBan || data.CanOpenCase) && !data.CanCases {
			data.Cases, _ = cases.PlayerCases(r.Context(), d.Pool, data.P.ID)
		}
		if data.CanCases {
			data.Cases, _ = cases.PlayerCases(r.Context(), d.Pool, data.P.ID)
		}
		switch q.Get("tab") {
		case "vehicles":
			if data.CanVehicles {
				data.Tab = "vehicles"
				data.Vehicles, data.VehLog, _ = players.Vehicles(r.Context(), d.Pool, data.P.ID)
			}
		case "audit":
			data.Tab = "audit"
			data.Audit, _ = players.Audit(r.Context(), d.Pool, data.P.ID)
		case "money":
			data.Tab = "money"
			data.Accounts, data.BankTx, _ = players.Money(r.Context(), d.Pool, data.P.ID)
		case "inventory":
			data.Tab = "inventory"
			data.Inv, _ = players.Inventories(r.Context(), d.Pool, data.P.ID)
		case "sessions":
			data.Tab = "sessions"
			data.Sess, _ = players.Sessions(r.Context(), d.Pool, data.P.ID)
		case "notes":
			if data.CanNotes {
				data.Tab = "notes"
				data.Notes, _ = players.Notes(r.Context(), d.Pool, data.P.ID)
			}
		}
	}
	d.Render.Render(w, "player_lookup.html", data)
}

func playerBack(r *http.Request, id int64) string {
	back := fmt.Sprintf("/admin/players?id=%d", id)
	if q := r.FormValue("q"); q != "" {
		back += "&q=" + url.QueryEscape(q)
	}
	return back
}

func finishPlayerAction(w http.ResponseWriter, r *http.Request, back string, err error, success string) {
	switch {
	case err == nil:
		http.Redirect(w, r, back+"&notice="+errMsg(success), http.StatusSeeOther)
	case errors.Is(err, players.ErrNotAllowed):
		msg := strings.TrimPrefix(err.Error(), players.ErrNotAllowed.Error()+": ")
		http.Redirect(w, r, back+"&error="+errMsg(strings.ToUpper(msg[:1])+msg[1:]+"."), http.StatusSeeOther)
	default:
		slog.Error("player action failed", "path", r.URL.Path, "error", err)
		http.Redirect(w, r, back+"&error="+errMsg("Something went wrong. Nothing was changed."), http.StatusSeeOther)
	}
}

func playerActor(r *http.Request) players.Actor {
	sess, _ := auth.FromContext(r.Context())
	return players.Actor{PlayerID: sess.PlayerID, Source: audit.SourceWebsite}
}

func (d *Deps) PlayerSetFactionLevel(w http.ResponseWriter, r *http.Request) {
	id, ok := profileID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	back := playerBack(r, id)
	faction := r.FormValue("faction")
	perm := map[string]string{"police": "players.edit_police", "ems": "players.edit_medic"}[faction]
	if perm == "" || !d.can(r, perm) {
		http.Redirect(w, r, back+"&error="+errMsg("You don't have permission to do that."), http.StatusSeeOther)
		return
	}
	level, err := strconv.Atoi(r.FormValue("level"))
	if err != nil {
		http.Redirect(w, r, back+"&error="+errMsg("Enter a level number."), http.StatusSeeOther)
		return
	}
	sess, _ := auth.FromContext(r.Context())
	ch, err := factions.SetLevel(r.Context(), d.Pool, factions.Actor{PlayerID: sess.PlayerID, Source: audit.SourceWebsite, Via: factions.ViaStaffOverride},
		faction, id, level, r.FormValue("reason"))
	if errors.Is(err, factions.ErrNotAllowed) {
		err = fmt.Errorf("%w: %s", players.ErrNotAllowed, strings.TrimPrefix(err.Error(), factions.ErrNotAllowed.Error()+": "))
	}
	msg := "Faction rank changed. It's in the faction's Command log as a staff override."
	if ch.OwnFaction {
		msg = "Faction rank changed. You're in this faction yourself, so it's flagged \"own faction\" in the Command log and Staff Log."
	}
	finishPlayerAction(w, r, back, err, msg)
}

func (d *Deps) PlayerCompensate(w http.ResponseWriter, r *http.Request) {
	id, ok := profileID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	back := playerBack(r, id)
	if !d.can(r, "players.compensate") {
		http.Redirect(w, r, back+"&error="+errMsg("You don't have permission to do that."), http.StatusSeeOther)
		return
	}
	// In-game money is whole dollars, so "1,500" and "$1500" are fine but
	// "12.50" isn't.
	amount, err := strconv.ParseInt(strings.ReplaceAll(strings.TrimPrefix(strings.TrimSpace(r.FormValue("amount")), "$"), ",", ""), 10, 64)
	if err != nil || amount <= 0 {
		http.Redirect(w, r, back+"&error="+errMsg("Enter a whole dollar amount greater than zero."), http.StatusSeeOther)
		return
	}
	if amount > players.LargeCompensation && !d.can(r, "players.compensate_large") {
		http.Redirect(w, r, back+"&error="+errMsg("Amounts over $"+players.Dollars(players.LargeCompensation)+" need players.compensate_large."), http.StatusSeeOther)
		return
	}
	err = players.Compensate(r.Context(), d.Pool, playerActor(r), id, r.FormValue("account"), amount, r.FormValue("reason"))
	finishPlayerAction(w, r, back, err, "Compensation of $"+players.Dollars(amount)+" added.")
}

// PlayerLicences sets one faction's licences (players.licences; offline
// players only -- see players.SetLicences).
func (d *Deps) PlayerLicences(w http.ResponseWriter, r *http.Request) {
	id, ok := profileID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	back := playerBack(r, id)
	if !d.can(r, "players.licences") {
		http.Redirect(w, r, back+"&error="+errMsg("You don't have permission to do that."), http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form.", http.StatusBadRequest)
		return
	}
	added, removed, err := players.SetLicences(r.Context(), d.Pool, playerActor(r), id, r.FormValue("faction"), r.Form["licence"])
	finishPlayerAction(w, r, back, err, fmt.Sprintf("Licences updated: %d given, %d taken. It's in the Staff Log.", len(added), len(removed)))
}

// PlayerAddNote adds a staff note about the player (players.notes).
func (d *Deps) PlayerAddNote(w http.ResponseWriter, r *http.Request) {
	id, ok := profileID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	back := playerBack(r, id) + "&tab=notes"
	if !d.can(r, "players.notes") {
		http.Redirect(w, r, back+"&error="+errMsg("You don't have permission to do that."), http.StatusSeeOther)
		return
	}
	err := players.AddNote(r.Context(), d.Pool, playerActor(r), id, r.FormValue("body"))
	finishPlayerAction(w, r, back, err, "Note added.")
}

// VehicleActionPost releases, destroys or restores a vehicle
// (players.vehicle_actions), from Vehicle Lookup or Player Lookup.
func (d *Deps) VehicleActionPost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	back := r.FormValue("back")
	if !strings.HasPrefix(back, "/admin/") || strings.HasPrefix(back, "//") {
		back = fmt.Sprintf("/admin/vehicles?id=%d", id)
	}
	if !strings.Contains(back, "?") {
		back += "?"
	}
	if !d.can(r, "players.vehicle_actions") {
		http.Redirect(w, r, back+"&error="+errMsg("You don't have permission to do that."), http.StatusSeeOther)
		return
	}
	sess, _ := auth.FromContext(r.Context())
	name, err := lookup.VehicleAction(r.Context(), d.Pool, lookup.Actor{PlayerID: sess.PlayerID, Source: audit.SourceWebsite}, id, r.FormValue("action"), r.FormValue("reason"))
	if errors.Is(err, lookup.ErrNotAllowed) {
		err = fmt.Errorf("%w: %s", players.ErrNotAllowed, strings.TrimPrefix(err.Error(), lookup.ErrNotAllowed.Error()+": "))
	}
	finishPlayerAction(w, r, back, err, name+": "+strings.ToLower(lookup.ActionLabel(r.FormValue("action")))+" done. It's in the Staff Log.")
}

// hoursMinutes renders a playtime duration like "142h 10m".
func hoursMinutes(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

// Signed is an amount with its sign before the dollar sign: "+$500", "-$500".
func (d playerLookupData) Signed(c int64) string {
	if c < 0 {
		return "-$" + players.Dollars(-c)
	}
	return "+$" + players.Dollars(c)
}

// Dur is a duration like "3h 20m" (jail time, sessions).
func (d playerLookupData) Dur(t time.Duration) string { return hoursMinutes(t) }

func (d playerLookupData) VehActions(status string) []string { return lookup.ActionsFor(status) }
func (d playerLookupData) VehActionLabel(a string) string  { return lookup.ActionLabel(a) }

func (d playerLookupData) Playtime() string {
	if d.P == nil {
		return ""
	}
	return hoursMinutes(d.P.Playtime)
}
