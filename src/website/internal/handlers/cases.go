package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"website/internal/audit"
	"website/internal/auth"
	"website/internal/cases"
)

// Moderation pages (layout plan "Admin — Moderation"): Cases, Case view,
// Bans and Anti-Cheat Flags. Business rules live in internal/cases; these
// handlers check permissions and render.

func caseActor(r *http.Request) cases.Actor {
	sess, _ := auth.FromContext(r.Context())
	return cases.Actor{PlayerID: sess.PlayerID, Source: audit.SourceWebsite}
}

// finishCaseAction redirects with the result of a moderation action.
func finishCaseAction(w http.ResponseWriter, r *http.Request, back string, err error, success string) {
	switch {
	case err == nil:
		http.Redirect(w, r, back+sep(back)+"notice="+errMsg(success), http.StatusSeeOther)
	case errors.Is(err, cases.ErrNotAllowed):
		msg := strings.TrimPrefix(err.Error(), cases.ErrNotAllowed.Error()+": ")
		http.Redirect(w, r, back+sep(back)+"error="+errMsg(strings.ToUpper(msg[:1])+msg[1:]+"."), http.StatusSeeOther)
	default:
		slog.Error("moderation action failed", "path", r.URL.Path, "error", err)
		http.Redirect(w, r, back+sep(back)+"error="+errMsg("Something went wrong. Nothing was changed."), http.StatusSeeOther)
	}
}

func (d *Deps) denyPerm(w http.ResponseWriter, r *http.Request, label string) {
	d.Denied(w, r, auth.DeniedInfo{Kind: "no_permission", Area: "admin", Permission: label})
}

var (
	steam64Re = regexp.MustCompile(`^\d{17}$`)
	idRe      = regexp.MustCompile(`^#?(\d{1,9})$`)
)

// resolvePlayer turns "76561…" (Steam64), "#12" (player id) or an exact
// current/past name into one player id.
func (d *Deps) resolvePlayer(ctx context.Context, s string) (int64, string, error) {
	s = strings.TrimSpace(s)
	var id int64
	var n int
	var err error
	switch {
	case s == "":
		return 0, "", nil
	case steam64Re.MatchString(s):
		err = d.Pool.QueryRow(ctx, `SELECT COALESCE(min(id), 0), count(*) FROM players WHERE uid = $1`, s).Scan(&id, &n)
	case idRe.MatchString(s):
		v, _ := strconv.ParseInt(idRe.FindStringSubmatch(s)[1], 10, 64)
		err = d.Pool.QueryRow(ctx, `SELECT COALESCE(min(id), 0), count(*) FROM players WHERE id = $1`, v).Scan(&id, &n)
	default:
		err = d.Pool.QueryRow(ctx, `
			SELECT COALESCE(min(id), 0), count(DISTINCT id) FROM players p
			WHERE lower(p.name) = lower($1) OR lower(p.steam_name) = lower($1)
			   OR EXISTS (SELECT 1 FROM player_aliases a WHERE a.player_id = p.id AND lower(a.name) = lower($1))`, s).Scan(&id, &n)
	}
	switch {
	case err != nil:
		return 0, "", err
	case n == 0:
		return 0, "", fmt.Errorf("%w: no player matches %q", cases.ErrNotAllowed, s)
	case n > 1:
		return 0, "", fmt.Errorf("%w: more than one player matches %q; use their Steam64 or #id", cases.ErrNotAllowed, s)
	}
	return id, s, nil
}

// ---- Case list ----

type casesListData struct {
	Base
	AdminShell
	Query   url.Values
	Rows    []cases.ListRow
	Status  string
	Type    string
	Lead    string
	Range   string
	Q       string
	Types   []struct{ Key, Label string }
	Leads   []staffOption
	Counts  map[string]int
	CanOpen bool
	Mine    bool
}

func (d *Deps) staffOptions(ctx context.Context) []staffOption {
	var out []staffOption
	rows, err := d.Pool.Query(ctx, `
		SELECT p.id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id)
		FROM players p JOIN staff_ranks sr ON sr.id = p.staff_rank_id ORDER BY sr.level DESC, 2`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var o staffOption
		if rows.Scan(&o.ID, &o.Name) == nil {
			out = append(out, o)
		}
	}
	return out
}

func (d *Deps) CasesList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	data := casesListData{Base: baseFrom(r, "Cases"), AdminShell: d.adminShell(r, "cases"),
		Status: q.Get("status"), Type: q.Get("type"), Lead: q.Get("lead"), Range: q.Get("range"), Q: strings.TrimSpace(q.Get("q")),
		Types: cases.Types, CanOpen: d.can(r, "cases.lead"), Mine: q.Get("mine") == "1", Query: q}
	if data.Status != "open" && data.Status != "closed" {
		data.Status = ""
	}
	if data.Range == "" {
		data.Range = "30d"
	}
	f := cases.Filter{Status: data.Status, Type: data.Type, Q: data.Q}
	f.LeadID, _ = strconv.ParseInt(data.Lead, 10, 64)
	switch data.Range {
	case "7d":
		f.Since = 7 * 24 * time.Hour
	case "30d":
		f.Since = 30 * 24 * time.Hour
	}
	if data.Mine {
		sess, _ := auth.FromContext(r.Context())
		f.Mine = sess.PlayerID
	}
	var err error
	if data.Rows, err = cases.List(r.Context(), d.Pool, f); err != nil {
		slog.Error("cases: list failed", "error", err)
		http.Error(w, "Failed to load cases.", http.StatusInternalServerError)
		return
	}
	data.Counts = map[string]int{}
	f.Status = ""
	if all, err := cases.List(r.Context(), d.Pool, f); err == nil {
		for _, c := range all {
			data.Counts[c.Status]++
			data.Counts[""]++
		}
	}
	data.Leads = d.staffOptions(r.Context())
	d.Render.Render(w, "cases.html", data)
}

// ---- New case ----

type caseNewData struct {
	Base
	AdminShell
	Types   []struct{ Key, Label string }
	Type    string
	Summary string
	Players []string
	FlagID  int64
	Note    string
}

func (d *Deps) CaseNew(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	data := caseNewData{Base: baseFrom(r, "Open a case"), AdminShell: d.adminShell(r, "cases"), Types: cases.Types, Players: make([]string, 4)}
	if pid, err := strconv.ParseInt(q.Get("player"), 10, 64); err == nil && pid > 0 {
		data.Players[0] = fmt.Sprintf("#%d", pid)
	}
	if fid, err := strconv.ParseInt(q.Get("flag"), 10, 64); err == nil && fid > 0 {
		if fl, err := cases.GetFlag(r.Context(), d.Pool, fid); err == nil {
			data.FlagID = fid
			data.Players[0] = fmt.Sprintf("#%d", fl.PlayerID)
			data.Type = "cheating"
			data.Summary = fl.Title
		}
	}
	d.Render.Render(w, "case_new.html", data)
}

func (d *Deps) CaseCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form", http.StatusBadRequest)
		return
	}
	var ids []int64
	var err error
	for _, s := range r.Form["player"] {
		id, _, rerr := d.resolvePlayer(r.Context(), s)
		if rerr != nil {
			err = rerr
			break
		}
		if id > 0 {
			ids = append(ids, id)
		}
	}
	flagID, _ := strconv.ParseInt(r.FormValue("flag"), 10, 64)
	var id int64
	if err == nil {
		id, err = cases.Open(r.Context(), d.Pool, caseActor(r), r.FormValue("type"), r.FormValue("summary"), ids, flagID, r.FormValue("note"))
	}
	if err != nil {
		data := caseNewData{Base: baseFrom(r, "Open a case"), AdminShell: d.adminShell(r, "cases"), Types: cases.Types,
			Type: r.FormValue("type"), Summary: r.FormValue("summary"), Players: r.Form["player"], FlagID: flagID, Note: r.FormValue("note")}
		for len(data.Players) < 4 {
			data.Players = append(data.Players, "")
		}
		if errors.Is(err, cases.ErrNotAllowed) {
			msg := strings.TrimPrefix(err.Error(), cases.ErrNotAllowed.Error()+": ")
			data.Error = strings.ToUpper(msg[:1]) + msg[1:] + "."
		} else {
			slog.Error("cases: open failed", "error", err)
			data.Error = "Something went wrong. The case wasn't opened."
		}
		d.Render.RenderStatus(w, http.StatusUnprocessableEntity, "case_new.html", data)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/cases/%d?notice=%s", id, errMsg("Case opened.")), http.StatusSeeOther)
}

// ---- Case view ----

type caseViewData struct {
	Base
	AdminShell
	Case       cases.Case
	Entries    []cases.Entry
	Bans       []cases.Ban
	Correcting int
	CanWrite   bool
	CanClose   bool
	CanBan     bool
	CanPerm    bool
	CanLift    bool
	Staff      []staffOption
	BanReasons []string
	Durations  []struct {
		Days  int
		Label string
	}
	OwnFaction  []string // subjects in the viewer's own faction (conflict-of-interest warning)
	DefaultEnds string   // default "stops counting" date for points
}

func caseID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id, err == nil && id > 0
}

func (d *Deps) CaseView(w http.ResponseWriter, r *http.Request) {
	id, ok := caseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	c, err := cases.Get(ctx, d.Pool, id)
	if errors.Is(err, cases.ErrNotAllowed) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("cases: get failed", "error", err)
		http.Error(w, "Failed to load the case.", http.StatusInternalServerError)
		return
	}
	data := caseViewData{Base: baseFrom(r, fmt.Sprintf("Case #%d", id)), AdminShell: d.adminShell(r, "cases"), Case: c,
		CanWrite: d.can(r, "cases.lead"), CanClose: d.can(r, "cases.close"), CanBan: d.can(r, "bans.issue"),
		CanPerm: d.can(r, "bans.permanent"), CanLift: d.can(r, "bans.revoke"),
		BanReasons: cases.BanReasons, Durations: cases.BanDurations,
		DefaultEnds: time.Now().AddDate(0, 3, 0).Format("2006-01-02")}
	data.Correcting, _ = strconv.Atoi(r.URL.Query().Get("correct"))
	if data.Entries, err = cases.Entries(ctx, d.Pool, id); err != nil {
		slog.Error("cases: entries failed", "error", err)
	}
	for _, active := range []bool{true, false} {
		bans, err := cases.Bans(ctx, d.Pool, active, 500)
		if err != nil {
			continue
		}
		for _, b := range bans {
			if b.CaseID == id {
				data.Bans = append(data.Bans, b)
			}
		}
	}
	data.Staff = d.staffOptions(ctx)
	// Conflict of interest (GAMEPANEL_PARITY §6.1 rule 6): the lead shares a
	// faction with a subject.
	sess, _ := auth.FromContext(ctx)
	var myCop, myMed int
	_ = d.Pool.QueryRow(ctx, `SELECT COALESCE(cop_level, 0), COALESCE(medic_level, 0) FROM players WHERE id = $1`, sess.PlayerID).Scan(&myCop, &myMed)
	if c.LeadID == sess.PlayerID {
		for _, p := range c.Players {
			if (myCop > 0 && p.PoliceLevel > 0) || (myMed > 0 && p.EMSLevel > 0) {
				data.OwnFaction = append(data.OwnFaction, p.Name)
			}
		}
	}
	d.Render.Render(w, "case_view.html", data)
}

func caseBack(id int64) string { return fmt.Sprintf("/admin/cases/%d", id) }

func (d *Deps) CaseAddEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := caseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !d.can(r, "cases.lead") {
		d.denyPerm(w, r, "Open and lead cases")
		return
	}
	corrects, _ := strconv.Atoi(r.FormValue("corrects"))
	err := cases.AddNote(r.Context(), d.Pool, caseActor(r), id, r.FormValue("body"), corrects)
	finishCaseAction(w, r, caseBack(id)+"#entries-end", err, "Entry added.")
}

func (d *Deps) CaseSetStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := caseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !d.can(r, "cases.close") {
		d.denyPerm(w, r, "Close cases")
		return
	}
	status := r.FormValue("status")
	err := cases.SetStatus(r.Context(), d.Pool, caseActor(r), id, status, r.FormValue("outcome"), r.FormValue("reason"))
	finishCaseAction(w, r, caseBack(id), err, map[string]string{"closed": "Case closed.", "open": "Case reopened."}[status])
}

func (d *Deps) CaseAddParticipant(w http.ResponseWriter, r *http.Request) {
	id, ok := caseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !d.can(r, "cases.lead") {
		d.denyPerm(w, r, "Open and lead cases")
		return
	}
	pid, _, err := d.resolvePlayer(r.Context(), r.FormValue("who"))
	if err == nil && pid == 0 {
		err = fmt.Errorf("%w: enter a Steam64, #id or exact name", cases.ErrNotAllowed)
	}
	if err == nil {
		err = cases.AddParticipant(r.Context(), d.Pool, caseActor(r), id, pid, r.FormValue("role"))
	}
	finishCaseAction(w, r, caseBack(id), err, "Added to the case.")
}

func (d *Deps) CaseIssuePoints(w http.ResponseWriter, r *http.Request) {
	id, ok := caseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !d.can(r, "cases.lead") {
		d.denyPerm(w, r, "Open and lead cases")
		return
	}
	pid, _ := strconv.ParseInt(r.FormValue("player"), 10, 64)
	pts, _ := strconv.Atoi(r.FormValue("points"))
	var expires *time.Time
	if s := r.FormValue("expires"); s != "" {
		t, err := time.ParseInLocation("2006-01-02", s, time.Local)
		if err != nil {
			finishCaseAction(w, r, caseBack(id), fmt.Errorf("%w: pick a valid date", cases.ErrNotAllowed), "")
			return
		}
		expires = &t
	}
	err := cases.IssuePoints(r.Context(), d.Pool, caseActor(r), id, pid, pts, r.FormValue("rules"), r.FormValue("comment"), expires)
	finishCaseAction(w, r, caseBack(id), err, "Punishment points issued.")
}

func (d *Deps) PointsRevoke(w http.ResponseWriter, r *http.Request) {
	pid, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !d.can(r, "cases.close") {
		d.denyPerm(w, r, "Close cases")
		return
	}
	back := r.FormValue("back")
	if !strings.HasPrefix(back, "/admin/") {
		back = "/admin/cases"
	}
	err = cases.RevokePoints(r.Context(), d.Pool, caseActor(r), pid, r.FormValue("reason"))
	finishCaseAction(w, r, back, err, "Points revoked.")
}

// ---- Bans ----

type bansData struct {
	Base
	AdminShell
	Tab         string
	Stats       cases.BanStats
	Bans        []cases.Ban
	Appeals     []cases.Appeal
	CaseOptions []cases.ListRow
	CasePlayers map[int64][]cases.Participant
	CanBan      bool
	CanPerm     bool
	CanLift     bool
	CanAppeal   bool
	BanReasons  []string
	Durations   []struct {
		Days  int
		Label string
	}
	PreCase int64
}

func (d *Deps) BansList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := bansData{Base: baseFrom(r, "Bans"), AdminShell: d.adminShell(r, "bans"), Tab: r.URL.Query().Get("tab"),
		CanBan: d.can(r, "bans.issue"), CanPerm: d.can(r, "bans.permanent"), CanLift: d.can(r, "bans.revoke"),
		CanAppeal: d.can(r, "bans.appeal_review"), BanReasons: cases.BanReasons, Durations: cases.BanDurations}
	data.PreCase, _ = strconv.ParseInt(r.URL.Query().Get("case"), 10, 64)
	if data.Tab != "expired" && data.Tab != "appeals" {
		data.Tab = "active"
	}
	var err error
	if data.Stats, err = cases.Stats(ctx, d.Pool); err != nil {
		slog.Error("bans: stats failed", "error", err)
	}
	switch data.Tab {
	case "appeals":
		data.Appeals, err = cases.Appeals(ctx, d.Pool)
	default:
		data.Bans, err = cases.Bans(ctx, d.Pool, data.Tab == "active", 300)
	}
	if err != nil {
		slog.Error("bans: list failed", "error", err)
		http.Error(w, "Failed to load bans.", http.StatusInternalServerError)
		return
	}
	if data.CanBan {
		data.CaseOptions, _ = cases.List(ctx, d.Pool, cases.Filter{Status: "open", Limit: 100})
		data.CasePlayers = map[int64][]cases.Participant{}
		for _, c := range data.CaseOptions {
			if full, err := cases.Get(ctx, d.Pool, c.ID); err == nil {
				data.CasePlayers[c.ID] = full.Players
			}
		}
	}
	d.Render.Render(w, "bans.html", data)
}

// discordBan applies or lifts a Discord ban through the bot, best effort:
// the game ban already stands; a Discord failure is reported, not fatal.
func (d *Deps) discordBan(ctx context.Context, discordID, reason string, lift bool) string {
	if discordID == "" {
		return ""
	}
	if d.Bot == nil {
		return " The Discord part couldn't be applied: the bot isn't running."
	}
	var err error
	if lift {
		err = d.Bot.UnbanMember(ctx, discordID)
	} else {
		err = d.Bot.BanMember(ctx, discordID, reason)
	}
	if err != nil {
		slog.Error("discord ban failed", "lift", lift, "error", err)
		return " The Discord part failed: " + err.Error()
	}
	if lift {
		return " Their Discord ban was lifted too."
	}
	return " They were banned from Discord too."
}

func (d *Deps) BanIssue(w http.ResponseWriter, r *http.Request) {
	caseNum, _ := strconv.ParseInt(r.FormValue("case"), 10, 64)
	back := r.FormValue("back")
	if !strings.HasPrefix(back, "/admin/") {
		back = "/admin/bans"
	}
	if !d.can(r, "bans.issue") {
		d.denyPerm(w, r, "Issue timed bans")
		return
	}
	days, err := strconv.Atoi(r.FormValue("days"))
	if err != nil {
		finishCaseAction(w, r, back, fmt.Errorf("%w: pick a ban length", cases.ErrNotAllowed), "")
		return
	}
	if days == 0 && !d.can(r, "bans.permanent") {
		finishCaseAction(w, r, back, fmt.Errorf("%w: permanent bans need the bans.permanent permission", cases.ErrNotAllowed), "")
		return
	}
	pid, _ := strconv.ParseInt(r.FormValue("player"), 10, 64)
	// The Bans page picks case and player together: "caseID:playerID".
	if cp := r.FormValue("case_player"); cp != "" {
		if c, p, ok := strings.Cut(cp, ":"); ok {
			caseNum, _ = strconv.ParseInt(c, 10, 64)
			pid, _ = strconv.ParseInt(p, 10, 64)
		}
	}
	reason := r.FormValue("reason")
	if reason == "Other" && strings.TrimSpace(r.FormValue("reason_other")) != "" {
		reason = strings.TrimSpace(r.FormValue("reason_other"))
	}
	if caseNum == 0 {
		finishCaseAction(w, r, back, fmt.Errorf("%w: every ban comes from a case; pick one", cases.ErrNotAllowed), "")
		return
	}
	_, discordID, err := cases.IssueBan(r.Context(), d.Pool, caseActor(r), caseNum, pid, reason, days, r.FormValue("discord") == "on", r.FormValue("note"))
	msg := cases.DurationLabel(days) + " issued. It applies the next time they connect."
	if err == nil {
		msg += d.discordBan(r.Context(), discordID, reason, false)
	}
	finishCaseAction(w, r, back, err, msg)
}

func (d *Deps) BanLift(w http.ResponseWriter, r *http.Request) {
	banID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !d.can(r, "bans.revoke") {
		d.denyPerm(w, r, "Lift bans")
		return
	}
	back := r.FormValue("back")
	if !strings.HasPrefix(back, "/admin/") {
		back = "/admin/bans"
	}
	discordID, err := cases.LiftBan(r.Context(), d.Pool, caseActor(r), banID, r.FormValue("reason"))
	msg := "Ban lifted. They can connect again."
	if err == nil {
		msg += d.discordBan(r.Context(), discordID, "", true)
	}
	finishCaseAction(w, r, back, err, msg)
}

func (d *Deps) AppealDecide(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !d.can(r, "bans.appeal_review") {
		d.denyPerm(w, r, "Review ban appeals")
		return
	}
	accept := r.FormValue("decision") == "accept"
	discordID, err := cases.DecideAppeal(r.Context(), d.Pool, caseActor(r), id, accept, r.FormValue("reason"))
	msg := "Appeal rejected. The ban stays."
	if accept {
		msg = "Appeal accepted and the ban lifted."
		if err == nil {
			msg += d.discordBan(r.Context(), discordID, "", true)
		}
	}
	finishCaseAction(w, r, "/admin/bans?tab=appeals", err, msg)
}

// ---- Anti-cheat ----

type antiCheatData struct {
	Base
	AdminShell
	Flags      []cases.Flag
	Selected   *cases.Flag
	Confidence string
	OpenOnly   bool
	CanCase    bool
	Counts     map[string]int
}

func (d *Deps) AntiCheat(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	data := antiCheatData{Base: baseFrom(r, "Anti-Cheat Flags"), AdminShell: d.adminShell(r, "anticheat"),
		Confidence: q.Get("confidence"), OpenOnly: q.Get("open") == "1", CanCase: d.can(r, "cases.lead")}
	if data.Confidence != "high" && data.Confidence != "medium" {
		data.Confidence = ""
	}
	var err error
	if data.Flags, err = cases.Flags(r.Context(), d.Pool, cases.FlagFilter{Confidence: data.Confidence, OpenOnly: data.OpenOnly}, 300); err != nil {
		slog.Error("anticheat: list failed", "error", err)
		http.Error(w, "Failed to load flags.", http.StatusInternalServerError)
		return
	}
	data.Counts = map[string]int{}
	if all, err := cases.Flags(r.Context(), d.Pool, cases.FlagFilter{}, 1000); err == nil {
		for _, f := range all {
			data.Counts[f.Confidence]++
			data.Counts[""]++
		}
	}
	if sel, err := strconv.ParseInt(q.Get("flag"), 10, 64); err == nil {
		if f, err := cases.GetFlag(r.Context(), d.Pool, sel); err == nil {
			data.Selected = &f
		}
	}
	if data.Selected == nil && len(data.Flags) > 0 {
		data.Selected = &data.Flags[0]
	}
	d.Render.Render(w, "anticheat.html", data)
}

func (d *Deps) FlagReview(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	res := r.FormValue("resolution")
	err = cases.ReviewFlag(r.Context(), d.Pool, caseActor(r), id, res, r.FormValue("note"))
	finishCaseAction(w, r, fmt.Sprintf("/admin/anticheat?flag=%d", id), err,
		map[string]string{"dismiss": "Dismissed as a false positive.", "watch": "Marked as watching.", "": "Flag reopened."}[res])
}

// ---- Player side: ban appeals ----

// SubmitAppeal is the dashboard's "Appeal your ban" form.
func (d *Deps) SubmitAppeal(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	err := cases.SubmitAppeal(r.Context(), d.Pool, sess.PlayerID, r.FormValue("body"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/dashboard?notice="+errMsg("Appeal sent. Staff will review it and you'll see the decision here."), http.StatusSeeOther)
	case errors.Is(err, cases.ErrNotAllowed):
		msg := strings.TrimPrefix(err.Error(), cases.ErrNotAllowed.Error()+": ")
		http.Redirect(w, r, "/dashboard?error="+errMsg(strings.ToUpper(msg[:1])+msg[1:]+"."), http.StatusSeeOther)
	default:
		slog.Error("appeal: submit failed", "error", err)
		http.Redirect(w, r, "/dashboard?error="+errMsg("Something went wrong sending your appeal."), http.StatusSeeOther)
	}
}

// playerPoints is shared with Player Lookup.
func (d *Deps) playerPoints(ctx context.Context, id int64) ([]cases.Points, int) {
	pts, total, err := cases.PlayerPoints(ctx, d.Pool, id)
	if err != nil {
		slog.Error("points: load failed", "error", err)
	}
	return pts, total
}
