package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"website/internal/auth"
	"website/internal/discord"
	"website/internal/rolesync"
)

type roleSyncData struct {
	Base
	AdminShell
	BotRunning     bool
	BotError       string
	CanManageRoles bool
	Roles          []discord.GuildRole
	Groups         []mapGroupView
	Mapped         int
	Log            []roleSyncLogView
	Failures24h    int
	Running        bool
}

type mapGroupView struct {
	Name string
	Rows []mapRowView
}

type mapRowView struct {
	rolesync.Option
	// Selected holds the currently mapped role IDs, always at least one
	// entry ("" = not mapped) so the row renders a select.
	Selected []string
	// Missing are mapped role IDs that no longer exist in the guild.
	Missing map[string]bool
}

type roleSyncLogView struct {
	rolesync.LogEntry
	GroupName string
}

// roleSyncRunning stops two "Sync everyone now" passes overlapping.
var roleSyncRunning atomic.Bool

// RoleSync is /admin/role-sync: which Discord role each rank, team or
// faction level gets (docs/INTEGRATIONS.md §2.3). Only mapped roles are
// ever touched by sync.
func (d *Deps) RoleSync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := roleSyncData{Base: baseFrom(r, "Role Sync"), AdminShell: d.adminShell(r, "rolesync"), Running: roleSyncRunning.Load()}

	names := map[string]string{}
	if d.Bot == nil {
		data.BotError = "The Discord bot isn't running (DISCORD_BOT_TOKEN not set, or it failed to start), so roles can't be listed or synced."
	} else if roles, err := d.Bot.GuildRoles(); err != nil {
		slog.Error("role sync: listing guild roles failed", "error", err)
		data.BotError = "Couldn't read the Discord server's roles: " + err.Error()
	} else {
		data.BotRunning = true
		data.Roles = roles
		data.CanManageRoles = d.Bot.CanManageRoles()
		for _, gr := range roles {
			names[gr.ID] = gr.Name
		}
	}

	opts, err := rolesync.Options(ctx, d.Pool)
	if err != nil {
		slog.Error("role sync: options failed", "error", err)
		http.Error(w, "Failed to load role sync.", http.StatusInternalServerError)
		return
	}
	maps, err := rolesync.Mappings(ctx, d.Pool, "discord")
	if err != nil {
		slog.Error("role sync: mappings failed", "error", err)
		http.Error(w, "Failed to load role sync.", http.StatusInternalServerError)
		return
	}
	data.Mapped = len(maps)
	byEnt := map[string][]string{}
	for _, m := range maps {
		byEnt[m.Entitlement] = append(byEnt[m.Entitlement], m.GroupID)
	}
	for _, o := range opts {
		row := mapRowView{Option: o, Selected: byEnt[o.Key], Missing: map[string]bool{}}
		if len(row.Selected) == 0 {
			row.Selected = []string{""}
		}
		for _, id := range row.Selected {
			if id != "" && data.BotRunning && names[id] == "" {
				row.Missing[id] = true
			}
		}
		if n := len(data.Groups); n == 0 || data.Groups[n-1].Name != o.Group {
			data.Groups = append(data.Groups, mapGroupView{Name: o.Group})
		}
		g := &data.Groups[len(data.Groups)-1]
		g.Rows = append(g.Rows, row)
	}

	entries, failures, err := rolesync.Recent(ctx, d.Pool, 25)
	if err != nil {
		slog.Error("role sync: recent log failed", "error", err)
	}
	data.Failures24h = failures
	for _, e := range entries {
		v := roleSyncLogView{LogEntry: e, GroupName: names[e.Group]}
		if v.GroupName == "" {
			v.GroupName = e.Group
		}
		data.Log = append(data.Log, v)
	}
	d.Render.Render(w, "role_sync.html", data)
}

// RoleSyncSave replaces the Discord mappings with the submitted form.
// Only known entitlements and roles the bot can assign are accepted (a role
// that was already mapped may be kept even if it's no longer assignable).
func (d *Deps) RoleSyncSave(w http.ResponseWriter, r *http.Request) {
	const back = "/admin/role-sync"
	if d.Bot == nil {
		http.Redirect(w, r, back+"?error="+errMsg("The Discord bot isn't running."), http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	guildRoles, err := d.Bot.GuildRoles()
	if err != nil {
		http.Redirect(w, r, back+"?error="+errMsg("Couldn't read the Discord server's roles, so nothing was saved."), http.StatusSeeOther)
		return
	}
	assignable := map[string]bool{}
	for _, gr := range guildRoles {
		assignable[gr.ID] = gr.Assignable
	}
	opts, err := rolesync.Options(ctx, d.Pool)
	if err != nil {
		finishRolesAction(w, r, back, err, "")
		return
	}
	existing, err := rolesync.Mappings(ctx, d.Pool, "discord")
	if err != nil {
		finishRolesAction(w, r, back, err, "")
		return
	}
	wasMapped := map[string]bool{}
	for _, m := range existing {
		wasMapped[m.Entitlement+"\x00"+m.GroupID] = true
	}

	m := map[string][]string{}
	for _, o := range opts {
		seen := map[string]bool{}
		for _, id := range r.Form["map:"+o.Key] {
			id = strings.TrimSpace(id)
			if id == "" || seen[id] {
				continue
			}
			if !assignable[id] && !wasMapped[o.Key+"\x00"+id] {
				http.Redirect(w, r, back+"?error="+errMsg("One of the chosen roles can't be assigned by the bot. Nothing was saved."), http.StatusSeeOther)
				return
			}
			seen[id] = true
			m[o.Key] = append(m[o.Key], id)
		}
	}
	sess, _ := auth.FromContext(ctx)
	err = rolesync.SaveMappings(ctx, d.Pool, sess.PlayerID, "discord", m)
	finishRolesAction(w, r, back, err, "Role mapping saved. Linked players are updated on their next change, or use \"Sync everyone now\".")
}

// RoleSyncRun starts a full pass in the background; results appear in the
// sync log on the page.
func (d *Deps) RoleSyncRun(w http.ResponseWriter, r *http.Request) {
	const back = "/admin/role-sync"
	if d.RoleSyncEngine == nil {
		http.Redirect(w, r, back+"?error="+errMsg("Role sync isn't running (the Discord bot is offline)."), http.StatusSeeOther)
		return
	}
	if !roleSyncRunning.CompareAndSwap(false, true) {
		http.Redirect(w, r, back+"?notice="+errMsg("A full sync is already running."), http.StatusSeeOther)
		return
	}
	go func() {
		defer roleSyncRunning.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		p, c, f := d.RoleSyncEngine.SyncAll(ctx)
		slog.Info("rolesync: manual full pass", "players", p, "changes", c, "failures", f)
	}()
	http.Redirect(w, r, back+"?notice="+errMsg("Full sync started. Refresh in a minute to see the results below."), http.StatusSeeOther)
}
