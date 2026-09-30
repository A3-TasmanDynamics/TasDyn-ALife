package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"website/internal/applications"
	"website/internal/auth"
	"website/internal/factions"
)

// Faction command: Recruits & training, Discipline, Divisions & quals and
// Ranks & gear (layout plan "Police command"). The rules live in
// internal/factions; every action re-checks the viewer's command authority
// there, so a stale page can't do more than they're allowed now.

// postFaction resolves the faction for a POST, or 404s.
func postFaction(w http.ResponseWriter, r *http.Request) (string, bool) {
	f := chi.URLParam(r, "faction")
	if !factions.Valid(f) {
		http.NotFound(w, r)
		return "", false
	}
	return f, true
}

func formInt(r *http.Request, key string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue(key)), 10, 64)
	return n
}

func urlID(r *http.Request, key string) int64 {
	n, _ := strconv.ParseInt(chi.URLParam(r, key), 10, 64)
	return n
}

// findPlayer resolves a Steam64 ID or exact in-game name to a player ID.
func (d *Deps) findPlayer(r *http.Request, who string) (int64, error) {
	who = strings.TrimSpace(who)
	var id int64
	var err error
	switch {
	case who == "":
		return 0, fmt.Errorf("%w: enter their Steam64 ID or exact in-game name", factions.ErrNotAllowed)
	case steam64.MatchString(who):
		err = d.Pool.QueryRow(r.Context(), `SELECT id FROM players WHERE uid = $1`, who).Scan(&id)
	default:
		var n int
		err = d.Pool.QueryRow(r.Context(), `SELECT COALESCE(min(id), 0), count(*) FROM players WHERE lower(name) = lower($1)`, who).Scan(&id, &n)
		if err == nil && n > 1 {
			return 0, fmt.Errorf("%w: more than one player is called that; use their Steam64 ID", factions.ErrNotAllowed)
		}
	}
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && id == 0) {
		return 0, fmt.Errorf("%w: no player found; they need to have joined the server at least once", factions.ErrNotAllowed)
	}
	return id, err
}

// ----- Recruits & training -----

type commandRecruitsData struct {
	commandBase
	View       string // apps / prob / sheet / done
	Apps       []applications.FactionApp
	Probations []factions.Probation
	Finished   []factions.Probation
	Sheet      *factions.Probation
	FTOs       []factions.Member
	Days       int
}

func (d *Deps) CommandRecruits(w http.ResponseWriter, r *http.Request) {
	cb, ok := d.commandAccess(w, r, "recruits")
	if !ok {
		return
	}
	ctx := r.Context()
	q := r.URL.Query()
	data := commandRecruitsData{commandBase: cb, View: q.Get("tab")}
	var err error
	if data.Apps, err = applications.FactionQueue(ctx, d.Pool, cb.Faction); err != nil {
		slog.Error("command: recruits failed", "error", err)
		http.Error(w, "Failed to load applications.", http.StatusInternalServerError)
		return
	}
	if data.Probations, err = factions.Probations(ctx, d.Pool, cb.Faction, true); err != nil {
		slog.Error("command: probations failed", "error", err)
	}
	if set, err := factions.GetSettings(ctx, d.Pool, cb.Faction); err == nil {
		data.Days = set.ProbationDays
	}
	if id, _ := strconv.ParseInt(q.Get("sheet"), 10, 64); id > 0 {
		if p, err := factions.GetProbation(ctx, d.Pool, cb.Faction, id); err == nil {
			data.Sheet, data.View = &p, "sheet"
		}
	}
	switch data.View {
	case "apps", "prob", "sheet", "done":
	default:
		data.View = "prob"
		if cb.OpenApps > 0 || len(data.Probations) == 0 {
			data.View = "apps"
		}
	}
	if data.View == "sheet" && data.Sheet == nil && len(data.Probations) > 0 {
		data.Sheet = &data.Probations[0]
	}
	if data.View == "done" {
		if data.Finished, err = factions.Probations(ctx, d.Pool, cb.Faction, false); err != nil {
			slog.Error("command: finished probations failed", "error", err)
		}
	}
	if data.Sheet != nil && !cb.ReadOnly {
		// FTO candidates: members ranked above the recruit, FTO holders first.
		members, _ := factions.Roster(ctx, d.Pool, cb.Faction)
		held, _ := factions.MemberQuals(ctx, d.Pool, cb.Faction)
		var rest []factions.Member
		for _, m := range members {
			if m.Level <= data.Sheet.Level || m.ID == data.Sheet.PlayerID {
				continue
			}
			if held[m.ID]["FTO"] {
				data.FTOs = append(data.FTOs, m)
			} else {
				rest = append(rest, m)
			}
		}
		data.FTOs = append(data.FTOs, rest...)
	}
	d.Render.Render(w, "command_recruits.html", data)
}

// CommandProbation handles the probation and training sheet actions.
func (d *Deps) CommandProbation(w http.ResponseWriter, r *http.Request) {
	faction, ok := postFaction(w, r)
	if !ok {
		return
	}
	id := urlID(r, "id")
	back := fmt.Sprintf("/command/%s/recruits?sheet=%d", faction, id)
	actor := commandActor(r)
	var err error
	msg := "Saved."
	switch r.FormValue("action") {
	case "training":
		err = factions.SetTraining(r.Context(), d.Pool, actor, faction, id, r.FormValue("item"), r.FormValue("result"))
		msg = "Training sheet updated. Logged in the Command log."
	case "details":
		err = factions.UpdateProbation(r.Context(), d.Pool, actor, faction, id, formInt(r, "fto"), r.FormValue("note"))
		msg = "Probation details saved."
	case "confirm":
		var ch factions.Change
		ch, err = factions.ConfirmProbation(r.Context(), d.Pool, actor, faction, id)
		msg = "Confirmed and promoted. Logged in the Command log; their Discord roles update automatically."
		if err == nil {
			back = fmt.Sprintf("/command/%s/recruits?tab=prob", faction)
			_ = ch
		}
	case "end":
		err = factions.EndProbation(r.Context(), d.Pool, actor, faction, id, r.FormValue("reason"))
		msg = "Probation ended and they've been removed. Recorded under Discipline → Discharges."
		if err == nil {
			back = fmt.Sprintf("/command/%s/recruits?tab=prob", faction)
		}
	default:
		http.Error(w, "Unknown action.", http.StatusBadRequest)
		return
	}
	finishCommandAction(w, r, back, err, msg)
}

// ----- Discipline -----

type officerOption struct {
	factions.Member
	Standing factions.Standing
}

type offenceGroup struct {
	Label    string
	Offences []factions.Offence
}

type commandDisciplineData struct {
	commandBase
	View       string // log / dis / bl
	Who        int64
	Officers   []officerOption
	Offences   []offenceGroup
	Ladder     []factions.Threshold
	Settings   factions.Settings
	Entries    []factions.Entry
	Discharges []factions.DischargeRow
	Blacklist  []factions.Blacklisting
	Types      []struct{ Key, Label string }
	CanLift    bool
}

func (d *Deps) CommandDiscipline(w http.ResponseWriter, r *http.Request) {
	cb, ok := d.commandAccess(w, r, "discipline")
	if !ok {
		return
	}
	ctx := r.Context()
	q := r.URL.Query()
	data := commandDisciplineData{commandBase: cb, View: q.Get("tab"), Ladder: factions.Ladder, Types: factions.DischargeTypes}
	data.Who, _ = strconv.ParseInt(q.Get("who"), 10, 64)
	if data.View != "dis" && data.View != "bl" {
		data.View = "log"
	}
	var err error
	if data.Settings, err = factions.GetSettings(ctx, d.Pool, cb.Faction); err != nil {
		slog.Error("command: settings failed", "error", err)
	}
	if !cb.ReadOnly {
		sess, _ := auth.FromContext(ctx)
		members, _ := factions.Roster(ctx, d.Pool, cb.Faction)
		standing, _ := factions.Standings(ctx, d.Pool, cb.Faction)
		for _, m := range members {
			if m.ID != sess.PlayerID && cb.Command.CanDiscipline(m.Level) {
				o := officerOption{Member: m, Standing: standing[m.ID]}
				if st, err := factions.StandingOf(ctx, d.Pool, cb.Faction, m.ID); err == nil {
					o.Standing = st
				}
				data.Officers = append(data.Officers, o)
			}
		}
		offs, err := factions.Offences(ctx, d.Pool, cb.Faction)
		if err != nil {
			slog.Error("command: offences failed", "error", err)
		}
		low, high := offenceGroup{Label: "Low tier"}, offenceGroup{Label: "High tier"}
		for _, o := range offs {
			if o.Tier == "high" {
				high.Offences = append(high.Offences, o)
			} else {
				low.Offences = append(low.Offences, o)
			}
		}
		data.Offences = []offenceGroup{low, high}
		data.CanLift = cb.Command.Cabinet
	}
	switch data.View {
	case "log":
		data.Entries, err = factions.DisciplineLog(ctx, d.Pool, cb.Faction, 0, 200)
	case "dis":
		data.Discharges, err = factions.Discharges(ctx, d.Pool, cb.Faction, 200)
	case "bl":
		data.Blacklist, err = factions.Blacklist(ctx, d.Pool, cb.Faction, 200)
	}
	if err != nil {
		slog.Error("command: discipline list failed", "tab", data.View, "error", err)
	}
	d.Render.Render(w, "command_discipline.html", data)
}

func (d *Deps) CommandDisciplineIssue(w http.ResponseWriter, r *http.Request) {
	faction, ok := postFaction(w, r)
	if !ok {
		return
	}
	who := formInt(r, "who")
	back := fmt.Sprintf("/command/%s/discipline?who=%d", faction, who)
	points, _ := strconv.Atoi(r.FormValue("points"))
	res, err := factions.IssueDiscipline(r.Context(), d.Pool, commandActor(r), faction, factions.Issue{
		TargetID: who, OffenceID: formInt(r, "offence"), MVW: r.FormValue("mvw") == "1", Points: points,
		Notes: r.FormValue("notes"), ApplyAction: r.FormValue("apply") == "1",
	})
	if err != nil {
		finishCommandAction(w, r, back, err, "")
		return
	}
	msg := fmt.Sprintf("Recorded for %s: %d → %d points.", res.Name, res.Before, res.After)
	if res.Converted {
		msg += fmt.Sprintf(" That was their %s warning, so they became %d points.", ordinal(factions.MVWsPerConversion), factions.MVWConversionPoints)
	}
	if res.Crossed != nil && res.Action == "" {
		msg += " This crosses " + strconv.Itoa(res.Crossed.At) + " points (suggested: " + strings.ToLower(res.Crossed.Label) + "); nothing was applied."
	} else if res.Action != "" {
		msg += " " + strings.ToUpper(res.Action[:1]) + res.Action[1:] + "."
	}
	if res.Warning != "" {
		http.Redirect(w, r, back+"&notice="+errMsg(msg)+"&error="+errMsg(res.Warning), http.StatusSeeOther)
		return
	}
	finishCommandAction(w, r, back, nil, msg)
}

func ordinal(n int) string {
	switch n {
	case 1:
		return "first"
	case 2:
		return "second"
	case 3:
		return "third"
	}
	return fmt.Sprintf("%dth", n)
}

func (d *Deps) CommandDisciplineCorrect(w http.ResponseWriter, r *http.Request) {
	faction, ok := postFaction(w, r)
	if !ok {
		return
	}
	err := factions.CorrectDiscipline(r.Context(), d.Pool, commandActor(r), faction, urlID(r, "id"), r.FormValue("reason"))
	finishCommandAction(w, r, "/command/"+faction+"/discipline", err, "Correction recorded. The original entry stays in the log, marked corrected.")
}

func (d *Deps) CommandDischarge(w http.ResponseWriter, r *http.Request) {
	faction, ok := postFaction(w, r)
	if !ok {
		return
	}
	err := factions.Discharge(r.Context(), d.Pool, commandActor(r), faction, formInt(r, "who"), r.FormValue("type"), r.FormValue("notes"))
	finishCommandAction(w, r, "/command/"+faction+"/discipline?tab=dis", err, "Discharged and removed from "+factions.Name(faction)+". Their Discord roles update automatically.")
}

func (d *Deps) CommandBlacklistAdd(w http.ResponseWriter, r *http.Request) {
	faction, ok := postFaction(w, r)
	if !ok {
		return
	}
	back := "/command/" + faction + "/discipline?tab=bl"
	id, err := d.findPlayer(r, r.FormValue("who"))
	if err == nil {
		days, _ := strconv.Atoi(r.FormValue("days"))
		if r.FormValue("permanent") == "1" {
			days = 0
		} else if days <= 0 {
			err = fmt.Errorf("%w: enter how many days, or tick permanent", factions.ErrNotAllowed)
		}
		if err == nil {
			err = factions.AddBlacklist(r.Context(), d.Pool, commandActor(r), faction, id, days, r.FormValue("reason"))
		}
	}
	finishCommandAction(w, r, back, err, "Blacklisted. They can't apply or be recruited until it ends.")
}

func (d *Deps) CommandBlacklistLift(w http.ResponseWriter, r *http.Request) {
	faction, ok := postFaction(w, r)
	if !ok {
		return
	}
	err := factions.LiftBlacklist(r.Context(), d.Pool, commandActor(r), faction, urlID(r, "id"), r.FormValue("reason"))
	finishCommandAction(w, r, "/command/"+faction+"/discipline?tab=bl", err, "Blacklist lifted. They can apply again.")
}

// ----- Divisions & quals -----

type divisionCard struct {
	factions.Division
	Count int
	Lead  string
}

type divisionMember struct {
	factions.Member
	Role    string
	CanEdit bool
	Auto    bool // Administration by being cabinet
}

type qualRow struct {
	factions.Qual
	Holders []factions.Member
}

type matrixRow struct {
	Member factions.Member
	Cells  []matrixCell
}

type matrixCell struct {
	Key  string
	Held bool
	Head bool
}

type commandDivisionsData struct {
	commandBase
	Divisions []divisionCard
	Cur       *divisionCard
	Members   []divisionMember
	Addable   []factions.Member
	General   int // members in no specialist division
	Quals     []qualRow
	Matrix    []matrixRow

	CanAppoint bool     // may add members to the current division
	Roles      []string // roles the viewer may give in it
	EntryRole  string
}

func (d *Deps) CommandDivisions(w http.ResponseWriter, r *http.Request) {
	cb, ok := d.commandAccess(w, r, "divisions")
	if !ok {
		return
	}
	ctx := r.Context()
	data := commandDivisionsData{commandBase: cb}
	divs, err := factions.Divisions(ctx, d.Pool, cb.Faction)
	if err != nil {
		slog.Error("command: divisions failed", "error", err)
	}
	members, _ := factions.Roster(ctx, d.Pool, cb.Faction)
	posts, _ := factions.MemberDivisions(ctx, d.Pool, cb.Faction)
	adminPosts, _ := factions.AdminPostings(ctx, d.Pool, cb.Faction)
	held, _ := factions.MemberQuals(ctx, d.Pool, cb.Faction)
	quals, _ := factions.Quals(ctx, d.Pool, cb.Faction)
	ranks, _ := factions.Ranks(ctx, d.Pool, cb.Faction)
	postingIn := func(dv factions.Division, id int64) (factions.Posting, bool) {
		if dv.IsAdmin {
			p, ok := adminPosts[id]
			return p, ok
		}
		p, ok := posts[id]
		return p, ok && p.Key == dv.Key
	}
	for _, dv := range divs {
		c := divisionCard{Division: dv}
		for _, m := range members {
			if p, ok := postingIn(dv, m.ID); ok {
				c.Count++
				if p.Role == dv.CommanderRole() && c.Lead == "" {
					c.Lead = m.Name
				}
			}
		}
		data.Divisions = append(data.Divisions, c)
	}
	for _, m := range members {
		if _, ok := posts[m.ID]; !ok {
			data.General++
		}
	}
	want := r.URL.Query().Get("div")
	for i := range data.Divisions {
		if data.Divisions[i].Key == want {
			data.Cur = &data.Divisions[i]
		}
	}
	if data.Cur == nil {
		for i := range data.Divisions { // first specialist division by default
			if !data.Divisions[i].IsAdmin {
				data.Cur = &data.Divisions[i]
				break
			}
		}
	}
	if data.Cur == nil && len(data.Divisions) > 0 {
		data.Cur = &data.Divisions[0]
	}
	sess, _ := auth.FromContext(ctx)
	if cur := data.Cur; cur != nil {
		// Who can change postings in this division, and which roles they
		// can give (only cabinet and Management appoint the Administration
		// Commander).
		canEdit := func(m factions.Member) bool {
			if m.ID == sess.PlayerID && !cb.Command.Cabinet && !cb.Management {
				return false // only cabinet and Management keep their own records
			}
			if cur.IsAdmin {
				return cb.Management || cb.Command.CanAppointAdmin()
			}
			return !cb.ReadOnly && cb.Command.CanRecord(m.Level, factions.RankFor(ranks, m.Level).IsCabinet)
		}
		data.Roles = cur.Roles
		if cur.IsAdmin && !cb.Management && !cb.Command.Cabinet {
			data.Roles = cur.Roles[1:]
		}
		data.EntryRole = cur.EntryRole()
		order := map[string]int{}
		for i, role := range cur.Roles {
			order[role] = i
		}
		for _, m := range members {
			if p, ok := postingIn(cur.Division, m.ID); ok {
				dm := divisionMember{Member: m, Role: p.Role, CanEdit: canEdit(m) && !p.Auto, Auto: p.Auto}
				if cur.IsAdmin && p.Role == cur.CommanderRole() && !cb.Management && !cb.Command.Cabinet {
					dm.CanEdit = false
				}
				data.Members = append(data.Members, dm)
			} else if canEdit(m) && m.Level >= cur.MinLevel && (cur.RequiredQual == "" || held[m.ID][cur.RequiredQual]) {
				data.Addable = append(data.Addable, m)
			}
		}
		if cur.IsAdmin {
			data.CanAppoint = cb.Management || cb.Command.CanAppointAdmin()
		} else {
			data.CanAppoint = !cb.ReadOnly
		}
		// Most senior role first, then rank.
		for i := 1; i < len(data.Members); i++ {
			for j := i; j > 0; j-- {
				a, b := data.Members[j-1], data.Members[j]
				if order[a.Role] > order[b.Role] || (order[a.Role] == order[b.Role] && a.Level < b.Level) {
					data.Members[j-1], data.Members[j] = b, a
				}
			}
		}
	}
	for _, q := range quals {
		row := qualRow{Qual: q}
		for _, m := range members {
			if held[m.ID][q.Key] {
				row.Holders = append(row.Holders, m)
			}
		}
		data.Quals = append(data.Quals, row)
	}
	for _, m := range members {
		row := matrixRow{Member: m}
		for _, q := range quals {
			row.Cells = append(row.Cells, matrixCell{Key: q.Key, Held: held[m.ID][q.Key], Head: q.HeadID == m.ID})
		}
		data.Matrix = append(data.Matrix, row)
	}
	d.Render.Render(w, "command_divisions.html", data)
}

func (d *Deps) CommandDivisionAction(w http.ResponseWriter, r *http.Request) {
	faction, ok := postFaction(w, r)
	if !ok {
		return
	}
	key := chi.URLParam(r, "key")
	back := "/command/" + faction + "/divisions?div=" + key
	who := formInt(r, "who")
	actor := d.panelActor(r, faction)
	var err error
	msg := "Saved. Logged in the Command log."
	switch r.FormValue("action") {
	case "add", "role":
		err = factions.SetDivision(r.Context(), d.Pool, actor, faction, who, key, r.FormValue("role"), false, r.FormValue("reason"))
	case "remove":
		err = factions.SetDivision(r.Context(), d.Pool, actor, faction, who, key, "", true, r.FormValue("reason"))
		msg = "Removed from the division."
	default:
		http.Error(w, "Unknown action.", http.StatusBadRequest)
		return
	}
	finishCommandAction(w, r, back, err, msg)
}

func (d *Deps) CommandQualHead(w http.ResponseWriter, r *http.Request) {
	faction, ok := postFaction(w, r)
	if !ok {
		return
	}
	err := factions.SetQualHead(r.Context(), d.Pool, commandActor(r), faction, chi.URLParam(r, "key"), formInt(r, "head"))
	finishCommandAction(w, r, "/command/"+faction+"/divisions#quals", err, "Head trainer updated.")
}

func (d *Deps) CommandMemberQual(w http.ResponseWriter, r *http.Request) {
	faction, ok := postFaction(w, r)
	if !ok {
		return
	}
	id := urlID(r, "id")
	grant := r.FormValue("grant") == "1"
	err := factions.SetQual(r.Context(), d.Pool, commandActor(r), faction, id, r.FormValue("qual"), grant, r.FormValue("note"))
	msg := "Qualification recorded."
	if !grant {
		msg = "Qualification removed."
	}
	finishCommandAction(w, r, fmt.Sprintf("/command/%s/members/%d#quals", faction, id), err, msg)
}

// ----- Ranks & gear -----

type factionRankRow struct {
	factions.Rank
	Filled int
	UpTo   string
}

type commandRanksData struct {
	commandBase
	Ranks       []factionRankRow // highest first
	Sel         *factionRankRow
	Quals       []factions.Qual
	UpTo        []factions.Rank // options for "promotes up to"
	CanEdit     bool
	CanEditType bool // Command / Cabinet ticks and "promotes up to"
	Why         string
	Settings    factions.Settings
	CanConfig   bool // top rank: settings
}

func (d *Deps) CommandRanks(w http.ResponseWriter, r *http.Request) {
	cb, ok := d.commandAccess(w, r, "ranks")
	if !ok {
		return
	}
	ctx := r.Context()
	data := commandRanksData{commandBase: cb}
	ranks, err := factions.Ranks(ctx, d.Pool, cb.Faction)
	if err != nil {
		slog.Error("command: ranks failed", "error", err)
	}
	members, _ := factions.Roster(ctx, d.Pool, cb.Faction)
	filled := map[int]int{}
	for _, m := range members {
		filled[m.Level]++
	}
	sel, _ := strconv.Atoi(r.URL.Query().Get("level"))
	for i := len(ranks) - 1; i >= 0; i-- {
		rk := ranks[i]
		row := factionRankRow{Rank: rk, Filled: filled[rk.Level], UpTo: "—"}
		if rk.CanPromote() {
			row.UpTo = factions.RankFor(ranks, rk.PromoteUpTo).Label()
		}
		data.Ranks = append(data.Ranks, row)
	}
	if sel == 0 && len(ranks) > 0 {
		sel = ranks[0].Level
		if cb.Command.Level > 1 {
			sel = cb.Command.Level - 1
		}
	}
	for i := range data.Ranks {
		if data.Ranks[i].Level == sel {
			data.Sel = &data.Ranks[i]
		}
	}
	data.Quals, _ = factions.Quals(ctx, d.Pool, cb.Faction)
	data.Settings, _ = factions.GetSettings(ctx, d.Pool, cb.Faction)
	if data.Sel != nil {
		for l := data.Sel.Level - 1; l >= 1; l-- {
			data.UpTo = append(data.UpTo, factions.RankFor(ranks, l))
		}
		switch {
		case cb.Management:
			data.CanEdit, data.CanEditType = true, true
		case !cb.ReadOnly && cb.Command.CanEditRank(data.Sel.Level):
			data.CanEdit, data.CanEditType = true, cb.Command.CanEditRankType()
		case cb.ReadOnly:
			data.Why = "You're viewing as staff (read-only)."
		default:
			data.Why = "You can't edit your own rank's rules; cabinet can."
		}
	}
	data.CanConfig = !cb.ReadOnly
	d.Render.Render(w, "command_ranks.html", data)
}

func (d *Deps) CommandRankSave(w http.ResponseWriter, r *http.Request) {
	faction, ok := postFaction(w, r)
	if !ok {
		return
	}
	level, _ := strconv.Atoi(chi.URLParam(r, "level"))
	back := fmt.Sprintf("/command/%s/ranks?level=%d", faction, level)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form.", http.StatusBadRequest)
		return
	}
	atoi := func(k string) int { n, _ := strconv.Atoi(strings.TrimSpace(r.FormValue(k))); return n }
	rr := factions.RankRules{Name: r.FormValue("name"), Short: r.FormValue("short"), Slots: atoi("slots"), MinDays: atoi("min_days"),
		PromoteUpTo: atoi("up_to"), Quals: r.Form["quals"], Description: r.FormValue("description"),
		Command: r.FormValue("is_command") == "1", Cabinet: r.FormValue("is_cabinet") == "1"}
	// Management edits as a staff override unless they're faction cabinet.
	actor := commandActor(r)
	if c, _, _ := factions.CommandIn(r.Context(), d.Pool, actor.PlayerID, faction); !c.Cabinet && d.can(r, "factions.configure") {
		actor.Via = factions.ViaStaffOverride
	}
	err := factions.UpdateRank(r.Context(), d.Pool, actor, faction, level, rr)
	finishCommandAction(w, r, back, err, "Rank saved. Logged in the Command log with the before and after values.")
}

func (d *Deps) CommandSettingsSave(w http.ResponseWriter, r *http.Request) {
	faction, ok := postFaction(w, r)
	if !ok {
		return
	}
	atoi := func(k string) int { n, _ := strconv.Atoi(strings.TrimSpace(r.FormValue(k))); return n }
	err := factions.UpdateSettings(r.Context(), d.Pool, commandActor(r), faction, factions.Settings{
		ProbationDays: atoi("probation_days"), PointsExpiryDays: atoi("points_expiry_days"), MVWDays: atoi("mvw_days"), BlacklistDays: atoi("blacklist_days"),
	}, r.FormValue("reason"))
	finishCommandAction(w, r, "/command/"+faction+"/ranks#settings", err, "Settings saved. New entries use them; existing ones keep their dates.")
}
