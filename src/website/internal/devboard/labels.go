package devboard

import (
	"context"
	"errors"
	"hash/fnv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Colour is one swatch in the label palette.
type Colour struct{ Key, Name, Hex string }

// Palette is the colours a label can be, Trello-style.
var Palette = []Colour{
	{"green", "Green", "#4bce97"}, {"yellow", "Yellow", "#f5cd47"}, {"orange", "Orange", "#fea362"},
	{"red", "Red", "#f87168"}, {"purple", "Purple", "#9f8fef"}, {"blue", "Blue", "#579dff"},
	{"sky", "Sky", "#6cc3e0"}, {"lime", "Lime", "#94c748"}, {"pink", "Pink", "#e774bb"},
	{"grey", "Grey", "#8590a2"},
}

// Label is a named, coloured tag that cards can carry.
type Label struct {
	Name  string
	Color string
	Cards int // cards using it
}

func validColour(key string) bool {
	for _, c := range Palette {
		if c.Key == key {
			return true
		}
	}
	return false
}

// defaultColour picks a stable palette colour for a label nobody has
// coloured yet.
func defaultColour(name string) string {
	h := fnv.New32a()
	h.Write([]byte(strings.ToLower(name)))
	return Palette[h.Sum32()%uint32(len(Palette))].Key
}

// labelName tidies a label name: single spaces, at most 32 characters.
func labelName(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// ListLabels is every label, in board order, with how many cards use it.
// Names used on cards but never saved as labels are included too.
func ListLabels(ctx context.Context, pool *pgxpool.Pool) ([]Label, error) {
	rows, err := pool.Query(ctx, `
		WITH used AS (SELECT unnest(labels) AS name FROM dev_tasks)
		SELECT l.name, l.color, (SELECT count(*) FROM used u WHERE u.name = l.name), l.sort
		  FROM dev_labels l
		UNION ALL
		SELECT u.name, '', count(*), 1e9 FROM used u
		 WHERE NOT EXISTS (SELECT 1 FROM dev_labels l WHERE l.name = u.name)
		 GROUP BY u.name
		ORDER BY 4, 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Label
	for rows.Next() {
		var l Label
		var sort float64
		if err := rows.Scan(&l.Name, &l.Color, &l.Cards, &sort); err != nil {
			return nil, err
		}
		if l.Color == "" {
			l.Color = defaultColour(l.Name)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// SaveLabel creates a label (oldName "") or renames / recolours one,
// renaming it on every card that uses it.
func SaveLabel(ctx context.Context, pool *pgxpool.Pool, oldName, name, colour string) error {
	name = labelName(name)
	if name == "" || len([]rune(name)) > 32 {
		return UserError("Label names need 1 to 32 characters.")
	}
	if !validColour(colour) {
		return UserError("Pick a colour from the palette.")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var clash bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM dev_labels WHERE lower(name) = lower($1) AND name <> $2)`,
		name, oldName).Scan(&clash); err != nil {
		return err
	}
	if clash {
		return UserError("There is already a label called " + name + ".")
	}
	if oldName == "" {
		_, err = tx.Exec(ctx, `INSERT INTO dev_labels (name, color, sort) VALUES ($1, $2, COALESCE((SELECT max(sort) FROM dev_labels), 0) + 1)`, name, colour)
	} else {
		// A label only seen on cards gets its row now.
		_, err = tx.Exec(ctx, `
			INSERT INTO dev_labels (name, color, sort) VALUES ($1, $2, COALESCE((SELECT max(sort) FROM dev_labels), 0) + 1)
			ON CONFLICT (name) DO NOTHING`, oldName, colour)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE dev_labels SET name = $2, color = $3 WHERE name = $1`, oldName, name, colour)
		}
		if err == nil && oldName != name {
			_, err = tx.Exec(ctx, `UPDATE dev_tasks SET labels = array_replace(labels, $1, $2) WHERE $1 = ANY(labels)`, oldName, name)
		}
	}
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			return UserError("There is already a label called " + name + ".")
		}
		return err
	}
	return tx.Commit(ctx)
}

// DeleteLabel removes a label and takes it off every card.
func DeleteLabel(ctx context.Context, pool *pgxpool.Pool, name string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM dev_labels WHERE name = $1`, name); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE dev_tasks SET labels = array_remove(labels, $1) WHERE $1 = ANY(labels)`, name); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// matchLabels swaps each name for the saved label it matches (ignoring
// case) and saves any new names as labels, so every card label has a
// colour.
func matchLabels(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, names []string) ([]string, error) {
	if len(names) == 0 {
		return names, nil
	}
	rows, err := q.Query(ctx, `SELECT name FROM dev_labels`)
	if err != nil {
		return nil, err
	}
	saved := map[string]string{}
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil {
			saved[strings.ToLower(n)] = n
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		key := strings.ToLower(n)
		if seen[key] {
			continue
		}
		seen[key] = true
		if s, ok := saved[key]; ok {
			out = append(out, s)
			continue
		}
		if _, err := q.Exec(ctx, `
			INSERT INTO dev_labels (name, color, sort) VALUES ($1, $2, COALESCE((SELECT max(sort) FROM dev_labels), 0) + 1)
			ON CONFLICT DO NOTHING`, n, defaultColour(n)); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}
