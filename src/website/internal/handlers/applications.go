package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"website/internal/applications"
	"website/internal/audit"
	"website/internal/auth"
	"website/internal/factions"
	"website/internal/staff"
)

// Recruitment pages (layout plan "Player — Join the staff team", "Player —
// Faction applications", "Admin — Applications", "Police command — Recruits
// & training"). Rules live in internal/applications.

func appActor(r *http.Request) applications.Actor {
	sess, _ := auth.FromContext(r.Context())
	return applications.Actor{PlayerID: sess.PlayerID, Source: audit.SourceWebsite}
}

func appMsg(err error) string {
	msg := strings.TrimPrefix(err.Error(), applications.ErrNotAllowed.Error()+": ")
	return strings.ToUpper(msg[:1]) + msg[1:] + "."
}

func finishAppAction(w http.ResponseWriter, r *http.Request, back string, err error, success string) {
	switch {
	case err == nil:
		http.Redirect(w, r, back+sep(back)+"notice="+errMsg(success), http.StatusSeeOther)
	case errors.Is(err, applications.ErrNotAllowed):
		http.Redirect(w, r, back+sep(back)+"error="+errMsg(appMsg(err)), http.StatusSeeOther)
	default:
		slog.Error("application action failed", "path", r.URL.Path, "error", err)
		http.Redirect(w, r, back+sep(back)+"error="+errMsg("Something went wrong. Nothing was changed."), http.StatusSeeOther)
	}
}

// ---- Player: staff application ----

type staffApplyData struct {
	Base
	E         applications.Eligibility
	Questions []applications.Question
	Form      map[string]string
	Agreed    bool
	Show      *applications.StaffApp // application whose status to show
}

func (d *Deps) StaffApply(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	e, err := applications.StaffEligibility(r.Context(), d.Pool, sess.PlayerID)
	if err != nil {
		slog.Error("staff apply: eligibility failed", "error", err)
		http.Error(w, "Failed to load the application.", http.StatusInternalServerError)
		return
	}
	data := staffApplyData{Base: baseFrom(r, "Join the staff team"), E: e, Questions: applications.StaffQuestions, Form: map[string]string{}}
	if e.Open != nil {
		data.Show = e.Open
	} else if e.Latest != nil && r.URL.Query().Get("new") != "1" {
		data.Show = e.Latest
	}
	d.Render.Render(w, "staff_apply.html", data)
}

func (d *Deps) StaffApplySubmit(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form", http.StatusBadRequest)
		return
	}
	form := map[string]string{}
	for _, q := range applications.StaffQuestions {
		form[q.Key] = r.FormValue(q.Key)
	}
	agreed := r.FormValue("agree") == "on"
	_, err := applications.SubmitStaff(r.Context(), d.Pool, sess.PlayerID, form, agreed)
	if err == nil {
		http.Redirect(w, r, "/staff/apply?notice="+errMsg("Application sent. It usually takes a few days to hear back."), http.StatusSeeOther)
		return
	}
	e, _ := applications.StaffEligibility(r.Context(), d.Pool, sess.PlayerID)
	data := staffApplyData{Base: baseFrom(r, "Join the staff team"), E: e, Questions: applications.StaffQuestions, Form: form, Agreed: agreed}
	if errors.Is(err, applications.ErrNotAllowed) {
		data.Error = appMsg(err)
	} else {
		slog.Error("staff apply: submit failed", "error", err)
		data.Error = "Something went wrong sending your application."
	}
	d.Render.RenderStatus(w, http.StatusUnprocessableEntity, "staff_apply.html", data)
}

func (d *Deps) StaffApplyWithdraw(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	err := applications.WithdrawStaff(r.Context(), d.Pool, sess.PlayerID)
	finishAppAction(w, r, "/staff/apply", err, "Application withdrawn.")
}

// ---- Player: faction applications ----

type factionOption struct {
	Key     string
	Name    string
	Members int
	Entry   string
	Queue   int
	Ranks   []factions.Rank
	Level   int // the player's level in it, 0 = not a member
	Open    *applications.FactionApp
}

type factionApplyData struct {
	Base
	Options   []factionOption
	Selected  *factionOption
	Questions []applications.Question
	Form      map[string]string
	Mine      []applications.FactionApp
}

func (d *Deps) factionApplyData(r *http.Request, selected string, form map[string]string) (factionApplyData, error) {
	ctx := r.Context()
	sess, _ := auth.FromContext(ctx)
	data := factionApplyData{Base: baseFrom(r, "Join a faction"), Form: form}
	var err error
	if data.Mine, err = applications.PlayerFactionApps(ctx, d.Pool, sess.PlayerID); err != nil {
		return data, err
	}
	var cop, med int
	_ = d.Pool.QueryRow(ctx, `SELECT COALESCE(cop_level, 0), COALESCE(medic_level, 0) FROM players WHERE id = $1`, sess.PlayerID).Scan(&cop, &med)
	for _, f := range factions.Factions {
		o := factionOption{Key: f, Name: factions.Name(f), Queue: applications.OpenFactionCount(ctx, d.Pool, f),
			Level: map[string]int{"police": cop, "ems": med}[f]}
		o.Ranks, _ = factions.Ranks(ctx, d.Pool, f)
		o.Entry = factions.RankFor(o.Ranks, 1).Label()
		col := map[string]string{"police": "cop_level", "ems": "medic_level"}[f]
		_ = d.Pool.QueryRow(ctx, `SELECT count(*) FROM players WHERE `+col+` > 0`).Scan(&o.Members)
		for i := range data.Mine {
			if data.Mine[i].Faction == f && data.Mine[i].Status == "pending" {
				o.Open = &data.Mine[i]
			}
		}
		data.Options = append(data.Options, o)
	}
	if !factions.Valid(selected) {
		selected = "police"
	}
	for i := range data.Options {
		if data.Options[i].Key == selected {
			data.Selected = &data.Options[i]
		}
	}
	data.Questions = applications.FactionQuestions(selected)
	return data, nil
}

func (d *Deps) FactionApply(w http.ResponseWriter, r *http.Request) {
	data, err := d.factionApplyData(r, r.URL.Query().Get("faction"), map[string]string{})
	if err != nil {
		slog.Error("faction apply: load failed", "error", err)
		http.Error(w, "Failed to load faction applications.", http.StatusInternalServerError)
		return
	}
	d.Render.Render(w, "faction_apply.html", data)
}

func (d *Deps) FactionApplySubmit(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	faction := r.FormValue("faction")
	if r.FormValue("withdraw") == "1" {
		err := applications.WithdrawFaction(r.Context(), d.Pool, sess.PlayerID, faction)
		finishAppAction(w, r, "/factions?faction="+faction, err, "Application withdrawn.")
		return
	}
	form := map[string]string{}
	for _, q := range applications.FactionQuestions(faction) {
		form[q.Key] = r.FormValue(q.Key)
	}
	err := applications.SubmitFaction(r.Context(), d.Pool, sess.PlayerID, faction, form)
	if err == nil {
		http.Redirect(w, r, "/factions?faction="+faction+"&notice="+errMsg("Application sent to "+factions.Name(faction)+" command."), http.StatusSeeOther)
		return
	}
	data, lerr := d.factionApplyData(r, faction, form)
	if lerr != nil {
		http.Error(w, "Failed to load faction applications.", http.StatusInternalServerError)
		return
	}
	if errors.Is(err, applications.ErrNotAllowed) {
		data.Error = appMsg(err)
	} else {
		slog.Error("faction apply: submit failed", "error", err)
		data.Error = "Something went wrong sending your application."
	}
	d.Render.RenderStatus(w, http.StatusUnprocessableEntity, "faction_apply.html", data)
}

// ---- Admin: staff applications ----

type applicationsData struct {
	Base
	AdminShell
	Status    string
	Counts    map[string]int
	List      []applications.StaffApp
	Selected  *applications.StaffApp
	CanDecide bool
	Ranks     []staff.Rank
	Teams     []string
	IVQs      []applications.Question
}

func (d *Deps) Applications(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ctx := r.Context()
	data := applicationsData{Base: baseFrom(r, "Applications"), AdminShell: d.adminShell(r, "applications"),
		Status: q.Get("status"), CanDecide: d.can(r, "applications.decide"), IVQs: applications.InterviewQuestions}
	switch data.Status {
	case "pending", "interview", "accepted", "rejected", "withdrawn", "all":
	default:
		data.Status = "pending"
	}
	filter := data.Status
	if filter == "all" {
		filter = ""
	}
	var err error
	if data.List, err = applications.ListStaff(ctx, d.Pool, filter); err != nil {
		slog.Error("applications: list failed", "error", err)
		http.Error(w, "Failed to load applications.", http.StatusInternalServerError)
		return
	}
	data.Counts, _ = applications.StaffCounts(ctx, d.Pool)
	if id, err := strconv.ParseInt(q.Get("id"), 10, 64); err == nil {
		if a, err := applications.GetStaffApp(ctx, d.Pool, id); err == nil {
			data.Selected = &a
		}
	}
	if data.Selected == nil && len(data.List) > 0 {
		data.Selected = &data.List[0]
	}
	if data.CanDecide {
		sess, _ := auth.FromContext(ctx)
		var myLevel int
		_ = d.Pool.QueryRow(ctx, `SELECT COALESCE(sr.level, 0) FROM players p LEFT JOIN staff_ranks sr ON sr.id = p.staff_rank_id WHERE p.id = $1`, sess.PlayerID).Scan(&myLevel)
		all, _ := staff.Ranks(ctx, d.Pool)
		for _, rk := range all {
			if rk.Level < myLevel || myLevel >= staff.HeadAdminLevel {
				data.Ranks = append(data.Ranks, rk)
			}
		}
		data.Teams, _, _ = staff.Suggestions(ctx, d.Pool)
	}
	d.Render.Render(w, "applications.html", data)
}

func appID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id, err == nil
}

func (d *Deps) ApplicationAction(w http.ResponseWriter, r *http.Request) {
	id, ok := appID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !d.can(r, "applications.decide") {
		d.denyPerm(w, r, "Accept or reject applications")
		return
	}
	back := fmt.Sprintf("/admin/applications?id=%d&status=%s", id, r.FormValue("status_tab"))
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form", http.StatusBadRequest)
		return
	}
	var err error
	var msg string
	switch r.FormValue("action") {
	case "interview":
		err, msg = applications.ToInterview(r.Context(), d.Pool, appActor(r), id), "Moved to interview. They've been sent a Discord message if linked."
	case "save_interview", "pass", "fail":
		form := map[string]string{}
		for _, q := range applications.InterviewQuestions {
			form[q.Key] = r.FormValue("iv_" + q.Key)
		}
		outcome := r.FormValue("outcome")
		err, msg = applications.SaveInterview(r.Context(), d.Pool, appActor(r), id, form, outcome), "Interview saved."
	case "accept":
		rankID, _ := strconv.Atoi(r.FormValue("rank"))
		err = applications.AcceptStaff(r.Context(), d.Pool, appActor(r), id, rankID, r.FormValue("team"), r.FormValue("note"))
		msg = "Accepted. They're on the staff team now, and it's logged in the Staff Log."
	case "reject":
		err, msg = applications.RejectStaff(r.Context(), d.Pool, appActor(r), id, r.FormValue("note")), "Rejected. They can apply again after the cooldown."
	default:
		err = fmt.Errorf("%w: unknown action", applications.ErrNotAllowed)
	}
	finishAppAction(w, r, back, err, msg)
}

// ---- Faction command: recruits ----

func (d *Deps) CommandDecideApp(w http.ResponseWriter, r *http.Request) {
	faction := chi.URLParam(r, "faction")
	id, ok := appID(r)
	if !ok || !factions.Valid(faction) {
		http.NotFound(w, r)
		return
	}
	accept := r.FormValue("decision") == "accept"
	err := applications.DecideFaction(r.Context(), d.Pool, appActor(r), id, accept, r.FormValue("note"))
	msg := "Not accepted. They've been told, with your reason."
	if accept {
		msg = "Accepted and recruited at the entry rank. Logged in the Command log."
	}
	finishAppAction(w, r, "/command/"+faction+"/recruits", err, msg)
}
