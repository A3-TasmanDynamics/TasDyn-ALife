package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"website/internal/livemap"
)

// missionSQM is where the mission's markers are read from (map labels).
func missionSQM() string {
	if p := os.Getenv("MISSION_SQM"); p != "" {
		return p
	}
	return "../ALife.Altis/mission.sqm" // the website runs from src/website
}

// mapImage is an optional background: a top-down Altis map image staff can
// drop in (not shipped). Without it the map is a grid with the places.
const mapImage = "web/static/map/altis.jpg"

type liveMapData struct {
	Base
	AdminShell
	World    int
	HasImage bool
}

// LiveMap is /admin/map (game.markers): players in game on Altis.
func (d *Deps) LiveMap(w http.ResponseWriter, r *http.Request) {
	_, err := os.Stat(mapImage)
	d.Render.Render(w, "live_map.html", liveMapData{Base: baseFrom(r, "Live Map"), AdminShell: d.adminShell(r, "map"),
		World: livemap.WorldSize, HasImage: err == nil})
}

// LiveMapData is the map's JSON (polled by /static/map.js).
func (d *Deps) LiveMapData(w http.ResponseWriter, r *http.Request) {
	pts, online, err := livemap.Points(r.Context(), d.Pool)
	if err != nil {
		slog.Error("live map failed", "error", err)
		http.Error(w, "Failed to load positions.", http.StatusInternalServerError)
		return
	}
	if pts == nil {
		pts = []livemap.Point{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"at": time.Now().Format("15:04:05"), "online": online, "players": pts, "places": livemap.Places(missionSQM()),
	})
}
