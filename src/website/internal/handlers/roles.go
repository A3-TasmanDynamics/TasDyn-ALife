package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"website/internal/auth"
	"website/internal/roles"
)

type rolesData struct {
	Base
	AdminShell
	Tab       string // "ranks" or "faction"
	Ranks     []rankRow
	Selected  *rankRow
	Groups    []permGroupView
	MyLevel   int
	NextLevel int // a free level suggestion for "New rank"

	CanRanks   bool
	CanFaction bool
	Police     []string
	EMS        []string
}

type rankRow struct {
	roles.Rank
	Locked    bool
	PermCount int
	CanUp     bool
	CanDown   bool
	CanDelete bool
}

type permGroupView struct {
	Name  string
	Perms []permView
}

type permView struct {
	auth.Permission
	On       bool
	Grantable bool // actor holds it, so may tick it
}

func finishRolesAction(w http.ResponseWriter, r *http.Request, back string, err error, success string) {
	switch {
	case err == nil:
		http.Redirect(w, r, back+sep(back)+"notice="+errMsg(success), http.StatusSeeOther)
	case errors.Is(err, roles.ErrNotAllowed):
		msg := strings.TrimPrefix(err.Error(), roles.ErrNotAllowed.Error()+": ")
		http.Redirect(w, r, back+sep(back)+"error="+errMsg(strings.ToUpper(msg[:1])+msg[1:]+"."), http.StatusSeeOther)
	default:
		slog.Error("roles action failed", "path", r.URL.Path, "error", err)
		http.Redirect(w, r, back+sep(back)+"error="+errMsg("Something went wrong. Nothing was changed."), http.StatusSeeOther)
	}
}

func sep(url string) string {
	if strings.Contains(url, "?") {
		return "&"
	}
	return "?"
}

// Roles is /admin/roles -- the layout plan's Roles & Permissions board:
// a "Staff ranks" tab (rank list + permission editor) and a "Faction rank
// names" tab.
func (d *Deps) Roles(w http.ResponseWriter, r *http.Request) {
	data := rolesData{Base: baseFrom(r, "Roles & Permissions"), AdminShell: d.adminShell(r, "roles"), Tab: "ranks"}
	if r.URL.Query().Get("tab") == "faction" {
		data.Tab = "faction"
	}
	sess, _ := auth.FromContext(r.Context())
	myLevel, err := roles.ActorLevel(r.Context(), d.Pool, sess.PlayerID)
	if err != nil {
		http.Error(w, "Failed to load roles.", http.StatusInternalServerError)
		return
	}
	data.MyLevel = myLevel
	data.CanFaction = d.can(r, "factions.configure")
	// The page opens for either permission; the ranks tab needs roles.manage.
	data.CanRanks = d.can(r, "roles.manage")
	if !data.CanRanks && !data.CanFaction {
		d.Denied(w, r, auth.DeniedInfo{Kind: "no_permission", Area: "admin", Permission: "Edit roles & permissions"})
		return
	}
	if !data.CanRanks {
		data.Tab = "faction"
	}

	list, err := roles.List(r.Context(), d.Pool)
	if err != nil {
		slog.Error("roles list failed", "error", err)
		http.Error(w, "Failed to load roles.", http.StatusInternalServerError)
		return
	}
	selID, _ := strconv.Atoi(r.URL.Query().Get("rank"))
	used := map[int]bool{}
	for i, rk := range list {
		used[rk.Level] = true
		row := rankRow{Rank: rk, Locked: rk.Level >= myLevel, PermCount: len(rk.Perms)}
		row.CanUp = !row.Locked && i > 0 && list[i-1].Level < myLevel
		row.CanDown = !row.Locked && i < len(list)-1
		row.CanDelete = !row.Locked && rk.Members == 0
		data.Ranks = append(data.Ranks, row)
	}
	for i := range data.Ranks {
		if data.Ranks[i].ID == selID || (selID == 0 && data.Selected == nil && !data.Ranks[i].Locked) {
			data.Selected = &data.Ranks[i]
		}
	}
	if data.Selected == nil && len(data.Ranks) > 0 {
		data.Selected = &data.Ranks[0]
	}
	for l := myLevel - 1; l > 0; l-- {
		if !used[l] {
			data.NextLevel = l
			break
		}
	}

	if data.Selected != nil {
		for _, g := range auth.CatalogueGroups() {
			gv := permGroupView{Name: g.Name}
			for _, p := range g.Perms {
				gv.Perms = append(gv.Perms, permView{Permission: p, On: data.Selected.Perms[p.Key], Grantable: d.can(r, p.Key)})
			}
			data.Groups = append(data.Groups, gv)
		}
	}

	if data.Tab == "faction" {
		names, err := roles.FactionNames(r.Context(), d.Pool)
		if err == nil {
			data.Police, data.EMS = names["police"], names["ems"]
		}
	}
	d.Render.Render(w, "roles.html", data)
}

func rankBack(id int) string { return fmt.Sprintf("/admin/roles?rank=%d", id) }

func (d *Deps) RoleSave(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.FormValue("rank_id"))
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form", http.StatusBadRequest)
		return
	}
	sess, _ := auth.FromContext(r.Context())
	err := roles.Update(r.Context(), d.Pool, sess.PlayerID, id, r.FormValue("name"),
		r.FormValue("admin_panel") == "on", r.FormValue("support_panel") == "on", r.Form["perm"])
	finishRolesAction(w, r, rankBack(id), err, "Rank saved. Staff pick up panel changes at their next sign-in; permissions apply immediately.")
}

func (d *Deps) RoleMove(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.FormValue("rank_id"))
	sess, _ := auth.FromContext(r.Context())
	err := roles.Move(r.Context(), d.Pool, sess.PlayerID, id, r.FormValue("dir") == "up")
	finishRolesAction(w, r, rankBack(id), err, "Rank moved.")
}

func (d *Deps) RoleCreate(w http.ResponseWriter, r *http.Request) {
	level, _ := strconv.Atoi(r.FormValue("level"))
	sess, _ := auth.FromContext(r.Context())
	id, err := roles.Create(r.Context(), d.Pool, sess.PlayerID, r.FormValue("name"), level)
	back := "/admin/roles"
	if err == nil {
		back = rankBack(id)
	}
	finishRolesAction(w, r, back, err, "Rank created. It has no permissions yet.")
}

func (d *Deps) RoleDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.FormValue("rank_id"))
	sess, _ := auth.FromContext(r.Context())
	err := roles.Delete(r.Context(), d.Pool, sess.PlayerID, id)
	back := rankBack(id)
	if err == nil {
		back = "/admin/roles"
	}
	finishRolesAction(w, r, back, err, "Rank deleted.")
}

func (d *Deps) RoleFactionNames(w http.ResponseWriter, r *http.Request) {
	const back = "/admin/roles?tab=faction"
	if !d.can(r, "factions.configure") {
		http.Redirect(w, r, back+"&error="+errMsg("You don't have permission to do that."), http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form", http.StatusBadRequest)
		return
	}
	faction := r.FormValue("faction")
	sess, _ := auth.FromContext(r.Context())
	err := roles.SaveFactionNames(r.Context(), d.Pool, sess.PlayerID, faction, r.Form["name"])
	finishRolesAction(w, r, back, err, "Rank names saved.")
}
