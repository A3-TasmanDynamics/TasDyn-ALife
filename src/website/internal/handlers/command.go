package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"website/internal/audit"
	"website/internal/auth"
	"website/internal/factions"
)

// The faction command panel (layout plan "Police command", GAMEPANEL_PARITY
// §6.1). Access comes from the player's faction rank -- a rank with command
// authority set on Roles → Faction rank names -- never from a staff role.
// Staff with factions.audit can look (read-only) but not act.

type commandShell struct {
	Faction     string
	FactionName string
	Tab         string
	Command     factions.Command // zero when viewing read-only as staff
	ReadOnly    bool
	AuthorityTo string // label of the highest rank they can set
	MemberCount int
	OpenApps    int // pending faction applications
	// Management: staff with factions.configure. They can appoint to the
	// Administration division and edit rank rules as a staff override.
	Management bool
}

type commandBase struct {
	Base
	commandShell
}

// commandAccess resolves the faction from the URL and the viewer's access,
// rendering the denied page itself when there's none.
func (d *Deps) commandAccess(w http.ResponseWriter, r *http.Request, tab string) (commandBase, bool) {
	faction := chi.URLParam(r, "faction")
	if !factions.Valid(faction) {
		http.NotFound(w, r)
		return commandBase{}, false
	}
	sess, _ := auth.FromContext(r.Context())
	cb := commandBase{
		Base:         baseFrom(r, factions.Name(faction)+" command"),
		commandShell: commandShell{Faction: faction, FactionName: factions.Name(faction), Tab: tab},
	}
	c, ok, err := factions.CommandIn(r.Context(), d.Pool, sess.PlayerID, faction)
	if err != nil {
		slog.Error("command: access check failed", "error", err)
		http.Error(w, "Couldn't check your access. Please try again.", http.StatusInternalServerError)
		return cb, false
	}
	switch {
	case ok:
		cb.Command = c
	case d.can(r, "factions.audit"):
		cb.ReadOnly = true
	default:
		d.Denied(w, r, auth.DeniedInfo{Kind: "not_command", Area: faction})
		return cb, false
	}
	cb.Management = d.can(r, "factions.configure")
	if ranks, err := factions.Ranks(r.Context(), d.Pool, faction); err == nil && cb.Command.Authority > 0 {
		cb.AuthorityTo = factions.RankFor(ranks, cb.Command.Authority).Label()
	}
	col := map[string]string{"police": "cop_level", "ems": "medic_level"}[faction]
	_ = d.Pool.QueryRow(r.Context(), `SELECT count(*) FROM players WHERE `+col+` > 0`).Scan(&cb.MemberCount)
	_ = d.Pool.QueryRow(r.Context(), `SELECT count(*) FROM faction_applications WHERE faction = $1 AND status = 'pending'`, faction).Scan(&cb.OpenApps)
	return cb, true
}

// CommandHome sends a commander to their faction's panel.
func (d *Deps) CommandHome(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.FromContext(r.Context())
	cmds, err := factions.CommandOf(r.Context(), d.Pool, sess.PlayerID)
	if err != nil {
		slog.Error("command: lookup failed", "error", err)
		http.Error(w, "Couldn't check your access. Please try again.", http.StatusInternalServerError)
		return
	}
	if len(cmds) > 0 {
		http.Redirect(w, r, "/command/"+cmds[0].Faction, http.StatusSeeOther)
		return
	}
	if d.can(r, "factions.audit") {
		http.Redirect(w, r, "/command/police", http.StatusSeeOther)
		return
	}
	d.Denied(w, r, auth.DeniedInfo{Kind: "not_command"})
}

type strengthRow struct {
	Rank    factions.Rank
	Filled  int
	Percent int
	Full    bool
}

type commandOverviewData struct {
	commandBase
	Online    int
	OpenSlots int
	HasSlots  bool
	Strength  []strengthRow
	Recent    []factions.LogEntry
}

func (d *Deps) CommandOverview(w http.ResponseWriter, r *http.Request) {
	cb, ok := d.commandAccess(w, r, "overview")
	if !ok {
		return
	}
	data := commandOverviewData{commandBase: cb}
	ctx := r.Context()
	ranks, err := factions.Ranks(ctx, d.Pool, cb.Faction)
	if err != nil {
		slog.Error("command: ranks failed", "error", err)
	}
	members, err := factions.Roster(ctx, d.Pool, cb.Faction)
	if err != nil {
		slog.Error("command: roster failed", "error", err)
		http.Error(w, "Failed to load the roster.", http.StatusInternalServerError)
		return
	}
	filled := map[int]int{}
	for _, m := range members {
		filled[m.Level]++
		if m.Online {
			data.Online++
		}
	}
	for i := len(ranks) - 1; i >= 0; i-- {
		rk := ranks[i]
		row := strengthRow{Rank: rk, Filled: filled[rk.Level]}
		if rk.Slots > 0 {
			data.HasSlots = true
			row.Percent = min(100, row.Filled*100/rk.Slots)
			row.Full = row.Filled >= rk.Slots
			data.OpenSlots += max(0, rk.Slots-row.Filled)
		}
		data.Strength = append(data.Strength, row)
	}
	if data.Recent, err = factions.Log(ctx, d.Pool, cb.Faction, "", "", 0, 8); err != nil {
		slog.Error("command: log failed", "error", err)
	}
	d.Render.Render(w, "command_overview.html", data)
}

type commandRosterData struct {
	commandBase
	Q          string
	Rows       []factions.Personnel
	Total      int
	CanRecruit bool
	Levels     []factions.Rank // ranks the viewer can recruit into
	Ranks      map[int]factions.Rank
}

// CommandRoster is the roster: rank, badge, name, division and status.
// Everything else is on each member's profile.
func (d *Deps) CommandRoster(w http.ResponseWriter, r *http.Request) {
	cb, ok := d.commandAccess(w, r, "roster")
	if !ok {
		return
	}
	ctx := r.Context()
	data := commandRosterData{commandBase: cb, Q: strings.TrimSpace(r.URL.Query().Get("q"))}
	ranks, _ := factions.Ranks(ctx, d.Pool, cb.Faction)
	data.Ranks = map[int]factions.Rank{}
	for _, rk := range ranks {
		data.Ranks[rk.Level] = rk
	}
	rows, err := factions.PersonnelRoster(ctx, d.Pool, cb.Faction, time.Now())
	if err != nil {
		slog.Error("command: roster failed", "error", err)
		http.Error(w, "Failed to load the roster.", http.StatusInternalServerError)
		return
	}
	data.Total = len(rows)
	needle := strings.ToLower(data.Q)
	for _, p := range rows {
		if needle != "" && !strings.Contains(strings.ToLower(p.Name+" "+p.Badge+" "+p.Department+" "+p.Rank.Label()), needle) {
			continue
		}
		data.Rows = append(data.Rows, p)
	}
	if !cb.ReadOnly && cb.Command.Authority > 0 {
		data.CanRecruit = true
		data.Levels = assignable(ranks, cb.Command.Authority)
	}
	d.Render.Render(w, "command_roster.html", data)
}

// CommandPersonnelSave saves a member's personnel file or a roll call mark,
// from their profile.
func (d *Deps) CommandPersonnelSave(w http.ResponseWriter, r *http.Request) {
	faction := chi.URLParam(r, "faction")
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || !factions.Valid(faction) {
		http.NotFound(w, r)
		return
	}
	back := fmt.Sprintf("/command/%s/members/%d", faction, id)
	switch r.FormValue("action") {
	case "rollcall":
		month, perr := time.Parse("2006-01", r.FormValue("month"))
		if perr != nil {
			finishCommandAction(w, r, back+"#rollcall", fmt.Errorf("%w: pick a month", factions.ErrNotAllowed), "")
			return
		}
		mark := r.FormValue("mark")
		err = factions.SetRollCall(r.Context(), d.Pool, commandActor(r), faction, id, month, mark)
		word := map[string]string{"present": "Marked present", "excused": "Marked excused", "": "Roll call mark cleared"}[mark]
		finishCommandAction(w, r, back+"#rollcall", err, word+" for "+month.Format("January 2006")+".")
	default:
		var changed []string
		changed, err = factions.UpdateMember(r.Context(), d.Pool, commandActor(r), faction, id, factions.MemberDetails{
			Badge: r.FormValue("badge"), Region: r.FormValue("region"), Status: r.FormValue("status"),
			Notes: r.FormValue("notes"), Enrolled: r.FormValue("enrolled"),
		})
		msg := "Nothing changed."
		if len(changed) > 0 {
			msg = "Saved: " + strings.Join(changed, "; ") + "."
		}
		finishCommandAction(w, r, back+"#file", err, msg)
	}
}

// assignable lists the ranks (lowest first) someone with this authority can
// set; configured ranks only, or bare levels when none are configured.
func assignable(ranks []factions.Rank, authority int) []factions.Rank {
	var out []factions.Rank
	for l := 1; l <= authority; l++ {
		out = append(out, factions.RankFor(ranks, l))
	}
	return out
}

type commandMemberData struct {
	commandBase
	Member    factions.Member
	History   []factions.LogEntry
	CanChange bool
	Levels    []factions.Rank
	Why       string // why the viewer can't change this member

	// Command records (Discipline, Divisions & quals, Recruits & training).
	Standing   factions.Standing
	Band       *factions.Threshold
	Discipline []factions.Entry
	Quals      []factions.HeldQual
	AllQuals   []factions.Qual
	Division   string // "S.R.G. · Operator"
	Probation  *factions.Probation
	CanAct     bool // personnel file, roll call, certifications (records)
	CanDisc    bool // discipline

	// Personnel file.
	File     factions.Personnel
	RankInfo factions.Rank
	RollCall []factions.RollCallMonth
	Regions  []string
	Statuses []struct{ Key, Label string }
	Initials string
	Held     int // certifications held
}

func (d *Deps) CommandMember(w http.ResponseWriter, r *http.Request) {
	cb, ok := d.commandAccess(w, r, "roster")
	if !ok {
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	members, err := factions.Roster(ctx, d.Pool, cb.Faction)
	if err != nil {
		http.Error(w, "Failed to load the roster.", http.StatusInternalServerError)
		return
	}
	data := commandMemberData{commandBase: cb}
	found := false
	for _, m := range members {
		if m.ID == id {
			data.Member, found = m, true
		}
	}
	if !found {
		http.Redirect(w, r, "/command/"+cb.Faction+"/roster?error="+errMsg("That player isn't in "+cb.FactionName+"."), http.StatusSeeOther)
		return
	}
	data.Title = data.Member.Name + " · " + cb.FactionName + " command"
	history, err := factions.Log(ctx, d.Pool, cb.Faction, "", "", id, 80)
	if err != nil {
		slog.Error("command: member history failed", "error", err)
	}
	for _, e := range history {
		if e.Kind == "discipline" { // shown in its own card
			continue
		}
		// On their own profile, "Kaz_N: roll call present" reads as "Roll call present".
		if d := strings.TrimPrefix(strings.TrimPrefix(e.Detail, data.Member.Name+": "), data.Member.Name+" "); d != e.Detail && d != "" {
			e.Detail = strings.ToUpper(d[:1]) + d[1:]
		}
		data.History = append(data.History, e)
	}
	sess, _ := auth.FromContext(ctx)
	if data.Standing, err = factions.StandingOf(ctx, d.Pool, cb.Faction, id); err != nil {
		slog.Error("command: standing failed", "error", err)
	}
	data.Band = factions.Band(data.Standing.Points)
	data.Discipline, _ = factions.DisciplineLog(ctx, d.Pool, cb.Faction, id, 50)
	data.Quals, _ = factions.QualsOf(ctx, d.Pool, cb.Faction, id)
	data.AllQuals, _ = factions.Quals(ctx, d.Pool, cb.Faction)
	if posts, err := factions.MemberDivisions(ctx, d.Pool, cb.Faction); err == nil {
		if p, ok := posts[id]; ok {
			divs, _ := factions.Divisions(ctx, d.Pool, cb.Faction)
			for _, dv := range divs {
				if dv.Key == p.Key {
					data.Division = dv.Name + " · " + p.Role
				}
			}
		}
	}
	if ps, err := factions.Probations(ctx, d.Pool, cb.Faction, true); err == nil {
		for i := range ps {
			if ps[i].PlayerID == id {
				data.Probation = &ps[i]
			}
		}
	}
	ranksAll, _ := factions.Ranks(ctx, d.Pool, cb.Faction)
	if !cb.ReadOnly {
		self := id == sess.PlayerID
		// Cabinet can keep their own records; nobody disciplines themselves.
		data.CanAct = (!self || cb.Command.Cabinet) && cb.Command.CanRecord(data.Member.Level, factions.RankFor(ranksAll, data.Member.Level).IsCabinet)
		data.CanDisc = !self && cb.Command.CanDiscipline(data.Member.Level)
	}
	data.Regions, data.Statuses = factions.Regions, factions.Statuses
	data.Initials = initials(data.Member.Name)
	if rows, err := factions.PersonnelRoster(ctx, d.Pool, cb.Faction, time.Now()); err == nil {
		for _, p := range rows {
			if p.ID == id {
				data.File = p
			}
		}
	} else {
		slog.Error("command: personnel file failed", "error", err)
	}
	if ranks, err := factions.Ranks(ctx, d.Pool, cb.Faction); err == nil {
		data.RankInfo = factions.RankFor(ranks, data.Member.Level)
	}
	if data.RollCall, err = factions.RollCallHistory(ctx, d.Pool, cb.Faction, id, 6); err != nil {
		slog.Error("command: roll call history failed", "error", err)
	}
	for _, q := range data.AllQuals {
		if data.File.Quals[q.Key] {
			data.Held++
		}
	}
	switch {
	case cb.ReadOnly:
		data.Why = "You're viewing as staff (read-only). Management can override ranks from Player Lookup."
	case id == sess.PlayerID:
		data.Why = "You can't change your own rank."
	case cb.Command.Authority == 0:
		data.Why = "Your rank can't change ranks. It needs a \"promotes up to\" rank on Ranks & gear."
	case data.Member.Level > cb.Command.Authority:
		data.Why = fmt.Sprintf("%s is above what you can change (up to %s).", data.Member.Rank.Label(), cb.AuthorityTo)
	default:
		data.CanChange = true
		ranks, _ := factions.Ranks(ctx, d.Pool, cb.Faction)
		data.Levels = assignable(ranks, cb.Command.Authority)
	}
	d.Render.Render(w, "command_member.html", data)
}

// finishCommandAction redirects with the result of a roster change.
func finishCommandAction(w http.ResponseWriter, r *http.Request, back string, err error, success string) {
	switch {
	case err == nil:
		http.Redirect(w, r, back+sep(back)+"notice="+errMsg(success), http.StatusSeeOther)
	case errors.Is(err, factions.ErrNotAllowed):
		msg := strings.TrimPrefix(err.Error(), factions.ErrNotAllowed.Error()+": ")
		http.Redirect(w, r, back+sep(back)+"error="+errMsg(strings.ToUpper(msg[:1])+msg[1:]+"."), http.StatusSeeOther)
	default:
		slog.Error("command action failed", "path", r.URL.Path, "error", err)
		http.Redirect(w, r, back+sep(back)+"error="+errMsg("Something went wrong. Nothing was changed."), http.StatusSeeOther)
	}
}

// panelActor is the authority to act under for panel maintenance and
// Administration appointments: faction access (command, cabinet or
// Administration) if they have it, otherwise Management's staff override.
func (d *Deps) panelActor(r *http.Request, faction string) factions.Actor {
	a := commandActor(r)
	if _, ok, err := factions.CommandIn(r.Context(), d.Pool, a.PlayerID, faction); (err != nil || !ok) && d.can(r, "factions.configure") {
		a.Via = factions.ViaStaffOverride
	}
	return a
}

func commandActor(r *http.Request) factions.Actor {
	sess, _ := auth.FromContext(r.Context())
	return factions.Actor{PlayerID: sess.PlayerID, Source: audit.SourceWebsite, Via: factions.ViaCommand}
}

var kindWords = map[string]string{"recruit": "Recruited", "promote": "Promoted", "demote": "Demoted", "remove": "Removed from the faction"}

// CommandSetRank promotes, demotes or removes a member. factions.SetLevel
// re-checks the viewer's command authority, so a stale page can't do more
// than they're allowed now.
func (d *Deps) CommandSetRank(w http.ResponseWriter, r *http.Request) {
	faction := chi.URLParam(r, "faction")
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || !factions.Valid(faction) {
		http.NotFound(w, r)
		return
	}
	back := fmt.Sprintf("/command/%s/members/%d", faction, id)
	level, err := strconv.Atoi(r.FormValue("level"))
	if err != nil {
		finishCommandAction(w, r, back, fmt.Errorf("%w: pick a rank", factions.ErrNotAllowed), "")
		return
	}
	ch, err := factions.SetLevel(r.Context(), d.Pool, commandActor(r), faction, id, level, r.FormValue("reason"))
	if err == nil && ch.Kind == "remove" {
		back = "/command/" + faction + "/roster"
	}
	finishCommandAction(w, r, back, err, kindWords[ch.Kind]+". Logged in the Command log; their Discord roles update automatically.")
}

var steam64 = regexp.MustCompile(`^\d{17}$`)

// CommandRecruit adds a player to the faction by Steam64 or exact name.
func (d *Deps) CommandRecruit(w http.ResponseWriter, r *http.Request) {
	faction := chi.URLParam(r, "faction")
	if !factions.Valid(faction) {
		http.NotFound(w, r)
		return
	}
	back := "/command/" + faction + "/roster"
	who := strings.TrimSpace(r.FormValue("who"))
	var id int64
	var err error
	switch {
	case who == "":
		err = fmt.Errorf("%w: enter their Steam64 ID or exact in-game name", factions.ErrNotAllowed)
	case steam64.MatchString(who):
		err = d.Pool.QueryRow(r.Context(), `SELECT id FROM players WHERE uid = $1`, who).Scan(&id)
	default:
		var n int
		err = d.Pool.QueryRow(r.Context(), `SELECT COALESCE(min(id), 0), count(*) FROM players WHERE lower(name) = lower($1)`, who).Scan(&id, &n)
		if err == nil && n > 1 {
			err = fmt.Errorf("%w: more than one player is called that; use their Steam64 ID", factions.ErrNotAllowed)
		} else if err == nil && n == 0 {
			err = pgx.ErrNoRows
		}
	}
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && id == 0) {
		err = fmt.Errorf("%w: no player found; they need to have joined the server at least once", factions.ErrNotAllowed)
	}
	if err != nil {
		finishCommandAction(w, r, back, err, "")
		return
	}
	level, _ := strconv.Atoi(r.FormValue("level"))
	if level <= 0 {
		level = 1
	}
	_, err = factions.SetLevel(r.Context(), d.Pool, commandActor(r), faction, id, level, r.FormValue("reason"))
	if err == nil {
		back = fmt.Sprintf("/command/%s/members/%d", faction, id)
	}
	finishCommandAction(w, r, back, err, "Recruited. Logged in the Command log; their Discord roles update automatically.")
}

type commandLogData struct {
	commandBase
	Kind    string
	Q       string
	Entries []factions.LogEntry
	Kinds   []logKind
}

type logKind struct{ Key, Label string }

// logKinds are the Command log's filters.
var logKinds = []logKind{
	{"recruit", "Recruited"}, {"promote", "Promotions"}, {"demote", "Demotions"}, {"remove", "Removals"},
	{"probation", "Probation"}, {"training", "Training"}, {"discipline", "Discipline"},
	{"blacklist", "Blacklist"}, {"division", "Divisions"}, {"qual", "Quals"}, {"rank_rules", "Rank rules"},
	{"roster", "Roster"}, {"roll_call", "Roll call"},
}

func (d *Deps) CommandLog(w http.ResponseWriter, r *http.Request) {
	cb, ok := d.commandAccess(w, r, "log")
	if !ok {
		return
	}
	data := commandLogData{commandBase: cb, Kind: r.URL.Query().Get("kind"), Q: strings.TrimSpace(r.URL.Query().Get("q"))}
	data.Kinds = logKinds
	known := false
	for _, k := range logKinds {
		known = known || k.Key == data.Kind
	}
	if !known {
		data.Kind = ""
	}
	var err error
	if data.Entries, err = factions.Log(r.Context(), d.Pool, cb.Faction, data.Kind, data.Q, 0, 200); err != nil {
		slog.Error("command: log failed", "error", err)
		http.Error(w, "Failed to load the command log.", http.StatusInternalServerError)
		return
	}
	d.Render.Render(w, "command_log.html", data)
}
