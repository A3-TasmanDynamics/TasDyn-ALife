package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"website/internal/audit"
	"website/internal/auth"
	"website/internal/cases"
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
	Teams     []staffTeam
	CanAdd    bool
	Ranks     []staff.Rank
	Total     int
	Active    int
	LOA       int
	Suspended int
	Filter    string // "", "active", "loa", "suspended"
	Pills     []staffPill
}

type staffTeam struct {
	Name    string
	Count   int // whole team, before the status filter
	Members []staffRow
}

type staffRow struct {
	staff.Member
	Initials   string
	StatusNote string // e.g. "Back 12 Oct · exams"
}

type staffPill struct {
	Label, Value string
	Active       bool
}

// statusNote renders the short "why / until when" shown beside a non-active
// staff member, e.g. "Back 12 Oct · exams" or "Since 22 Sep · lifted by hand".
func statusNote(m staff.Member) string {
	switch m.Status {
	case "loa":
		note := "On leave"
		if m.StatusUntil != nil {
			note = "Back " + m.StatusUntil.Format("2 Jan")
		}
		if m.StatusReason != "" {
			note += " · " + m.StatusReason
		}
		return note
	case "suspended":
		note := "Lifted by hand"
		if m.StatusReason != "" {
			note = m.StatusReason + " · " + note
		}
		return note
	}
	return ""
}

func initials(name string) string {
	// Unnamed players display as "Player #35" -- show "#35", not "PL".
	if rest, ok := strings.CutPrefix(name, "Player #"); ok {
		return "#" + rest
	}
	r := []rune(strings.TrimSpace(name))
	if len(r) > 2 {
		r = r[:2]
	}
	return strings.ToUpper(string(r))
}

// StaffList is /admin/staff: every staff member grouped by team (with an
// "Unassigned" group), filterable by status -- the layout plan's Staff
// Directory board.
func (d *Deps) StaffList(w http.ResponseWriter, r *http.Request) {
	data := staffListData{Base: baseFrom(r, "Staff Directory"), AdminShell: d.adminShell(r, "staff")}
	switch f := r.URL.Query().Get("status"); f {
	case "active", "loa", "suspended":
		data.Filter = f
	}
	for _, p := range []staffPill{{"All", "", false}, {"Active", "active", false}, {"LOA", "loa", false}, {"Suspended", "suspended", false}} {
		p.Active = p.Value == data.Filter
		data.Pills = append(data.Pills, p)
	}

	members, err := staff.List(r.Context(), d.Pool)
	if err != nil {
		slog.Error("staff list failed", "error", err)
		http.Error(w, "Failed to load staff.", http.StatusInternalServerError)
		return
	}
	teams := map[string]*staffTeam{}
	var names []string
	for _, m := range members {
		switch m.Status {
		case "active":
			data.Active++
		case "loa":
			data.LOA++
		case "suspended":
			data.Suspended++
		}
		name := m.Team
		if name == "" {
			name = "Unassigned"
		}
		t, ok := teams[name]
		if !ok {
			t = &staffTeam{Name: name}
			teams[name] = t
			if name != "Unassigned" {
				names = append(names, name)
			}
		}
		t.Count++
		if data.Filter == "" || m.Status == data.Filter {
			t.Members = append(t.Members, staffRow{Member: m, Initials: initials(m.Name), StatusNote: statusNote(m)})
		}
	}
	sort.Strings(names)
	if _, ok := teams["Unassigned"]; ok {
		names = append(names, "Unassigned")
	}
	for _, n := range names {
		if len(teams[n].Members) > 0 {
			data.Teams = append(data.Teams, *teams[n])
		}
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
	Member     staff.Member
	Extras     staff.Extras
	Initials   string
	StatusNote string
	IsSelf     bool
	Ranks      []staff.Rank
	Teams      []string
	Regions    []string
	Timeline   []staff.TimelineEntry

	CanPlace   bool // rank, team and region all need staff.edit
	CanRank    bool
	CanTeam    bool
	CanRemove  bool
	CanLOA     bool
	CanSuspend bool
	CanNotes   bool
	Today      string

	Activity   cases.Activity // case activity (GAMEPANEL_PARITY §3.4)
	CanCases   bool
}

func profileID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id, err == nil && id > 0
}

// StaffProfile is /admin/staff/{id} -- the layout plan's Staff profile
// board: header with status actions, placement, notes & history, removal.
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
		Initials: initials(m.Name), StatusNote: statusNote(m),
		IsSelf: sess.PlayerID == id, Today: time.Now().Format("2006-01-02"),
	}
	data.Extras, _ = staff.ProfileExtras(r.Context(), d.Pool, id)
	if !data.IsSelf {
		isStaff := m.RankID != 0
		data.CanRank = d.can(r, "staff.edit")
		data.CanTeam = isStaff && data.CanRank
		data.CanPlace = data.CanRank || data.CanTeam
		data.CanRemove = isStaff && d.can(r, "staff.remove")
		data.CanLOA = isStaff && m.Status != "suspended" && d.can(r, "staff.loa")
		data.CanSuspend = isStaff && d.can(r, "staff.suspend")
		data.CanNotes = d.can(r, "staff.notes")
	}
	if data.CanRank {
		data.Ranks, _ = staff.Ranks(r.Context(), d.Pool)
	}
	if data.CanTeam {
		data.Teams, data.Regions, _ = staff.Suggestions(r.Context(), d.Pool)
	}
	if data.CanNotes {
		data.Timeline, _ = staff.Timeline(r.Context(), d.Pool, id)
	} else {
		// Without notes access, still show the rank/status history.
		all, _ := staff.Timeline(r.Context(), d.Pool, id)
		for _, e := range all {
			if e.Kind == "status" {
				data.Timeline = append(data.Timeline, e)
			}
		}
	}
	if data.CanCases = d.can(r, "cases.view"); data.CanCases {
		data.Activity, _ = cases.StaffActivity(r.Context(), d.Pool, data.Member.ID)
	}
	d.Render.Render(w, "staff_profile.html", data)
}

// StaffSetPlacement saves the Placement card: rank, team and region in one
// form. Each part is only applied if it changed, and each is checked
// against its own permission; a rank change needs a reason.
func (d *Deps) StaffSetPlacement(w http.ResponseWriter, r *http.Request) {
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
	var done []string

	if rankID, _ := strconv.Atoi(r.FormValue("rank_id")); rankID != 0 && rankID != m.RankID {
		if !d.requireCan(w, r, "staff.edit", back) {
			return
		}
		if err := staff.SetRank(r.Context(), d.Pool, actorFrom(r), id, rankID, r.FormValue("reason")); err != nil {
			finishStaffAction(w, r, back, err, "")
			return
		}
		done = append(done, "rank")
		m.RankID = rankID
	}

	team, region := strings.TrimSpace(r.FormValue("team")), strings.TrimSpace(r.FormValue("region"))
	if r.Form.Has("team") && m.RankID != 0 && (team != m.Team || region != m.Region) {
		if !d.requireCan(w, r, "staff.edit", back) {
			return
		}
		if err := staff.SetTeam(r.Context(), d.Pool, actorFrom(r), id, team, region); err != nil {
			finishStaffAction(w, r, back, err, "")
			return
		}
		done = append(done, "team/region")
	}

	if len(done) == 0 {
		http.Redirect(w, r, back+"?notice="+errMsg("Nothing changed."), http.StatusSeeOther)
		return
	}
	finishStaffAction(w, r, back, nil, "Updated "+strings.Join(done, ", ")+".")
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
