package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"website/internal/audit"
	"website/internal/auth"
	"website/internal/discord"
	"website/internal/servercontrol"
)

// Server Control (layout plan "Admin — Server Control", GAMEPANEL_PARITY
// §8): start/stop/restart and live logs, all through server_manager. The
// page opens for server.control or server.logs; actions need
// server.control, logs need server.logs.

// ServerID is the one game server this deployment runs; typing it confirms
// a restart or stop.
const ServerID = "altis-main"

type serverData struct {
	Base
	AdminShell
	Configured  bool
	ConnErr     string
	Status      servercontrol.Status
	Players     string
	CanControl  bool
	CanLogs     bool
	ServerID    string
	Uptime      string
	LastRestart string
	Cooldown    string
	RestartAt   string
}

func dur(sec int64) string {
	d := time.Duration(sec) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h >= 24 {
		return fmt.Sprintf("%dd %dh", h/24, h%24)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

func localTime(rfc string) string {
	t, err := time.Parse(time.RFC3339, rfc)
	if err != nil {
		return ""
	}
	return t.Local().Format("2 Jan 15:04")
}

func (d *Deps) ServerControl(w http.ResponseWriter, r *http.Request) {
	data := serverData{Base: baseFrom(r, "Server Control"), AdminShell: d.adminShell(r, "server"),
		CanControl: d.can(r, "server.control"), CanLogs: d.can(r, "server.logs"), ServerID: ServerID,
		Configured: d.ServerMgr.Configured()}
	if !data.CanControl && !data.CanLogs {
		d.denyPerm(w, r, "Restart, stop and start servers")
		return
	}
	if data.Configured {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		st, err := d.ServerMgr.Status(ctx)
		if err != nil {
			data.ConnErr = err.Error()
		} else {
			data.Status = st
			data.Uptime = dur(st.UptimeSeconds)
			data.LastRestart = localTime(st.LastRestart)
			data.Cooldown = localTime(st.CooldownUntil)
			data.RestartAt = localTime(st.RestartAt)
		}
	}
	// Player count from the status page's own A2S check of the game server.
	if d.StatusMonitor != nil {
		if snap, err := d.StatusMonitor.Snapshot(r.Context(), time.UTC); err == nil {
			for _, c := range snap.Components {
				if c.Key == "game" && c.State == "up" {
					data.Players = strings.TrimSuffix(c.Detail, " players")
				}
			}
		}
	}
	d.Render.Render(w, "server_control.html", data)
}

func (d *Deps) logServerAction(r *http.Request, action, reason string, after map[string]any) {
	sess, _ := auth.FromContext(r.Context())
	tx, err := d.Pool.Begin(r.Context())
	if err != nil {
		return
	}
	defer tx.Rollback(r.Context())
	if err := audit.LogStaffAction(r.Context(), tx, audit.Entry{StaffID: sess.PlayerID, Action: action, Reason: reason, After: after}); err == nil {
		_ = tx.Commit(r.Context())
	}
}

// announce posts to #server-status through the bot, if both exist. There's
// no in-game broadcast yet (no RCon), so Discord is where players hear it.
func (d *Deps) announce(r *http.Request, msg string) bool {
	if d.Bot == nil {
		return false
	}
	ch := d.Bot.Settings.Get(r.Context(), discord.ChannelServerStatus)
	if ch == "" {
		return false
	}
	return discord.EnqueueNow(r.Context(), d.Pool, discord.OutboxMessage{Kind: discord.KindChannelPost, Target: ch, Content: msg}) == nil
}

func (d *Deps) ServerAction(w http.ResponseWriter, r *http.Request) {
	const back = "/admin/server"
	if !d.can(r, "server.control") {
		d.denyPerm(w, r, "Restart, stop and start servers")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	action := r.FormValue("action")
	reason := strings.TrimSpace(r.FormValue("reason"))
	fail := func(msg string) {
		http.Redirect(w, r, back+"?error="+errMsg(msg), http.StatusSeeOther)
	}
	if (action == "restart" || action == "stop") && strings.TrimSpace(r.FormValue("confirm")) != ServerID {
		fail("Type " + ServerID + " to confirm.")
		return
	}
	if (action == "restart" || action == "stop") && reason == "" {
		fail("Give a reason; it goes in the Staff Log.")
		return
	}
	var err error
	var msg string
	switch action {
	case "restart":
		mins, _ := strconv.Atoi(r.FormValue("warn"))
		if mins < 0 || mins > 30 {
			mins = 5
		}
		var at time.Time
		if at, err = d.ServerMgr.Restart(ctx, time.Duration(mins)*time.Minute); err == nil {
			msg = "Restart scheduled for " + at.Local().Format("15:04") + "."
			if mins > 0 && d.announce(r, fmt.Sprintf("🔄 **Server restart in %d minute%s** (%s). Find a safe spot and log off before then.", mins, map[bool]string{true: "", false: "s"}[mins == 1], reason)) {
				msg += " Announced in #server-status."
			}
			d.logServerAction(r, "server.restart", reason, map[string]any{"server": ServerID, "in_minutes": mins})
		}
	case "cancel":
		if err = d.ServerMgr.CancelRestart(ctx); err == nil {
			msg = "Scheduled restart cancelled."
			d.announce(r, "The scheduled server restart was cancelled.")
			d.logServerAction(r, "server.restart_cancel", reason, map[string]any{"server": ServerID})
		}
	case "stop":
		if err = d.ServerMgr.Stop(ctx); err == nil {
			msg = "Server stopped."
			d.logServerAction(r, "server.stop", reason, map[string]any{"server": ServerID})
		}
	case "start":
		if err = d.ServerMgr.Start(ctx); err == nil {
			msg = "Server starting. It takes a minute or two to come up."
			d.logServerAction(r, "server.start", reason, map[string]any{"server": ServerID})
		}
	default:
		fail("Unknown action.")
		return
	}
	if err != nil {
		slog.Warn("server control failed", "action", action, "error", err)
		fail(err.Error())
		return
	}
	http.Redirect(w, r, back+"?notice="+errMsg(msg), http.StatusSeeOther)
}

// ServerLogs is the live log feed (JSON, polled by the page). IPs are
// masked for everyone below Management (level 100).
func (d *Deps) ServerLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !d.can(r, "server.logs") {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "server.logs is required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	lines, err := d.ServerMgr.Logs(ctx, 300)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "lines": []string{}})
		return
	}
	sess, _ := auth.FromContext(r.Context())
	var level int
	_ = d.Pool.QueryRow(r.Context(), `SELECT COALESCE(sr.level, 0) FROM players p LEFT JOIN staff_ranks sr ON sr.id = p.staff_rank_id WHERE p.id = $1`, sess.PlayerID).Scan(&level)
	if level < 100 {
		for i, l := range lines {
			lines[i] = servercontrol.MaskIPs(l)
		}
	}
	if lines == nil {
		lines = []string{}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"lines": lines})
}
