package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"

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

type rosterGroup struct {
	Rank    factions.Rank
	Members []factions.Member
	Filled  int
}

type commandRosterData struct {
	commandBase
	Q          string
	Groups     []rosterGroup
	Total      int
	CanRecruit bool
	Levels     []factions.Rank // ranks the viewer can recruit into
}

func (d *Deps) CommandRoster(w http.ResponseWriter, r *http.Request) {
	cb, ok := d.commandAccess(w, r, "roster")
	if !ok {
		return
	}
	data := commandRosterData{commandBase: cb, Q: strings.TrimSpace(r.URL.Query().Get("q"))}
	ctx := r.Context()
	ranks, _ := factions.Ranks(ctx, d.Pool, cb.Faction)
	members, err := factions.Roster(ctx, d.Pool, cb.Faction)
	if err != nil {
		slog.Error("command: roster failed", "error", err)
		http.Error(w, "Failed to load the roster.", http.StatusInternalServerError)
		return
	}
	data.Total = len(members)
	q := strings.ToLower(data.Q)
	for _, m := range members {
		if q != "" && !strings.Contains(strings.ToLower(m.Name), q) {
			continue
		}
		if n := len(data.Groups); n == 0 || data.Groups[n-1].Rank.Level != m.Level {
			data.Groups = append(data.Groups, rosterGroup{Rank: m.Rank})
		}
		g := &data.Groups[len(data.Groups)-1]
		g.Members = append(g.Members, m)
	}
	for i := range data.Groups {
		for _, m := range members {
			if m.Level == data.Groups[i].Rank.Level {
				data.Groups[i].Filled++
			}
		}
	}
	if !cb.ReadOnly {
		data.CanRecruit = true
		data.Levels = assignable(ranks, cb.Command.Authority)
	}
	d.Render.Render(w, "command_roster.html", data)
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
	if data.History, err = factions.Log(ctx, d.Pool, cb.Faction, "", "", id, 50); err != nil {
		slog.Error("command: member history failed", "error", err)
	}
	sess, _ := auth.FromContext(ctx)
	switch {
	case cb.ReadOnly:
		data.Why = "You're viewing as staff (read-only). Management can override ranks from Player Lookup."
	case id == sess.PlayerID:
		data.Why = "You can't change your own rank."
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
}

func (d *Deps) CommandLog(w http.ResponseWriter, r *http.Request) {
	cb, ok := d.commandAccess(w, r, "log")
	if !ok {
		return
	}
	data := commandLogData{commandBase: cb, Kind: r.URL.Query().Get("kind"), Q: strings.TrimSpace(r.URL.Query().Get("q"))}
	if kindWords[data.Kind] == "" {
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
