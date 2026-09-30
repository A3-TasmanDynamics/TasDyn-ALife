// Package dbbrowser is the read-only database browser (layout plan
// "Database browser (read-only)", GAMEPANEL_PARITY #39): tables, rows, a
// row inspector with relationships, foreign keys, and a SELECT-only SQL
// console. Head Admin only (database.query).
//
// Read-only is enforced by Postgres, not by this UI: every query runs in a
// READ ONLY transaction with a statement timeout. When DATABASE_READONLY_URL
// points at a role that only has SELECT (database/fixes/2026-09-30_readonly_role.sql),
// the connection itself can't write either. Without it, the console also
// refuses functions a read-only transaction still allows (ending other
// connections, sleeping, advisory locks, notifications). Secret columns
// (session and link-code hashes, idempotency tokens) are never shown.
package dbbrowser

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrRefused wraps a query the console won't run, with the reason.
var ErrRefused = errors.New("refused")

// Browser reads through pool; Dedicated reports a read-only role.
type Browser struct {
	Pool      *pgxpool.Pool
	Dedicated bool
}

// hidden are columns never shown (their values are secrets or single-use
// tokens), and tables the console may not name at all.
var hidden = map[string]map[string]bool{
	"web_sessions":           {"token_hash": true},
	"discord_link_codes":     {"code": true},
	"applied_request_tokens": {"token": true},
	"bank_transactions":      {"request_token": true},
}

// Hidden reports whether table.column is masked.
func Hidden(table, column string) bool { return hidden[table][column] }

const timeout = 5 * time.Second

// readOnly runs fn in a READ ONLY transaction with a statement timeout.
func (b *Browser) readOnly(ctx context.Context, fn func(tx pgx.Tx) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout+time.Second)
	defer cancel()
	tx, err := b.Pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = %d", timeout.Milliseconds())); err != nil {
		return err
	}
	return fn(tx)
}

// Table is one table in the public schema.
type Table struct {
	Name string
	Rows int64
}

// Tables lists the public tables with exact row counts.
func (b *Browser) Tables(ctx context.Context) ([]Table, error) {
	var out []Table
	err := b.readOnly(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE' ORDER BY 1`)
		if err != nil {
			return err
		}
		var names []string
		for rows.Next() {
			var n string
			if rows.Scan(&n) == nil {
				names = append(names, n)
			}
		}
		rows.Close()
		for _, n := range names {
			t := Table{Name: n}
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM `+pgx.Identifier{n}.Sanitize()).Scan(&t.Rows); err != nil {
				return err
			}
			out = append(out, t)
		}
		return nil
	})
	return out, err
}

// Column describes one column.
type Column struct {
	Name   string
	Type   string
	PK     bool
	Ref    string // "table.column" for a foreign key
	Hidden bool
}

// Columns returns a table's columns, or an error if the table isn't known.
func (b *Browser) Columns(ctx context.Context, table string) ([]Column, error) {
	var out []Column
	err := b.readOnly(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT c.column_name, c.data_type,
			       EXISTS (SELECT 1 FROM information_schema.table_constraints tc
			               JOIN information_schema.key_column_usage k ON k.constraint_name = tc.constraint_name AND k.table_name = tc.table_name
			               WHERE tc.table_schema = 'public' AND tc.table_name = c.table_name AND tc.constraint_type = 'PRIMARY KEY' AND k.column_name = c.column_name),
			       COALESCE((SELECT ccu.table_name || '.' || ccu.column_name FROM information_schema.table_constraints tc
			                 JOIN information_schema.key_column_usage k ON k.constraint_name = tc.constraint_name AND k.table_name = tc.table_name
			                 JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = tc.constraint_name
			                 WHERE tc.table_schema = 'public' AND tc.table_name = c.table_name AND tc.constraint_type = 'FOREIGN KEY'
			                   AND k.column_name = c.column_name LIMIT 1), '')
			FROM information_schema.columns c
			WHERE c.table_schema = 'public' AND c.table_name = $1 ORDER BY c.ordinal_position`, table)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c Column
			if err := rows.Scan(&c.Name, &c.Type, &c.PK, &c.Ref); err != nil {
				return err
			}
			c.Hidden = Hidden(table, c.Name)
			out = append(out, c)
		}
		return rows.Err()
	})
	if err == nil && len(out) == 0 {
		return nil, fmt.Errorf("%w: no table called %q", ErrRefused, table)
	}
	return out, err
}

// Page is a page of rows as display strings.
type Page struct {
	Columns []Column
	Rows    [][]string
	Keys    []string // primary key value per row ("" if none)
	Total   int64
}

func display(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case time.Time:
		return x.Format("2006-01-02 15:04:05")
	case []byte:
		return string(x)
	case string:
		return x
	}
	return fmt.Sprint(v)
}

// Rows returns one page of a table. filter matches anywhere in the row's
// text form. Hidden columns are masked.
func (b *Browser) Rows(ctx context.Context, table, filter string, offset, limit int) (Page, error) {
	cols, err := b.Columns(ctx, table)
	if err != nil {
		return Page{}, err
	}
	p := Page{Columns: cols}
	pk := ""
	// Secret columns are never selected at all (a dedicated read-only role
	// isn't allowed to), and the filter searches only the visible ones.
	names := make([]string, len(cols))
	var visible []string
	for i, c := range cols {
		if c.Hidden {
			names[i] = "NULL"
		} else {
			names[i] = pgx.Identifier{c.Name}.Sanitize()
			visible = append(visible, names[i]+"::text")
		}
		if c.PK && pk == "" {
			pk = c.Name
		}
	}
	tbl := pgx.Identifier{table}.Sanitize()
	where := ""
	args := []any{}
	if filter = strings.TrimSpace(filter); filter != "" {
		where = ` WHERE concat_ws(' ', ` + strings.Join(visible, ", ") + `) ILIKE '%' || $1 || '%'`
		args = append(args, filter)
	}
	order := "1"
	if pk != "" {
		order = pgx.Identifier{pk}.Sanitize() + " DESC"
	}
	err = b.readOnly(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM `+tbl+` t`+where, args...).Scan(&p.Total); err != nil {
			return err
		}
		q := fmt.Sprintf(`SELECT %s FROM %s t%s ORDER BY %s LIMIT %d OFFSET %d`, strings.Join(names, ", "), tbl, where, order, limit, offset)
		rows, err := tx.Query(ctx, q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			vals, err := rows.Values()
			if err != nil {
				return err
			}
			row := make([]string, len(vals))
			key := ""
			for i, v := range vals {
				if cols[i].Hidden {
					row[i] = "••••••"
				} else {
					row[i] = display(v)
				}
				if cols[i].Name == pk {
					key = display(v)
				}
			}
			p.Rows = append(p.Rows, row)
			p.Keys = append(p.Keys, key)
		}
		return rows.Err()
	})
	return p, err
}

// Ref is a count of rows in another table pointing at a row.
type Ref struct {
	Table  string
	Column string
	Count  int64
}

// Referencing counts rows in other tables whose foreign keys point at the
// row with primary key value key.
func (b *Browser) Referencing(ctx context.Context, table, key string) ([]Ref, error) {
	var out []Ref
	err := b.readOnly(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT k.table_name, k.column_name FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage k ON k.constraint_name = tc.constraint_name AND k.table_name = tc.table_name
			JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = tc.constraint_name
			WHERE tc.table_schema = 'public' AND tc.constraint_type = 'FOREIGN KEY' AND ccu.table_name = $1
			ORDER BY 1, 2`, table)
		if err != nil {
			return err
		}
		var refs []Ref
		for rows.Next() {
			var r Ref
			if rows.Scan(&r.Table, &r.Column) == nil {
				refs = append(refs, r)
			}
		}
		rows.Close()
		for _, r := range refs {
			q := fmt.Sprintf(`SELECT count(*) FROM %s WHERE %s::text = $1`, pgx.Identifier{r.Table}.Sanitize(), pgx.Identifier{r.Column}.Sanitize())
			if err := tx.QueryRow(ctx, q, key).Scan(&r.Count); err != nil {
				return err
			}
			if r.Count > 0 {
				out = append(out, r)
			}
		}
		return nil
	})
	return out, err
}

// FK is one foreign key, for the Schema tab.
type FK struct {
	Table, Column, RefTable, RefColumn string
}

func (b *Browser) ForeignKeys(ctx context.Context) ([]FK, error) {
	var out []FK
	err := b.readOnly(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT k.table_name, k.column_name, ccu.table_name, ccu.column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage k ON k.constraint_name = tc.constraint_name AND k.table_name = tc.table_name
			JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = tc.constraint_name
			WHERE tc.table_schema = 'public' AND tc.constraint_type = 'FOREIGN KEY' ORDER BY 3, 1, 2`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var f FK
			if err := rows.Scan(&f.Table, &f.Column, &f.RefTable, &f.RefColumn); err != nil {
				return err
			}
			out = append(out, f)
		}
		return rows.Err()
	})
	sort.SliceStable(out, func(i, j int) bool { return out[i].RefTable < out[j].RefTable })
	return out, err
}

// ---- SQL console ----

var (
	leadingComments = regexp.MustCompile(`^(\s*(--[^\n]*\n|/\*.*?\*/))*\s*`)
	startsReadable  = regexp.MustCompile(`(?i)^(select|with|table|values|explain)\b`)
	// Functions a READ ONLY transaction still allows but that affect other
	// sessions or the server, plus explicit writes in CTEs (which read-only
	// mode would refuse anyway -- this gives a clearer message).
	blocked = regexp.MustCompile(`(?i)\b(pg_terminate_backend|pg_cancel_backend|pg_sleep\w*|pg_advisory\w*|pg_try_advisory\w*|` +
		`set_config|pg_reload_conf|pg_rotate_logfile|pg_notify|pg_read_\w+|pg_ls_\w+|pg_stat_file|lo_\w+|dblink\w*|` +
		`pg_logical_\w+|pg_create_\w+|pg_drop_\w+|pg_switch_\w+|pg_promote|pg_backup_\w+|nextval|setval|` +
		`insert|update|delete|merge|truncate|copy|grant|revoke|alter|drop|create|vacuum|analyze|lock|listen|notify|do|call|refresh|reindex|cluster|security)\b`)
)

// Result is a console result.
type Result struct {
	Columns []string
	Rows    [][]string
	Elapsed time.Duration
	Capped  bool // more rows than MaxRows
}

// MaxRows is the console's row cap.
const MaxRows = 500

// Check says why sql would be refused, or "" if it may run.
func Check(sql string, dedicated bool) string {
	s := strings.TrimSpace(sql)
	s = strings.TrimSpace(leadingComments.ReplaceAllString(s, ""))
	s = strings.TrimSuffix(s, ";")
	switch {
	case s == "":
		return "write a SELECT query"
	case strings.Contains(s, ";"):
		return "one statement at a time"
	case len(s) > 8000:
		return "the query is too long"
	case !startsReadable.MatchString(s):
		return "only SELECT queries can run here"
	}
	if strings.Contains(strings.ToLower(s), "analyze") && strings.HasPrefix(strings.ToLower(s), "explain") {
		return "EXPLAIN ANALYZE runs the query; use plain EXPLAIN"
	}
	if !dedicated {
		if m := blocked.FindString(s); m != "" && !strings.EqualFold(m, "analyze") {
			return fmt.Sprintf("%q isn't allowed in the console", strings.ToLower(m))
		}
		for t := range hidden {
			if regexp.MustCompile(`(?i)\b` + t + `\b`).MatchString(s) {
				return fmt.Sprintf("the %s table holds secrets and can't be queried from the console", t)
			}
		}
	}
	return ""
}

// Query runs a SELECT through the read-only transaction.
func (b *Browser) Query(ctx context.Context, sql string) (Result, error) {
	if why := Check(sql, b.Dedicated); why != "" {
		return Result{}, fmt.Errorf("%w: %s", ErrRefused, why)
	}
	var res Result
	start := time.Now()
	err := b.readOnly(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, strings.TrimSuffix(strings.TrimSpace(sql), ";"))
		if err != nil {
			return err
		}
		defer rows.Close()
		for _, f := range rows.FieldDescriptions() {
			res.Columns = append(res.Columns, f.Name)
		}
		for rows.Next() {
			if len(res.Rows) == MaxRows {
				res.Capped = true
				break
			}
			vals, err := rows.Values()
			if err != nil {
				return err
			}
			row := make([]string, len(vals))
			for i, v := range vals {
				row[i] = display(v)
			}
			res.Rows = append(res.Rows, row)
		}
		return rows.Err()
	})
	res.Elapsed = time.Since(start)
	return res, err
}
