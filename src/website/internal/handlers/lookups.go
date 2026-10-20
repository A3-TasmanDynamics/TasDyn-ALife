package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"website/internal/lookup"
	"website/internal/players"
)

// ---- Vehicle Lookup (/admin/vehicles, players.vehicles) ----

type vehicleLookupData struct {
	Base
	AdminShell
	Q        string
	Status   string
	Statuses []string
	Query    url.Values
	Results  []lookup.VehicleRow
	V        *lookup.Vehicle
}

func (vehicleLookupData) StatusText(s string) string { return lookup.StatusLabel(s) }

// VehicleLookup: search on the left (plate, vehicle, owner, id; by
// status), the selected vehicle on the right -- owner, state, trunk and
// history. Read-only.
func (d *Deps) VehicleLookup(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	data := vehicleLookupData{
		Base: baseFrom(r, "Vehicle Lookup"), AdminShell: d.adminShell(r, "vehicles"),
		Q: strings.TrimSpace(q.Get("q")), Status: q.Get("status"), Statuses: lookup.Statuses, Query: q,
	}
	if !slices.Contains(lookup.Statuses, data.Status) {
		data.Status = ""
	}
	var err error
	if data.Results, err = lookup.SearchVehicles(r.Context(), d.Pool, data.Q, data.Status); err != nil {
		slog.Error("vehicle search failed", "error", err)
		http.Error(w, "Search failed.", http.StatusInternalServerError)
		return
	}
	id, _ := strconv.ParseInt(q.Get("id"), 10, 64)
	if id == 0 && len(data.Results) > 0 && data.Q != "" {
		id = data.Results[0].ID
	}
	if id > 0 {
		v, err := lookup.GetVehicle(r.Context(), d.Pool, id)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			slog.Error("vehicle lookup failed", "error", err)
		default:
			data.V = &v
		}
	}
	d.Render.Render(w, "vehicle_lookup.html", data)
}

// ---- Houses & Garages (/admin/houses, players.view; Garages needs players.vehicles) ----

type housesData struct {
	Base
	AdminShell
	Tab         string
	Q           string
	Owned       string
	Query       url.Values
	Results     []lookup.HouseRow
	H           *lookup.House
	CanVehicles bool
	Garages     lookup.Garages
}

func (housesData) Money(c int64) string { return players.Dollars(c) }

func (d *Deps) HousesGarages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	data := housesData{
		Base: baseFrom(r, "Houses & Garages"), AdminShell: d.adminShell(r, "houses"),
		Tab: "houses", Q: strings.TrimSpace(q.Get("q")), Owned: q.Get("owned"), Query: q,
		CanVehicles: d.can(r, "players.vehicles"),
	}
	if data.Owned != "owned" && data.Owned != "forsale" {
		data.Owned = ""
	}
	if q.Get("tab") == "garages" && data.CanVehicles {
		data.Tab = "garages"
		var err error
		if data.Garages, err = lookup.GarageOverview(r.Context(), d.Pool); err != nil {
			slog.Error("garage overview failed", "error", err)
			data.Error = "Couldn't load the garages."
		}
		d.Render.Render(w, "houses.html", data)
		return
	}

	var err error
	if data.Results, err = lookup.SearchHouses(r.Context(), d.Pool, data.Q, data.Owned); err != nil {
		slog.Error("house search failed", "error", err)
		http.Error(w, "Search failed.", http.StatusInternalServerError)
		return
	}
	id, _ := strconv.ParseInt(q.Get("id"), 10, 64)
	if id == 0 && len(data.Results) > 0 && data.Q != "" {
		id = data.Results[0].ID
	}
	if id > 0 {
		h, err := lookup.GetHouse(r.Context(), d.Pool, id)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			slog.Error("house lookup failed", "error", err)
		default:
			data.H = &h
		}
	}
	d.Render.Render(w, "houses.html", data)
}
