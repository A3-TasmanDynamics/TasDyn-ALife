// Package replies is the staff team's saved replies & templates: canned
// ticket replies, case-entry templates and player-note templates that staff
// insert with one click (the "Insert a saved reply" menus) and manage on
// /admin/replies (replies.manage). Placeholders -- {player}, {staff},
// {ticket}, {case} -- are filled in by the page when inserted.
package replies

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Scopes: where a reply can be inserted.
var Scopes = []Scope{{"ticket", "Ticket replies"}, {"case", "Case entries"}, {"note", "Player notes"}}

type Scope struct{ Key, Label string }

// Placeholders, for the page's help text.
var Placeholders = []string{"{player}", "{staff}", "{ticket}", "{case}"}

// ErrNotAllowed wraps a reason it can't be saved.
var ErrNotAllowed = errors.New("not allowed")

func notAllowed(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrNotAllowed, fmt.Sprintf(format, a...))
}

type Reply struct {
	ID        int64
	Scope     string
	Title     string
	Body      string
	UpdatedBy string
	UpdatedAt time.Time
}

// List is every saved reply in scope ("" = all), by scope then title.
func List(ctx context.Context, pool *pgxpool.Pool, scope string) ([]Reply, error) {
	rows, err := pool.Query(ctx, `
		SELECT r.id, r.scope, r.title, r.body, COALESCE(NULLIF(u.name, ''), NULLIF(u.steam_name, ''), ''), r.updated_at
		FROM saved_replies r LEFT JOIN players u ON u.id = r.updated_by
		WHERE $1 = '' OR r.scope = $1
		ORDER BY array_position(ARRAY['ticket', 'case', 'note'], r.scope), lower(r.title)`, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Reply
	for rows.Next() {
		var r Reply
		if err := rows.Scan(&r.ID, &r.Scope, &r.Title, &r.Body, &r.UpdatedBy, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Save adds a reply (id 0) or changes one.
func Save(ctx context.Context, pool *pgxpool.Pool, staffID, id int64, scope, title, body string) (int64, error) {
	ok := false
	for _, s := range Scopes {
		ok = ok || s.Key == scope
	}
	if !ok {
		return 0, notAllowed("choose where it's used")
	}
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
	if title == "" || len([]rune(title)) > 80 {
		return 0, notAllowed("give it a title (up to 80 characters)")
	}
	if body == "" || len([]rune(body)) > 4000 {
		return 0, notAllowed("write the text (up to 4000 characters)")
	}
	var err error
	if id == 0 {
		err = pool.QueryRow(ctx, `INSERT INTO saved_replies (scope, title, body, created_by, updated_by) VALUES ($1, $2, $3, $4, $4) RETURNING id`,
			scope, title, body, staffID).Scan(&id)
	} else {
		var tag pgconn.CommandTag
		tag, err = pool.Exec(ctx, `UPDATE saved_replies SET scope = $2, title = $3, body = $4, updated_by = $5, updated_at = now() WHERE id = $1`,
			id, scope, title, body, staffID)
		if err == nil && tag.RowsAffected() == 0 {
			return 0, notAllowed("that saved reply doesn't exist any more")
		}
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return 0, notAllowed("there's already one called %q there", title)
	}
	return id, err
}

// Delete removes one.
func Delete(ctx context.Context, pool *pgxpool.Pool, id int64) error {
	var gone int64
	err := pool.QueryRow(ctx, `DELETE FROM saved_replies WHERE id = $1 RETURNING id`, id).Scan(&gone)
	if errors.Is(err, pgx.ErrNoRows) {
		return notAllowed("that saved reply doesn't exist any more")
	}
	return err
}
