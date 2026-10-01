package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"website/internal/auth"
	"website/internal/devboard"
)

// Admin → Development (dev.tools): system health, the website's own log,
// and the development project board.

// ---- System health ----

type healthCheck struct {
	Name   string
	State  string // ok / warn / off / bad
	Detail string
}

type healthJob struct {
	Name    string
	Last    *time.Time
	Detail  string
	Problem string
}

type devHealthData struct {
	Base
	AdminShell
	Version   string
	Commit    string
	GoVersion string
	Started   time.Time
	Uptime    string
	Goroutine int
	MemMB     float64
	Errors    int64
	Warns     int64

	DBVersion  string
	DBSize     string
	DBConns    string
	DBLatency  string
	Players    int64
	Tables     int64
	Checks     []healthCheck
	Jobs       []healthJob
	OutboxWait int64
	OutboxDead int64
}

var (
	commitOnce sync.Once
	commitID   string
)

// buildCommit is the git commit the website runs from: from the build info
// when stamped, else asked of git once.
func buildCommit() string {
	commitOnce.Do(func() {
		if bi, ok := debug.ReadBuildInfo(); ok {
			for _, s := range bi.Settings {
				if s.Key == "vcs.revision" && len(s.Value) >= 7 {
					commitID = s.Value[:7]
				}
			}
		}
		if commitID == "" {
			if out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
				commitID = strings.TrimSpace(string(out))
			}
		}
		if commitID == "" {
			commitID = "unknown"
		}
	})
	return commitID
}

func humanUptime(d time.Duration) string {
	d = d.Round(time.Minute)
	days := int(d.Hours()) / 24
	h := int(d.Hours()) % 24
	m := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh %dm", days, h, m)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func (d *Deps) DevHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := devHealthData{Base: baseFrom(r, "System health"), AdminShell: d.adminShell(r, "dev-health"),
		Commit: buildCommit(), GoVersion: runtime.Version(), Started: d.StartedAt, Goroutine: runtime.NumGoroutine()}
	if !d.StartedAt.IsZero() {
		data.Uptime = humanUptime(time.Since(d.StartedAt))
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	data.MemMB = float64(ms.Alloc) / (1 << 20)
	if d.LogBuf != nil {
		data.Errors, data.Warns = d.LogBuf.Counts()
	}

	// Database.
	t0 := time.Now()
	if err := d.Pool.QueryRow(ctx, `SELECT current_setting('server_version'), pg_size_pretty(pg_database_size(current_database())),
		(SELECT count(*) FROM players), (SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public')`).
		Scan(&data.DBVersion, &data.DBSize, &data.Players, &data.Tables); err != nil {
		slog.Error("dev health: database query failed", "error", err)
	}
	data.DBLatency = fmt.Sprintf("%d ms", time.Since(t0).Milliseconds())
	st := d.Pool.Stat()
	data.DBConns = fmt.Sprintf("%d in use · %d idle · %d max", st.AcquiredConns(), st.IdleConns(), st.MaxConns())

	// Integrations.
	cfg := d.Cfg
	set := func(name string, ok bool, detail, missing string) {
		c := healthCheck{Name: name, State: "ok", Detail: detail}
		if !ok {
			c.State, c.Detail = "off", missing
		}
		data.Checks = append(data.Checks, c)
	}
	set("Discord bot", d.Bot != nil, "Connected; role sync running", "Not running (DISCORD_BOT_TOKEN not set, or it failed to start)")
	set("Discord sign-in", cfg.DiscordOAuthClientID != "", "OAuth configured", "DISCORD_OAUTH_CLIENT_ID not set")
	hooks := 0
	for _, h := range []string{cfg.DiscordStaffLogWebhook, cfg.DiscordTicketLogWebhook, cfg.DiscordAntiCheatWebhook} {
		if h != "" {
			hooks++
		}
	}
	set("Discord webhooks", hooks > 0, fmt.Sprintf("%d of 3 set (staff log, tickets, anti-cheat)", hooks), "None set")
	set("Steam Web API", cfg.SteamWebAPIKey != "", "Profiles and VAC bans refresh daily", "STEAM_WEB_API_KEY not set: no Steam names, avatars or ban data")
	set("Game server query", cfg.GameQueryAddr != "", cfg.GameQueryAddr, "GAME_QUERY_ADDR not set: status page can't see the server")
	set("Read-only database role", d.DB != nil && d.DB.Dedicated, "Database browser uses a SELECT-only role", "DATABASE_READONLY_URL not set: browser uses read-only transactions")
	sc := healthCheck{Name: "Server Control", State: "off", Detail: "SERVER_MANAGER_URL / TOKEN not set"}
	if d.ServerMgr.Configured() {
		cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		s, err := d.ServerMgr.Status(cctx)
		cancel()
		switch {
		case err != nil:
			sc.State, sc.Detail = "bad", "server_manager isn't answering ("+err.Error()+")"
		case s.Running:
			sc.State, sc.Detail = "ok", fmt.Sprintf("Connected; game server running (PID %d)", s.PID)
		default:
			sc.State, sc.Detail = "warn", "Connected; game server is stopped"
		}
	}
	data.Checks = append(data.Checks, sc)

	// Background jobs.
	var outboxLast *time.Time
	var outboxErr string
	_ = d.Pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE sent_at IS NULL AND gave_up_at IS NULL),
		count(*) FILTER (WHERE gave_up_at IS NOT NULL), max(sent_at),
		COALESCE((SELECT last_error FROM discord_outbox WHERE last_error IS NOT NULL ORDER BY id DESC LIMIT 1), '')
		FROM discord_outbox`).Scan(&data.OutboxWait, &data.OutboxDead, &outboxLast, &outboxErr)
	job := healthJob{Name: "Discord outbox", Last: outboxLast, Detail: fmt.Sprintf("%d waiting · %d gave up", data.OutboxWait, data.OutboxDead)}
	if data.OutboxDead > 0 && outboxErr != "" {
		job.Problem = "Last error: " + outboxErr
	}
	data.Jobs = append(data.Jobs, job)

	var syncLast *time.Time
	var syncFails int64
	var syncErr string
	_ = d.Pool.QueryRow(ctx, `SELECT max(created_at), count(*) FILTER (WHERE NOT ok AND created_at > now() - interval '24 hours'),
		COALESCE((SELECT error FROM sync_log WHERE NOT ok ORDER BY id DESC LIMIT 1), '') FROM sync_log`).Scan(&syncLast, &syncFails, &syncErr)
	job = healthJob{Name: "Discord role sync", Last: syncLast, Detail: fmt.Sprintf("%d failures in the last 24 h", syncFails)}
	if syncFails > 0 && syncErr != "" {
		job.Problem = "Last error: " + syncErr
	}
	data.Jobs = append(data.Jobs, job)

	var postLast *time.Time
	var unposted int64
	_ = d.Pool.QueryRow(ctx, `SELECT max(discord_posted_at), count(*) FILTER (WHERE discord_posted_at IS NULL AND created_at > now() - interval '24 hours') FROM staff_log`).Scan(&postLast, &unposted)
	job = healthJob{Name: "Staff Log → Discord", Last: postLast, Detail: fmt.Sprintf("%d from the last 24 h not posted", unposted)}
	data.Jobs = append(data.Jobs, job)

	var steamLast *time.Time
	var stale int64
	_ = d.Pool.QueryRow(ctx, `SELECT max(steam_refreshed_at), count(*) FILTER (WHERE steam_refreshed_at IS NULL OR steam_refreshed_at < now() - interval '2 days') FROM players`).Scan(&steamLast, &stale)
	data.Jobs = append(data.Jobs, healthJob{Name: "Steam profile refresh", Last: steamLast, Detail: fmt.Sprintf("%d players not refreshed in 2 days", stale)})

	d.Render.Render(w, "dev_health.html", data)
}

// ---- Website logs ----

type devLogsData struct {
	Base
	AdminShell
}

func (d *Deps) DevLogs(w http.ResponseWriter, r *http.Request) {
	d.Render.Render(w, "dev_logs.html", devLogsData{Base: baseFrom(r, "Website logs"), AdminShell: d.adminShell(r, "dev-logs")})
}

// DevLogsJSON returns log entries after ?after=<id>.
func (d *Deps) DevLogsJSON(w http.ResponseWriter, r *http.Request) {
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if d.LogBuf == nil {
		w.Write([]byte("[]"))
		return
	}
	_ = json.NewEncoder(w).Encode(d.LogBuf.Since(after))
}

// ---- Project board ----

type devBoardData struct {
	Base
	AdminShell
	Columns   []devboard.Column
	Staff     []devboard.Person
	Labels    []devboard.Label
	Palette   []devboard.Colour
	Mine      bool
	Q         string
	Label     string
	Edit      *devboard.Card
	Statuses  []devboard.Status
	Options   []devboard.Task // cards to link to
	LinkKinds []struct{ Key, Label string }
	Me        int64
}

// StatusLabel names a column for the board template.
func (devBoardData) StatusLabel(key string) string { return devboard.StatusLabel(key) }

// LabelColor is the palette key for a label name.
func (d devBoardData) LabelColor(name string) string {
	for _, l := range d.Labels {
		if l.Name == name {
			return l.Color
		}
	}
	return "grey"
}

// HasLabel reports whether a card carries the label.
func (devBoardData) HasLabel(labels []string, name string) bool {
	for _, l := range labels {
		if l == name {
			return true
		}
	}
	return false
}

// NoLabels is an empty selection for the new-task label picker.
func (devBoardData) NoLabels() []string { return nil }

// BoardURL is the board with the current filters, for returning to.
func (d devBoardData) BoardURL() string {
	v := url.Values{}
	if d.Q != "" {
		v.Set("q", d.Q)
	}
	if d.Label != "" {
		v.Set("label", d.Label)
	}
	if d.Mine {
		v.Set("mine", "1")
	}
	if d.Edit != nil {
		v.Set("task", strconv.FormatInt(d.Edit.ID, 10))
	}
	if len(v) == 0 {
		return "/admin/dev/board"
	}
	return "/admin/dev/board?" + v.Encode()
}

func (d *Deps) DevBoard(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	q := r.URL.Query()
	data := devBoardData{Base: baseFrom(r, "Project board"), AdminShell: d.adminShell(r, "dev-board"),
		Mine: q.Get("mine") == "1", Q: strings.TrimSpace(q.Get("q")), Label: q.Get("label"), Statuses: devboard.Statuses,
		LinkKinds: devboard.LinkKinds, Me: sess.PlayerID, Palette: devboard.Palette}
	f := devboard.Filter{Q: data.Q, Label: data.Label}
	if data.Mine {
		f.Assignee = sess.PlayerID
	}
	var err error
	if data.Columns, err = devboard.Board(r.Context(), d.Pool, f); err != nil {
		slog.Error("dev board: load failed", "error", err)
		http.Error(w, "Failed to load the board.", http.StatusInternalServerError)
		return
	}
	data.Staff, _ = devboard.People(r.Context(), d.Pool)
	data.Labels, _ = devboard.ListLabels(r.Context(), d.Pool)
	if id, _ := strconv.ParseInt(q.Get("task"), 10, 64); id > 0 {
		if c, err := devboard.GetCard(r.Context(), d.Pool, id); err == nil {
			data.Edit = &c
			data.Options, _ = devboard.Options(r.Context(), d.Pool)
		} else if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("dev board: card load failed", "error", err)
		}
	}
	d.Render.Render(w, "dev_board.html", data)
}

func devBoardBack(w http.ResponseWriter, r *http.Request, err error, notice string) {
	back := "/admin/dev/board"
	if ref := r.FormValue("back"); strings.HasPrefix(ref, "/admin/dev/board") {
		back = ref
	}
	sep := "?"
	if strings.Contains(back, "?") {
		sep = "&"
	}
	if err != nil {
		msg := "Something went wrong."
		var ue devboard.UserError
		if errors.As(err, &ue) {
			msg = ue.Error()
		} else if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("dev board: change failed", "error", err)
		}
		http.Redirect(w, r, back+sep+"error="+errMsg(msg), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, back+sep+"notice="+errMsg(notice), http.StatusSeeOther)
}

func taskFromForm(r *http.Request) devboard.Task {
	assignee, _ := strconv.ParseInt(r.FormValue("assignee"), 10, 64)
	labels := r.Form["label"]
	for _, l := range strings.Split(r.FormValue("labels"), ",") {
		if l = strings.TrimSpace(l); l != "" {
			labels = append(labels, l)
		}
	}
	t := devboard.Task{Title: r.FormValue("title"), Body: r.FormValue("body"), Status: r.FormValue("status"),
		Priority: r.FormValue("priority"), Labels: labels, AssigneeID: assignee}
	if due, err := time.ParseInLocation("2006-01-02", r.FormValue("due"), time.Local); err == nil {
		t.Due = &due
	}
	return t
}

func (d *Deps) DevTaskCreate(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	_, err := devboard.Create(r.Context(), d.Pool, sess.PlayerID, taskFromForm(r))
	devBoardBack(w, r, err, "Task added.")
}

func (d *Deps) DevTaskUpdate(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	t := taskFromForm(r)
	t.ID = id
	cardBack(w, r, id, devboard.Update(r.Context(), d.Pool, sess.PlayerID, t), "Card saved.")
}

// DevTaskMove moves a card to a column, before another card (or to the
// end). Answers JSON for the drag-and-drop board, or redirects for forms.
func (d *Deps) DevTaskMove(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	before, _ := strconv.ParseInt(r.FormValue("before"), 10, 64)
	err := devboard.Move(r.Context(), d.Pool, sess.PlayerID, id, r.FormValue("status"), before)
	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Write([]byte(`{"ok":true}`))
		return
	}
	devBoardBack(w, r, err, "Task moved.")
}

func (d *Deps) DevTaskDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	devBoardBack(w, r, devboard.Delete(r.Context(), d.Pool, id), "Task deleted.")
}

// ---- Card details: checklists, links, comments ----

// cardBack returns to the open card (or answers JSON for fetch requests).
func cardBack(w http.ResponseWriter, r *http.Request, taskID int64, err error, notice string) {
	back := fmt.Sprintf("/admin/dev/board?task=%d", taskID)
	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Write([]byte(`{"ok":true}`))
		return
	}
	if err != nil {
		msg := "Something went wrong."
		var ue devboard.UserError
		if errors.As(err, &ue) {
			msg = ue.Error()
		} else {
			slog.Error("dev board: card change failed", "error", err)
		}
		http.Redirect(w, r, back+"&error="+errMsg(msg), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, back+"&notice="+errMsg(notice), http.StatusSeeOther)
}

func idParam(r *http.Request, name string) int64 {
	n, _ := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	return n
}

func (d *Deps) DevChecklistAdd(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	id := idParam(r, "id")
	cardBack(w, r, id, devboard.AddChecklist(r.Context(), d.Pool, sess.PlayerID, id, r.FormValue("title")), "Checklist added.")
}

func (d *Deps) DevChecklistDelete(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	id := idParam(r, "id")
	cardBack(w, r, id, devboard.DeleteChecklist(r.Context(), d.Pool, sess.PlayerID, id, idParam(r, "cid")), "Checklist deleted.")
}

func (d *Deps) DevItemAdd(w http.ResponseWriter, r *http.Request) {
	id := idParam(r, "id")
	cardBack(w, r, id, devboard.AddItem(r.Context(), d.Pool, id, idParam(r, "cid"), r.FormValue("body")), "Item added.")
}

// DevItemToggle ticks or unticks an item; fetch requests get the card's
// new checklist progress back.
func (d *Deps) DevItemToggle(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	id := idParam(r, "id")
	done, total, err := devboard.SetItem(r.Context(), d.Pool, sess.PlayerID, id, idParam(r, "iid"), r.FormValue("done") == "1")
	if r.Header.Get("Accept") == "application/json" && err == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"done": done, "total": total})
		return
	}
	cardBack(w, r, id, err, "Checklist updated.")
}

func (d *Deps) DevItemDelete(w http.ResponseWriter, r *http.Request) {
	id := idParam(r, "id")
	cardBack(w, r, id, devboard.DeleteItem(r.Context(), d.Pool, id, idParam(r, "iid")), "Item removed.")
}

func (d *Deps) DevLinkAdd(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	id := idParam(r, "id")
	other, _ := strconv.ParseInt(strings.TrimPrefix(strings.TrimSpace(r.FormValue("other")), "#"), 10, 64)
	cardBack(w, r, id, devboard.AddLink(r.Context(), d.Pool, sess.PlayerID, id, other, r.FormValue("kind")), "Cards linked.")
}

func (d *Deps) DevLinkDelete(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	id := idParam(r, "id")
	cardBack(w, r, id, devboard.RemoveLink(r.Context(), d.Pool, sess.PlayerID, id, idParam(r, "other")), "Link removed.")
}

func (d *Deps) DevCommentAdd(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	id := idParam(r, "id")
	cardBack(w, r, id, devboard.AddComment(r.Context(), d.Pool, sess.PlayerID, id, r.FormValue("body")), "Comment added.")
}

func (d *Deps) DevCommentDelete(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	id := idParam(r, "id")
	cardBack(w, r, id, devboard.DeleteComment(r.Context(), d.Pool, sess.PlayerID, id, idParam(r, "cid")), "Comment deleted.")
}

// ---- Board labels ----

func (d *Deps) DevLabelSave(w http.ResponseWriter, r *http.Request) {
	devBoardBack(w, r, devboard.SaveLabel(r.Context(), d.Pool, r.FormValue("old"), r.FormValue("name"), r.FormValue("color")), "Label saved.")
}

func (d *Deps) DevLabelDelete(w http.ResponseWriter, r *http.Request) {
	devBoardBack(w, r, devboard.DeleteLabel(r.Context(), d.Pool, r.FormValue("old")), "Label deleted.")
}
