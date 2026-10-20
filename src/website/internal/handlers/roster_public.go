package handlers

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"website/internal/factions"
)

// publicRosters are the factions whose roster anyone can see at
// /roster/{faction} (signed in or not). Add "ems" to open the EMS one too.
var publicRosters = map[string]bool{"police": true}

type publicRosterRow struct {
	Rank       string
	Cabinet    bool
	Command    bool
	Badge      string
	Name       string
	Department string
	Status     string
	Online     bool
	Break      bool // first of a new rank (a divider above it)
}

type publicRosterData struct {
	Base
	Faction     string
	FactionName string
	Callsign    string
	Q           string
	Total       int
	Online      int
	Rows        []publicRosterRow
}

// PublicRoster is the faction's roster for everyone: rank, callsign, name,
// division and status (LOA / reserve / probation ...). Nothing from the
// personnel files -- notes, discipline, points, roll call, region or
// certifications stay in the Command panel, and Suspended shows as the
// status command set (discipline isn't public).
func (d *Deps) PublicRoster(w http.ResponseWriter, r *http.Request) {
	faction := chi.URLParam(r, "faction")
	if !publicRosters[faction] {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	name := factions.Name(faction)
	data := publicRosterData{Base: baseFrom(r, name+" Roster"), Faction: faction, FactionName: name,
		Callsign: map[bool]string{true: "Callsign", false: "Badge / callsign"}[faction == "ems"],
		Q:        strings.TrimSpace(r.URL.Query().Get("q"))}
	rows, err := factions.PersonnelRoster(ctx, d.Pool, faction, time.Now())
	if err != nil {
		slog.Error("public roster failed", "faction", faction, "error", err)
		http.Error(w, "Failed to load the roster.", http.StatusInternalServerError)
		return
	}
	ranks, _ := factions.Ranks(ctx, d.Pool, faction)
	byLevel := map[int]factions.Rank{}
	for _, rk := range ranks {
		byLevel[rk.Level] = rk
	}
	data.Total = len(rows)
	needle := strings.ToLower(data.Q)
	prev := -1
	for _, p := range rows {
		if p.Online {
			data.Online++
		}
		if needle != "" && !strings.Contains(strings.ToLower(p.Name+" "+p.Badge+" "+p.Department+" "+p.Rank.Label()), needle) {
			continue
		}
		status := p.Shown
		if status == "Suspended" {
			status = factions.StatusLabel(p.Status)
		}
		rk := byLevel[p.Level]
		data.Rows = append(data.Rows, publicRosterRow{
			Rank: p.Rank.Label(), Cabinet: rk.IsCabinet, Command: rk.IsCommand && !rk.IsCabinet,
			Badge: p.Badge, Name: p.Name, Department: p.Department, Status: status, Online: p.Online,
			Break: prev != -1 && prev != p.Level,
		})
		prev = p.Level
	}
	d.Render.Render(w, "roster_public.html", data)
}
