package handlers

import (
	"errors"
	"log/slog"
	"net/http"
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
	Tab       string // data / schema / sql
	Tables    []dbbrowser.Table
	TableQ    string
	Table     string
	Filter    string
	Page      dbbrowser.Page
	PageN     int
	PageSize  int
	Pages     int
	Row       []dbbrowser.Column
	RowVals   []string
	RowKey    string
	Refs      []dbbrowser.Ref
	FKs       []dbbrowser.FK
	SQL       string
	Result    *dbbrowser.Result
	QueryErr  string
	Dedicated bool
}

const dbPageSize = 50

func (d *Deps) Database(w http.ResponseWriter, r *http.Request) {
	d.renderDatabase(w, r, "", nil, "")
}

func (d *Deps) renderDatabase(w http.ResponseWriter, r *http.Request, sql string, res *dbbrowser.Result, qerr string) {
	ctx := r.Context()
	q := r.URL.Query()
	data := databaseData{Base: baseFrom(r, "Database"), AdminShell: d.adminShell(r, "database"), Tab: q.Get("tab"),
		TableQ: strings.TrimSpace(q.Get("tq")), Table: q.Get("table"), Filter: q.Get("q"), PageSize: dbPageSize,
		SQL: sql, Result: res, QueryErr: qerr, Dedicated: d.DB.Dedicated}
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
	for _, t := range tables {
		if data.TableQ == "" || strings.Contains(t.Name, strings.ToLower(data.TableQ)) {
			data.Tables = append(data.Tables, t)
		}
	}
	if data.Table == "" {
		data.Table = "players"
	}
	switch data.Tab {
	case "data":
		data.PageN, _ = strconv.Atoi(q.Get("page"))
		if data.PageN < 1 {
			data.PageN = 1
		}
		data.Page, err = d.DB.Rows(ctx, data.Table, data.Filter, (data.PageN-1)*dbPageSize, dbPageSize)
		if errors.Is(err, dbbrowser.ErrRefused) {
			http.Redirect(w, r, "/admin/database?error="+errMsg("That table doesn't exist."), http.StatusSeeOther)
			return
		}
		if err != nil {
			slog.Error("database: rows failed", "error", err)
			data.Error = "Couldn't read that table: " + err.Error()
		}
		data.Pages = int((data.Page.Total + dbPageSize - 1) / dbPageSize)
		// Inspector: the selected row (or the first).
		sel := q.Get("row")
		for i, k := range data.Page.Keys {
			if (sel == "" && i == 0) || (sel != "" && k == sel) {
				data.Row, data.RowVals, data.RowKey = data.Page.Columns, data.Page.Rows[i], k
			}
		}
		if data.RowKey != "" {
			data.Refs, _ = d.DB.Referencing(ctx, data.Table, data.RowKey)
		}
	case "schema":
		if data.FKs, err = d.DB.ForeignKeys(ctx); err != nil {
			slog.Error("database: foreign keys failed", "error", err)
		}
	}
	d.Render.Render(w, "database.html", data)
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
