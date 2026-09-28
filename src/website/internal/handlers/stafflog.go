package handlers

import (
	"encoding/csv"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"website/internal/audit"
)

// Actions offered in the Action filter: the raw prefixes the log actually
// uses, labelled in the plan's dotted style.
var staffLogActions = []struct{ Value, Label string }{
	{"rank_change:staff_rank_id", "staff.rank"},
	{"rank_change:staff_status", "staff.status"},
	{"rank_change:staff_team", "staff.team"},
	{"rank_change:cop_level", "player.police_level"},
	{"rank_change:medic_level", "player.ems_level"},
	{"rank_edit:", "roles.edit"},
	{"rank_create:", "roles.create"},
	{"rank_delete:", "roles.delete"},
	{"rank_reorder", "roles.reorder"},
	{"faction_rank_names:", "factions.rank_names"},
}

var staffLogRanges = []struct{ Value, Label string }{
	{"7d", "Last 7 days"}, {"24h", "Last 24 hours"}, {"30d", "Last 30 days"}, {"all", "All time"},
}

type staffLogData struct {
	Base
	AdminShell
	Filter     audit.Filter
	Categories []string
	Staff      []audit.StaffOption
	Actions    []struct{ Value, Label string }
	Ranges     []struct{ Value, Label string }
	Range      string
	Page       audit.Page
	Selected   *audit.LogEntry
	Payload    string
	Query      url.Values // current filters, for building links
}

// staffLogFilter reads the filter from the query string.
func staffLogFilter(q url.Values) (audit.Filter, string) {
	f := audit.Filter{
		Category: q.Get("cat"),
		Search:   q.Get("q"),
		Target:   q.Get("target"),
		Action:   q.Get("action"),
	}
	f.StaffID, _ = strconv.ParseInt(q.Get("staff"), 10, 64)
	f.BeforeID, _ = strconv.ParseInt(q.Get("before"), 10, 64)
	f.AfterID, _ = strconv.ParseInt(q.Get("after"), 10, 64)
	rng := q.Get("range")
	switch rng {
	case "24h":
		f.Since = time.Now().Add(-24 * time.Hour)
	case "30d":
		f.Since = time.Now().AddDate(0, 0, -30)
	case "all":
	default:
		rng = "7d"
		f.Since = time.Now().AddDate(0, 0, -7)
	}
	return f, rng
}

// StaffLog is /admin/staff-log -- the layout plan's Staff Log board.
func (d *Deps) StaffLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f, rng := staffLogFilter(q)
	data := staffLogData{
		Base: baseFrom(r, "Staff Log"), AdminShell: d.adminShell(r, "stafflog"),
		Filter: f, Categories: audit.Categories, Actions: staffLogActions, Ranges: staffLogRanges, Range: rng,
		Query: q,
	}
	var err error
	if data.Page, err = audit.Query(r.Context(), d.Pool, f); err != nil {
		slog.Error("staff log query failed", "error", err)
		http.Error(w, "Failed to load the staff log.", http.StatusInternalServerError)
		return
	}
	data.Staff, _ = audit.StaffWithEntries(r.Context(), d.Pool)

	if id, _ := strconv.ParseInt(q.Get("entry"), 10, 64); id > 0 {
		if e, err := audit.Get(r.Context(), d.Pool, id); err == nil {
			data.Selected = &e
		}
	} else if len(data.Page.Entries) > 0 {
		data.Selected = &data.Page.Entries[0]
	}
	if data.Selected != nil {
		data.Payload = payloadJSON(data.Selected.Before, data.Selected.After)
	}
	d.Render.Render(w, "staff_log.html", data)
}

// payloadJSON pretty-prints before/after for the inspector.
func payloadJSON(before, after []byte) string {
	obj := map[string]json.RawMessage{}
	if len(before) > 0 {
		obj["before"] = before
	}
	if len(after) > 0 {
		obj["after"] = after
	}
	if len(obj) == 0 {
		return ""
	}
	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return ""
	}
	return string(out)
}

// StaffLogCSV exports every entry matching the current filters (up to
// 10,000 rows), ignoring paging.
func (d *Deps) StaffLogCSV(w http.ResponseWriter, r *http.Request) {
	f, _ := staffLogFilter(r.URL.Query())
	f.BeforeID, f.AfterID = 0, 0
	f.Limit = 200

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="staff-log-`+time.Now().Format("2006-01-02")+`.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "time", "staff", "staff_rank", "action", "target", "details", "reason", "source", "before", "after"})
	written := 0
	for written < 10000 {
		page, err := audit.Query(r.Context(), d.Pool, f)
		if err != nil {
			slog.Error("staff log export failed", "error", err)
			break
		}
		for _, e := range page.Entries {
			_ = cw.Write([]string{
				strconv.FormatInt(e.ID, 10), e.Time.Format(time.RFC3339), csvSafe(e.Staff), csvSafe(e.StaffRank),
				e.DisplayAction(), csvSafe(e.Target), csvSafe(audit.Describe(e.Action, e.Before, e.After)),
				csvSafe(e.Reason), e.Source, string(e.Before), string(e.After),
			})
			written++
		}
		if !page.HasOlder || len(page.Entries) == 0 {
			break
		}
		f.BeforeID = page.Entries[len(page.Entries)-1].ID
	}
	cw.Flush()
}

// csvSafe stops spreadsheet formula injection: player-chosen names or
// reasons starting with = + - @ would otherwise run as formulas in Excel.
func csvSafe(s string) string {
	if s != "" && (s[0] == '=' || s[0] == '+' || s[0] == '-' || s[0] == '@' || s[0] == '\t' || s[0] == '\r') {
		return "'" + s
	}
	return s
}
