package handlers

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"website/internal/events"
)

const eventsPerPage = 50

type eventLogData struct {
	Base
	AdminShell
	Query      url.Values
	Filter     events.Filter
	Categories []events.Category
	Ranges     []events.Range
	Events     []events.Event
	More       bool
	Page       int
	Prev, Next string // page numbers for the pager ("" = none)
}

// eventFilter reads the Event Log's filters from the query string. The
// default range is the last 7 days, so the page opens quickly.
func eventFilter(q url.Values) (events.Filter, int) {
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	f := events.Filter{Search: q.Get("q"), Category: q.Get("cat"), Range: q.Get("range"), Limit: eventsPerPage, Offset: (page - 1) * eventsPerPage}
	if f.Range == "" {
		f.Range = "7d"
	}
	return f, page
}

// EventLog is /admin/events: the game's event log across every player --
// buys, sells, gives, the bank, storage, garages, fuel, robberies, police.
func (d *Deps) EventLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f, page := eventFilter(q)
	data := eventLogData{
		Base: baseFrom(r, "Event Log"), AdminShell: d.adminShell(r, "events"),
		Query: q, Filter: f, Categories: events.Categories, Ranges: events.Ranges, Page: page,
	}
	var err error
	if data.Events, data.More, err = events.List(r.Context(), d.Pool, f); err != nil {
		slog.Error("event log query failed", "error", err)
		http.Error(w, "Failed to load the event log.", http.StatusInternalServerError)
		return
	}
	if page > 1 {
		data.Prev = strconv.Itoa(page - 1)
	}
	if data.More {
		data.Next = strconv.Itoa(page + 1)
	}
	d.Render.Render(w, "event_log.html", data)
}
