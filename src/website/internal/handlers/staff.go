package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"website/internal/audit"
	"website/internal/auth"
	"website/internal/staff"
)

// can is a best-effort permission check for deciding which controls to
// show. It never replaces the check on the action itself.
func (d *Deps) can(r *http.Request, key string) bool {
	sess, ok := auth.FromContext(r.Context())
	if !ok {
		return false
	}
	denial, err := auth.Can(r.Context(), d.Pool, sess.PlayerID, key)
	return err == nil && denial == auth.Allowed
}

// requireCan checks key for an action, writing a redirect with an error
// message and returning false when it's not allowed.
func (d *Deps) requireCan(w http.ResponseWriter, r *http.Request, key, back string) bool {
	if d.can(r, key) {
		return true
	}
	http.Redirect(w, r, back+"?error="+errMsg("You don't have permission to do that."), http.StatusSeeOther)
	return false
}

func actorFrom(r *http.Request) staff.Actor {
	sess, _ := auth.FromContext(r.Context())
	return staff.Actor{PlayerID: sess.PlayerID, Source: audit.SourceWebsite}
}

// finishStaffAction redirects back to the profile with a notice, or with
// the rule that was broken, or logs an unexpected error.
func finishStaffAction(w http.ResponseWriter, r *http.Request, back string, err error, success string) {
	switch {
	case err == nil:
		http.Redirect(w, r, back+"?notice="+errMsg(success), http.StatusSeeOther)
	case errors.Is(err, staff.ErrNotAllowed):
		msg := strings.TrimPrefix(err.Error(), staff.ErrNotAllowed.Error()+": ")
		http.Redirect(w, r, back+"?error="+errMsg(strings.ToUpper(msg[:1])+msg[1:]+"."), http.StatusSeeOther)
	default:
		slog.Error("staff action failed", "path", r.URL.Path, "error", err)
		http.Redirect(w, r, back+"?error="+errMsg("Something went wrong. Nothing was changed."), http.StatusSeeOther)
	}
}

type staffListData struct {
	Base
	AdminShell
	Teams    []staffTeam
	CanAdd   bool
	Ranks    []staff.Rank
	Total    int
	OnLeave  int
	Unlinked int
}

type staffTeam struct {
	Name    string
	Members []staff.Member
}

// StaffList is /admin/staff: every staff member grouped by team, with an
// "Unassigned" group (Gamepanel's team overview, GAMEPANEL_PARITY §2.2).
func (d *Deps) StaffList(w http.ResponseWriter, r *http.Request) {
	data := staffListData{Base: baseFrom(r, "Staff"), AdminShell: d.adminShell(r, "staff")}
	members, err := staff.List(r.Context(), d.Pool)
	if err != nil {
		slog.Error("staff list failed", "error", err)
		http.Error(w, "Failed to load staff.", http.StatusInternalServerError)
		return
	}
	byTeam := map[string][]staff.Member{}
	var order []string
	for _, m := range members {
		team := m.Team
		if team == "" {
			team = "Unassigned"
		}
		if _, seen := byTeam[team]; !seen {
			order = append(order, team)
		}
		byTeam[team] = append(byTeam[team], m)
		if m.Status != "active" {
			data.OnLeave++
		}
		if m.DiscordUsername == "" {
			data.Unlinked++
		}
	}
	for _, t := range order {
		if t != "Unassigned" {
			data.Teams = append(data.Teams, staffTeam{Name: t, Members: byTeam[t]})
		}
	}
	if u, ok := byTeam["Unassigned"]; ok {
		data.Teams = append(data.Teams, staffTeam{Name: "Unassigned", Members: u})
	}
	data.Total = len(members)

	if data.CanAdd = d.can(r, "staff.edit"); data.CanAdd {
		data.Ranks, _ = staff.Ranks(r.Context(), d.Pool)
	}
	d.Render.Render(w, "staff_list.html", data)
}

// StaffAdd adds a player to staff by Steam64 ID. The player row is created
// if they've never signed in (the same path as a first Steam login).
func (d *Deps) StaffAdd(w http.ResponseWriter, r *http.Request) {
	const back = "/admin/staff"
	if !d.requireCan(w, r, "staff.edit", back) {
		return
	}
	uid := strings.TrimSpace(r.FormValue("steam_id"))
	if len(uid) != 17 || strings.Trim(uid, "0123456789") != "" {
		http.Redirect(w, r, back+"?error="+errMsg("Enter a 17-digit Steam64 ID."), http.StatusSeeOther)
		return
	}
	rankID, _ := strconv.Atoi(r.FormValue("rank_id"))
	if rankID == 0 {
		http.Redirect(w, r, back+"?error="+errMsg("Choose a rank."), http.StatusSeeOther)
		return
	}
	playerID, err := auth.FindOrCreatePlayerBySteamUID(r.Context(), d.Pool, uid)
	if err != nil {
		finishStaffAction(w, r, back, err, "")
		return
	}
	err = staff.SetRank(r.Context(), d.Pool, actorFrom(r), playerID, rankID, r.FormValue("reason"))
	finishStaffAction(w, r, fmt.Sprintf("/admin/staff/%d", playerID), err, "Added to staff.")
}

type staffProfileData struct {
	Base
	AdminShell
	Member  staff.Member
	IsSelf  bool
	Ranks   []staff.Rank
	Notes   []staff.Note
	History []staff.Change

	CanEditRank bool
	CanRemove   bool
	CanLOA      bool
	CanSuspend  bool
	CanTeam     bool
	CanNotes    bool
	Today       string
}

func profileID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id, err == nil && id > 0
}

// StaffProfile is /admin/staff/{id}.
func (d *Deps) StaffProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := profileID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	m, err := staff.Get(r.Context(), d.Pool, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess, _ := auth.FromContext(r.Context())
	data := staffProfileData{
		Base: baseFrom(r, m.Name), AdminShell: d.adminShell(r, "staff"), Member: m,
		IsSelf: sess.PlayerID == id, Today: time.Now().Format("2006-01-02"),
	}
	if !data.IsSelf {
		data.CanEditRank = d.can(r, "staff.edit")
		data.CanRemove = m.RankID != 0 && d.can(r, "staff.remove")
		data.CanLOA = m.RankID != 0 && m.Status != "suspended" && d.can(r, "staff.loa")
		data.CanSuspend = m.RankID != 0 && d.can(r, "staff.suspend")
		data.CanTeam = m.RankID != 0 && d.can(r, "staff.team")
		data.CanNotes = d.can(r, "staff.notes")
	}
	if data.CanEditRank {
		data.Ranks, _ = staff.Ranks(r.Context(), d.Pool)
	}
	if data.CanNotes {
		data.Notes, _ = staff.Notes(r.Context(), d.Pool, id)
	}
	data.History, _ = staff.History(r.Context(), d.Pool, id)
	d.Render.Render(w, "staff_profile.html", data)
}

func (d *Deps) StaffSetRank(w http.ResponseWriter, r *http.Request) {
	id, ok := profileID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	back := fmt.Sprintf("/admin/staff/%d", id)
	rankID, _ := strconv.Atoi(r.FormValue("rank_id"))
	perm := "staff.edit"
	if rankID == 0 {
		perm = "staff.remove"
	}
	if !d.requireCan(w, r, perm, back) {
		return
	}
	err := staff.SetRank(r.Context(), d.Pool, actorFrom(r), id, rankID, r.FormValue("reason"))
	msg := "Rank updated."
	if rankID == 0 {
		msg = "Removed from staff."
	}
	finishStaffAction(w, r, back, err, msg)
}

func (d *Deps) StaffSetStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := profileID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	back := fmt.Sprintf("/admin/staff/%d", id)
	m, err := staff.Get(r.Context(), d.Pool, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	next := r.FormValue("status")
	if !d.requireCan(w, r, staff.RequiredStatusPermission(m.Status, next), back) {
		return
	}
	var until *time.Time
	if s := strings.TrimSpace(r.FormValue("until")); s != "" {
		t, err := time.ParseInLocation("2006-01-02", s, time.Local)
		if err != nil {
			http.Redirect(w, r, back+"?error="+errMsg("Invalid end date."), http.StatusSeeOther)
			return
		}
		t = t.Add(24*time.Hour - time.Second) // through the end of that day
		until = &t
	}
	err = staff.SetStatus(r.Context(), d.Pool, actorFrom(r), id, next, r.FormValue("reason"), until)
	finishStaffAction(w, r, back, err, map[string]string{
		"active": "Reinstated.", "loa": "Placed on leave.", "suspended": "Suspended.",
	}[next])
}

func (d *Deps) StaffSetTeam(w http.ResponseWriter, r *http.Request) {
	id, ok := profileID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	back := fmt.Sprintf("/admin/staff/%d", id)
	if !d.requireCan(w, r, "staff.team", back) {
		return
	}
	err := staff.SetTeam(r.Context(), d.Pool, actorFrom(r), id, r.FormValue("team"), r.FormValue("region"))
	finishStaffAction(w, r, back, err, "Team updated.")
}

func (d *Deps) StaffAddNote(w http.ResponseWriter, r *http.Request) {
	id, ok := profileID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	back := fmt.Sprintf("/admin/staff/%d", id)
	if !d.requireCan(w, r, "staff.notes", back) {
		return
	}
	err := staff.AddNote(r.Context(), d.Pool, actorFrom(r), id, r.FormValue("kind"), r.FormValue("body"))
	finishStaffAction(w, r, back, err, "Note added.")
}
