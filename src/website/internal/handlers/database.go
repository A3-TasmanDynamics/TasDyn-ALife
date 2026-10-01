package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"website/internal/audit"
	"website/internal/auth"
	"website/internal/dbbrowser"
)

// Database is Admin → Database (layout plan "Database browser (read-only)"),
// Head Admin only (database.query).

type databaseData struct {
	Base
	AdminShell
	Tab        string // data / schema / sql
	Tables     []dbbrowser.Table
	TableCount int
	Groups     []tableGroup
	TableQ     string
	Table      string
	Filter     string
	Col, Eq    string // exact column match (foreign-key links)
	Sort       string
	Asc        bool
	Page       dbbrowser.Page
	PageN      int
	PageSize   int
	Pages      int
	From, To   int64
	Row        []dbbrowser.Column
	RowCells   []dbbrowser.Cell
	RowKey     string
	Refs       []dbbrowser.Ref
	FKs        []dbbrowser.FK
	Info       *dbbrowser.TableInfo
	SQL        string
	Result     *dbbrowser.Result
	QueryErr   string
	Dedicated  bool
}

// tableGroup is a run of tables sharing a name prefix (faction_, gang_...).
type tableGroup struct {
	Name   string
	Tables []dbbrowser.Table
	Rows   int64
	Open   bool
}

// URL is a link to the database page keeping the current table, filters,
// sort and page size, with kv overrides ("page", "2"). An empty value
// removes a parameter.
func (d databaseData) URL(kv ...string) string {
	v := url.Values{}
	set := func(k, val string) {
		if val != "" {
			v.Set(k, val)
		}
	}
	set("table", d.Table)
	if d.Tab != "data" {
		set("tab", d.Tab)
	}
	set("q", d.Filter)
	if d.Col != "" {
		set("col", d.Col)
		set("eq", d.Eq)
	}
	set("sort", d.Sort)
	if d.Sort != "" && d.Asc {
		set("dir", "asc")
	}
	if d.PageSize != dbPageSize {
		set("size", strconv.Itoa(d.PageSize))
	}
	if d.PageN > 1 {
		set("page", strconv.Itoa(d.PageN))
	}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] == "" {
			v.Del(kv[i])
		} else {
			v.Set(kv[i], kv[i+1])
		}
	}
	return "/admin/database?" + v.Encode()
}

// SortURL toggles sorting on a column (descending first).
func (d databaseData) SortURL(col string) string {
	dir := ""
	if d.Sort == col && !d.Asc {
		dir = "asc"
	}
	return d.URL("sort", col, "dir", dir, "page", "", "row", "")
}

const dbPageSize = 50

var dbPageSizes = map[int]bool{25: true, 50: true, 100: true, 200: true}

func (d *Deps) Database(w http.ResponseWriter, r *http.Request) {
	d.renderDatabase(w, r, "", nil, "")
}

func (d *Deps) renderDatabase(w http.ResponseWriter, r *http.Request, sql string, res *dbbrowser.Result, qerr string) {
	ctx := r.Context()
	q := r.URL.Query()
	data := databaseData{Base: baseFrom(r, "Database"), AdminShell: d.adminShell(r, "database"), Tab: q.Get("tab"),
		TableQ: strings.TrimSpace(q.Get("tq")), Table: q.Get("table"), Filter: q.Get("q"), PageSize: dbPageSize,
		Col: q.Get("col"), Eq: q.Get("eq"), Sort: q.Get("sort"), Asc: q.Get("dir") == "asc",
		SQL: sql, Result: res, QueryErr: qerr, Dedicated: d.DB.Dedicated}
	if n, _ := strconv.Atoi(q.Get("size")); dbPageSizes[n] {
		data.PageSize = n
	}
	if sql != "" || res != nil || qerr != "" {
		data.Tab = "sql"
	}
	if data.Tab != "schema" && data.Tab != "sql" {
		data.Tab = "data"
	}
	tables, err := d.DB.Tables(ctx)
	if err != nil {
		slog.Error("database: tables failed", "error", err)
		http.Error(w, "Couldn't read the database.", http.StatusInternalServerError)
		return
	}
	data.TableCount = len(tables)
	for _, t := range tables {
		if data.TableQ == "" || strings.Contains(t.Name, strings.ToLower(data.TableQ)) {
			data.Tables = append(data.Tables, t)
		}
	}
	if data.Table == "" {
		data.Table = "players"
	}
	data.Groups = groupTables(data.Tables, data.Table, data.TableQ != "")

	switch data.Tab {
	case "data":
		data.PageN, _ = strconv.Atoi(q.Get("page"))
		if data.PageN < 1 {
			data.PageN = 1
		}
		data.Page, err = d.DB.Rows(ctx, data.Table, dbbrowser.Opts{Filter: data.Filter, Col: data.Col, Eq: data.Eq,
			Sort: data.Sort, Asc: data.Asc, Offset: (data.PageN - 1) * data.PageSize, Limit: data.PageSize})
		if errors.Is(err, dbbrowser.ErrRefused) {
			http.Redirect(w, r, "/admin/database?error="+errMsg("That table doesn't exist."), http.StatusSeeOther)
			return
		}
		if err != nil {
			slog.Error("database: rows failed", "error", err)
			data.Error = "Couldn't read that table: " + err.Error()
		}
		data.Pages = int((data.Page.Total + int64(data.PageSize) - 1) / int64(data.PageSize))
		if data.Page.Total > 0 {
			data.From = int64((data.PageN-1)*data.PageSize) + 1
			data.To = data.From + int64(len(data.Page.Rows)) - 1
		}
		// Inspector: the selected row (or the first).
		sel := q.Get("row")
		for i, k := range data.Page.Keys {
			if (sel == "" && i == 0) || (sel != "" && k == sel) {
				data.Row, data.RowKey = data.Page.Columns, k
				data.RowCells = make([]dbbrowser.Cell, len(data.Page.Rows[i]))
				for j, c := range data.Page.Rows[i] {
					if data.Page.Columns[j].Kind == "json" && !c.Null {
						c.Text = dbbrowser.PrettyJSON(c.Text)
					}
					data.RowCells[j] = c
				}
			}
		}
		if data.RowKey != "" {
			data.Refs, _ = d.DB.Referencing(ctx, data.Table, data.RowKey)
		}
	case "schema":
		if data.FKs, err = d.DB.ForeignKeys(ctx); err != nil {
			slog.Error("database: foreign keys failed", "error", err)
		}
		if info, err := d.DB.Describe(ctx, data.Table); err == nil {
			data.Info = &info
		} else if !errors.Is(err, dbbrowser.ErrRefused) {
			slog.Error("database: describe failed", "error", err)
		}
	}
	d.Render.Render(w, "database.html", data)
}

// groupTables groups tables by the first word of their name (faction_,
// gang_, staff_...); words used by a single table go under "Other". The
// group holding the open table starts expanded (all of them when
// searching).
func groupTables(tables []dbbrowser.Table, current string, all bool) []tableGroup {
	stem := func(name string) string {
		w, _, _ := strings.Cut(name, "_")
		return strings.TrimSuffix(w, "s")
	}
	count := map[string]int{}
	for _, t := range tables {
		count[stem(t.Name)]++
	}
	byName := map[string]*tableGroup{}
	var order []string
	for _, t := range tables {
		g := stem(t.Name)
		if count[g] < 2 {
			g = "other"
		}
		if byName[g] == nil {
			label := g
			if g == "other" {
				label = "Other"
			}
			byName[g] = &tableGroup{Name: label}
			order = append(order, g)
		}
		grp := byName[g]
		grp.Tables = append(grp.Tables, t)
		grp.Rows += t.Rows
		if all || t.Name == current {
			grp.Open = true
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		if (order[i] == "other") != (order[j] == "other") {
			return order[j] == "other"
		}
		return order[i] < order[j]
	})
	out := make([]tableGroup, 0, len(order))
	for _, g := range order {
		out = append(out, *byName[g])
	}
	return out
}

// DatabaseQuery runs a console query and logs it to the Staff Log.
func (d *Deps) DatabaseQuery(w http.ResponseWriter, r *http.Request) {
	sql := strings.TrimSpace(r.FormValue("sql"))
	res, err := d.DB.Query(r.Context(), sql)
	qerr := ""
	if err != nil {
		qerr = strings.TrimPrefix(err.Error(), dbbrowser.ErrRefused.Error()+": ")
		res = dbbrowser.Result{}
	}
	// Every query run here is written to the Staff Log (layout plan), refused
	// ones too, so attempts are visible.
	sess, _ := auth.FromContext(r.Context())
	if tx, terr := d.Pool.Begin(r.Context()); terr == nil {
		outcome := map[string]any{"rows": len(res.Rows), "ms": res.Elapsed.Milliseconds()}
		if qerr != "" {
			outcome = map[string]any{"refused": qerr}
		}
		logSQL := sql
		if len(logSQL) > 1000 {
			logSQL = logSQL[:1000] + "…"
		}
		if lerr := audit.LogStaffAction(r.Context(), tx, audit.Entry{StaffID: sess.PlayerID, Action: "database.query",
			Reason: logSQL, After: outcome}); lerr == nil {
			_ = tx.Commit(r.Context())
		} else {
			tx.Rollback(r.Context())
			slog.Error("database: logging query failed", "error", lerr)
		}
	}
	var resPtr *dbbrowser.Result
	if qerr == "" {
		resPtr = &res
	}
	d.renderDatabase(w, r, sql, resPtr, qerr)
}
