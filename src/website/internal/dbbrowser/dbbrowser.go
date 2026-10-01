// Package dbbrowser is the read-only database browser (layout plan
// "Database browser (read-only)", GAMEPANEL_PARITY #39): tables, rows, a
// row inspector with relationships, foreign keys, and a SELECT-only SQL
// console. Management only (database.query).
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
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
	Name     string
	Type     string
	PK       bool
	Ref      string // "table.column" for a foreign key
	Hidden   bool
	Nullable bool
	Default  string
	Kind     string // num / bool / json / time / text: how the grid shows it
}

// RefTable and RefColumn split Ref.
func (c Column) RefTable() string  { t, _, _ := strings.Cut(c.Ref, "."); return t }
func (c Column) RefColumn() string { _, col, _ := strings.Cut(c.Ref, "."); return col }

func kindOf(dataType string) string {
	switch {
	case strings.Contains(dataType, "int"), dataType == "numeric", dataType == "real", dataType == "double precision":
		return "num"
	case dataType == "boolean":
		return "bool"
	case strings.HasPrefix(dataType, "json"):
		return "json"
	case strings.HasPrefix(dataType, "timestamp"), dataType == "date":
		return "time"
	}
	return "text"
}

// fkPairs is every foreign-key column pair in the public schema, from
// pg_catalog (information_schema pairs composite keys up wrongly).
const fkPairs = `
	SELECT src.relname, sa.attname, dst.relname, da.attname
	FROM pg_constraint c
	JOIN pg_class src ON src.oid = c.conrelid
	JOIN pg_class dst ON dst.oid = c.confrelid
	JOIN pg_namespace n ON n.oid = src.relnamespace AND n.nspname = 'public'
	CROSS JOIN LATERAL unnest(c.conkey, c.confkey) AS k(src_att, dst_att)
	JOIN pg_attribute sa ON sa.attrelid = c.conrelid AND sa.attnum = k.src_att
	JOIN pg_attribute da ON da.attrelid = c.confrelid AND da.attnum = k.dst_att
	WHERE c.contype = 'f'`

// Columns returns a table's columns, or an error if the table isn't known.
func (b *Browser) Columns(ctx context.Context, table string) ([]Column, error) {
	var out []Column
	err := b.readOnly(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			WITH fk(src, scol, dst, dcol) AS (`+fkPairs+`)
			SELECT c.column_name, c.data_type,
			       EXISTS (SELECT 1 FROM information_schema.table_constraints tc
			               JOIN information_schema.key_column_usage k ON k.constraint_name = tc.constraint_name AND k.table_name = tc.table_name
			               WHERE tc.table_schema = 'public' AND tc.table_name = c.table_name AND tc.constraint_type = 'PRIMARY KEY' AND k.column_name = c.column_name),
			       COALESCE((SELECT fk.dst || '.' || fk.dcol FROM fk
			                 WHERE fk.src = c.table_name AND fk.scol = c.column_name ORDER BY 1 LIMIT 1), ''),
			       c.is_nullable = 'YES', COALESCE(c.column_default, '')
			FROM information_schema.columns c
			WHERE c.table_schema = 'public' AND c.table_name = $1 ORDER BY c.ordinal_position`, table)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c Column
			if err := rows.Scan(&c.Name, &c.Type, &c.PK, &c.Ref, &c.Nullable, &c.Default); err != nil {
				return err
			}
			c.Hidden = Hidden(table, c.Name)
			c.Kind = kindOf(c.Type)
			out = append(out, c)
		}
		return rows.Err()
	})
	if err == nil && len(out) == 0 {
		return nil, fmt.Errorf("%w: no table called %q", ErrRefused, table)
	}
	return out, err
}

// Cell is one value as shown.
type Cell struct {
	Text string
	Null bool
}

// Page is a page of rows as display strings.
type Page struct {
	Columns []Column
	Rows    [][]Cell
	Keys    []string // primary key value per row ("" if none)
	Total   int64
	Sort    string // column sorted on
	Desc    bool
}

// Opts narrows and orders Rows.
type Opts struct {
	Filter        string // matches anywhere in the row
	Col, Eq       string // exact match on one column (foreign-key links)
	Sort          string // column to sort on ("" = primary key, newest first)
	Asc           bool
	Offset, Limit int
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
	case map[string]any, []any:
		b, err := json.Marshal(x)
		if err == nil {
			return string(b)
		}
	}
	return fmt.Sprint(v)
}

// PrettyJSON indents a JSON value for the inspector; other text is
// returned as is.
func PrettyJSON(s string) string {
	var buf bytes.Buffer
	if json.Indent(&buf, []byte(s), "", "  ") != nil {
		return s
	}
	return buf.String()
}

// Rows returns one page of a table. o.Filter matches anywhere in the row's
// text form; o.Col/o.Eq is an exact match on one column. Hidden columns are
// masked.
func (b *Browser) Rows(ctx context.Context, table string, o Opts) (Page, error) {
	cols, err := b.Columns(ctx, table)
	if err != nil {
		return Page{}, err
	}
	p := Page{Columns: cols}
	filter, offset, limit := o.Filter, o.Offset, o.Limit
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
	var conds []string
	args := []any{}
	if filter = strings.TrimSpace(filter); filter != "" {
		args = append(args, filter)
		conds = append(conds, fmt.Sprintf(`concat_ws(' ', %s) ILIKE '%%' || $%d || '%%'`, strings.Join(visible, ", "), len(args)))
	}
	known := func(name string) bool {
		for _, c := range cols {
			if c.Name == name && !c.Hidden {
				return true
			}
		}
		return false
	}
	if o.Col != "" && known(o.Col) {
		args = append(args, o.Eq)
		conds = append(conds, fmt.Sprintf(`%s::text = $%d`, pgx.Identifier{o.Col}.Sanitize(), len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	order := "1"
	if pk != "" {
		order = pgx.Identifier{pk}.Sanitize() + " DESC"
		p.Sort, p.Desc = pk, true
	}
	if o.Sort != "" && known(o.Sort) {
		dir := " DESC NULLS LAST"
		if o.Asc {
			dir = " ASC NULLS LAST"
		}
		order = pgx.Identifier{o.Sort}.Sanitize() + dir
		p.Sort, p.Desc = o.Sort, !o.Asc
		if pk != "" && o.Sort != pk {
			order += ", " + pgx.Identifier{pk}.Sanitize()
		}
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
			row := make([]Cell, len(vals))
			key := ""
			for i, v := range vals {
				if cols[i].Hidden {
					row[i] = Cell{Text: "••••••"}
				} else {
					row[i] = Cell{Text: display(v), Null: v == nil}
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
		// Only foreign keys onto the table's primary key: the key passed in
		// is that row's primary key value.
		rows, err := tx.Query(ctx, `
			SELECT DISTINCT fk.src, fk.scol FROM (`+fkPairs+`) fk(src, scol, dst, dcol)
			WHERE fk.dst = $1 AND fk.dcol IN (
				SELECT a.attname FROM pg_index i JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
				WHERE i.indrelid = ('public.' || quote_ident($1))::regclass AND i.indisprimary)
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
		rows, err := tx.Query(ctx, `SELECT DISTINCT * FROM (`+fkPairs+`) fk(src, scol, dst, dcol) ORDER BY 1, 2, 3`)
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
	return out, err
}

// Index is one index on a table.
type Index struct {
	Name, Def string
	Unique    bool
}

// TableInfo is the Schema tab's view of one table.
type TableInfo struct {
	Columns  []Column
	Indexes  []Index
	Incoming []FK // other tables' columns pointing here
	Size     string
	Rows     int64
}

// Describe reads one table's columns, indexes, incoming foreign keys and
// size.
func (b *Browser) Describe(ctx context.Context, table string) (TableInfo, error) {
	var ti TableInfo
	cols, err := b.Columns(ctx, table)
	if err != nil {
		return ti, err
	}
	ti.Columns = cols
	all, err := b.ForeignKeys(ctx)
	if err != nil {
		return ti, err
	}
	for _, f := range all {
		if f.RefTable == table {
			ti.Incoming = append(ti.Incoming, f)
		}
	}
	err = b.readOnly(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT indexname, indexdef, indexdef ILIKE 'CREATE UNIQUE%' FROM pg_indexes WHERE schemaname = 'public' AND tablename = $1 ORDER BY 1`, table)
		if err != nil {
			return err
		}
		for rows.Next() {
			var ix Index
			if err := rows.Scan(&ix.Name, &ix.Def, &ix.Unique); err != nil {
				rows.Close()
				return err
			}
			if _, after, ok := strings.Cut(ix.Def, " USING "); ok {
				ix.Def = after
			}
			ti.Indexes = append(ti.Indexes, ix)
		}
		rows.Close()
		tbl := pgx.Identifier{table}.Sanitize()
		if err := tx.QueryRow(ctx, `SELECT pg_size_pretty(pg_total_relation_size('public.' || quote_ident($1)))`, table).Scan(&ti.Size); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT count(*) FROM `+tbl).Scan(&ti.Rows)
	})
	return ti, err
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
	Rows    [][]Cell
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
			row := make([]Cell, len(vals))
			for i, v := range vals {
				row[i] = Cell{Text: display(v), Null: v == nil}
			}
			res.Rows = append(res.Rows, row)
		}
		return rows.Err()
	})
	res.Elapsed = time.Since(start)
	return res, err
}
