package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"website/internal/boards"
)

// ---- /admin/gangs ----

type gangsData struct {
	Base
	AdminShell
	Q        string
	Gangs    []boards.Gang
	Selected *boards.GangDetail
}

func (d *Deps) GangsBoard(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	data := gangsData{Base: baseFrom(r, "Gangs"), AdminShell: d.adminShell(r, "gangs"), Q: strings.TrimSpace(q.Get("q"))}
	var err error
	if data.Gangs, err = boards.Gangs(r.Context(), d.Pool, data.Q); err != nil {
		slog.Error("gangs board failed", "error", err)
		http.Error(w, "Failed to load gangs.", http.StatusInternalServerError)
		return
	}
	id, _ := strconv.ParseInt(q.Get("id"), 10, 64)
	if id == 0 && len(data.Gangs) > 0 {
		id = data.Gangs[0].ID
	}
	if id > 0 {
		g, ok, err := boards.GetGang(r.Context(), d.Pool, id)
		if err != nil {
			slog.Error("gang detail failed", "error", err)
		}
		if ok {
			data.Selected = &g
		}
	}
	d.Render.Render(w, "gangs.html", data)
}

// ---- /admin/wanted ----

type wantedData struct {
	Base
	AdminShell
	Q     string
	Board boards.Wanted
}

func (d wantedData) Dur(t time.Duration) string { return hoursMinutes(t) }

func (d *Deps) WantedBoard(w http.ResponseWriter, r *http.Request) {
	data := wantedData{Base: baseFrom(r, "Wanted & Jail"), AdminShell: d.adminShell(r, "wanted"), Q: strings.TrimSpace(r.URL.Query().Get("q"))}
	var err error
	if data.Board, err = boards.Board(r.Context(), d.Pool, data.Q); err != nil {
		slog.Error("wanted board failed", "error", err)
		http.Error(w, "Failed to load the wanted board.", http.StatusInternalServerError)
		return
	}
	d.Render.Render(w, "wanted.html", data)
}
