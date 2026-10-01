// Package devboard is the development project board (Admin → Development
// → Project board): tasks in To do / In progress / Review / Done columns,
// ordered by hand within each column.
package devboard

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserError is a message safe to show as-is.
type UserError string

func (e UserError) Error() string { return string(e) }

// Status is one board column.
type Status struct{ Key, Label string }

// Statuses in board order.
var Statuses = []Status{{"todo", "To do"}, {"doing", "In progress"}, {"review", "Review"}, {"done", "Done"}}

// Priorities, most urgent first.
var Priorities = []string{"urgent", "high", "normal", "low"}

func validStatus(s string) bool {
	for _, x := range Statuses {
		if x.Key == s {
			return true
		}
	}
	return false
}

func validPriority(p string) bool {
	for _, x := range Priorities {
		if x == p {
			return true
		}
	}
	return false
}

// Task is one card.
type Task struct {
	ID         int64
	Title      string
	Body       string
	Status     string
	Priority   string
	Labels     []string
	AssigneeID int64
	Assignee   string
	CreatedBy  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DoneAt     *time.Time
}

// Column is a status with its tasks.
type Column struct {
	Status
	Tasks []Task
}

// Filter narrows the board.
type Filter struct {
	Q        string
	Label    string
	Assignee int64
}

const nameOf = `COALESCE(NULLIF(%[1]s.display_name, ''), NULLIF(%[1]s.name, ''), NULLIF(%[1]s.steam_name, ''), 'Player #' || %[1]s.id)`

func taskSelect() string {
	return `SELECT t.id, t.title, t.body, t.status, t.priority, t.labels, COALESCE(t.assignee_id, 0),
		COALESCE(` + fmt.Sprintf(nameOf, "a") + `, ''), COALESCE(` + fmt.Sprintf(nameOf, "c") + `, ''),
		t.created_at, t.updated_at, t.done_at
		FROM dev_tasks t LEFT JOIN players a ON a.id = t.assignee_id LEFT JOIN players c ON c.id = t.created_by`
}

func scanTask(row pgx.Row) (Task, error) {
	var t Task
	err := row.Scan(&t.ID, &t.Title, &t.Body, &t.Status, &t.Priority, &t.Labels, &t.AssigneeID, &t.Assignee, &t.CreatedBy,
		&t.CreatedAt, &t.UpdatedAt, &t.DoneAt)
	return t, err
}

// Board returns every column with its tasks in order. Done shows the 50
// most recently finished.
func Board(ctx context.Context, pool *pgxpool.Pool, f Filter) ([]Column, error) {
	where := []string{"true"}
	args := []any{}
	if q := strings.TrimSpace(f.Q); q != "" {
		args = append(args, q)
		where = append(where, fmt.Sprintf("(t.title ILIKE '%%' || $%d || '%%' OR t.body ILIKE '%%' || $%d || '%%')", len(args), len(args)))
	}
	if f.Label != "" {
		args = append(args, f.Label)
		where = append(where, fmt.Sprintf("$%d = ANY(t.labels)", len(args)))
	}
	if f.Assignee != 0 {
		args = append(args, f.Assignee)
		where = append(where, fmt.Sprintf("t.assignee_id = $%d", len(args)))
	}
	rows, err := pool.Query(ctx, taskSelect()+` WHERE `+strings.Join(where, " AND ")+` ORDER BY t.sort, t.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := make([]Column, len(Statuses))
	idx := map[string]int{}
	for i, s := range Statuses {
		cols[i] = Column{Status: s}
		idx[s.Key] = i
	}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		cols[idx[t.Status]].Tasks = append(cols[idx[t.Status]].Tasks, t)
	}
	done := &cols[idx["done"]]
	sort.SliceStable(done.Tasks, func(i, j int) bool {
		a, b := done.Tasks[i].DoneAt, done.Tasks[j].DoneAt
		return a != nil && (b == nil || a.After(*b))
	})
	if len(done.Tasks) > 50 {
		done.Tasks = done.Tasks[:50]
	}
	return cols, rows.Err()
}

// Get returns one task.
func Get(ctx context.Context, pool *pgxpool.Pool, id int64) (Task, error) {
	return scanTask(pool.QueryRow(ctx, taskSelect()+` WHERE t.id = $1`, id))
}

func clean(t *Task) error {
	t.Title = strings.Join(strings.Fields(t.Title), " ")
	t.Body = strings.TrimSpace(t.Body)
	if t.Title == "" {
		return UserError("Give the task a title.")
	}
	if len(t.Title) > 140 {
		return UserError("Keep the title under 140 characters.")
	}
	if len(t.Body) > 10000 {
		return UserError("The description is too long.")
	}
	if t.Status == "" {
		t.Status = "todo"
	}
	if t.Priority == "" {
		t.Priority = "normal"
	}
	if !validStatus(t.Status) || !validPriority(t.Priority) {
		return UserError("Unknown status or priority.")
	}
	seen := map[string]bool{}
	var labels []string
	for _, l := range t.Labels {
		l = strings.ToLower(strings.Join(strings.Fields(l), "-"))
		if l != "" && len(l) <= 24 && !seen[l] {
			seen[l] = true
			labels = append(labels, l)
		}
	}
	if len(labels) > 6 {
		return UserError("Use at most 6 labels.")
	}
	t.Labels = labels
	if t.Labels == nil {
		t.Labels = []string{}
	}
	return nil
}

func assignee(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

// Create adds a task at the bottom of its column.
func Create(ctx context.Context, pool *pgxpool.Pool, by int64, t Task) (int64, error) {
	if err := clean(&t); err != nil {
		return 0, err
	}
	var id int64
	err := pool.QueryRow(ctx, `
		INSERT INTO dev_tasks (title, body, status, priority, labels, assignee_id, created_by, sort, done_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE((SELECT max(sort) FROM dev_tasks WHERE status = $3), 0) + 1,
		        CASE WHEN $3 = 'done' THEN now() END)
		RETURNING id`, t.Title, t.Body, t.Status, t.Priority, t.Labels, assignee(t.AssigneeID), assignee(by)).Scan(&id)
	return id, err
}

// Update saves a task's fields (moving column keeps it at the bottom).
func Update(ctx context.Context, pool *pgxpool.Pool, t Task) error {
	if err := clean(&t); err != nil {
		return err
	}
	tag, err := pool.Exec(ctx, `
		UPDATE dev_tasks SET title = $2, body = $3, priority = $5, labels = $6, assignee_id = $7, updated_at = now(),
		       sort = CASE WHEN status = $4 THEN sort ELSE COALESCE((SELECT max(sort) FROM dev_tasks WHERE status = $4), 0) + 1 END,
		       done_at = CASE WHEN $4 = 'done' THEN COALESCE(done_at, now()) END,
		       status = $4
		WHERE id = $1`, t.ID, t.Title, t.Body, t.Status, t.Priority, t.Labels, assignee(t.AssigneeID))
	if err == nil && tag.RowsAffected() == 0 {
		return UserError("That task no longer exists.")
	}
	return err
}

// Move puts a task in a column, just before the task before (0 = at the
// end of the column).
func Move(ctx context.Context, pool *pgxpool.Pool, id int64, status string, before int64) error {
	if !validStatus(status) {
		return UserError("Unknown column.")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `LOCK TABLE dev_tasks IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return err
	}
	var sortAt float64
	if before != 0 && before != id {
		var prev *float64
		var at float64
		err := tx.QueryRow(ctx, `SELECT sort FROM dev_tasks WHERE id = $1 AND status = $2`, before, status).Scan(&at)
		if err != nil {
			return UserError("That card moved; refresh and try again.")
		}
		_ = tx.QueryRow(ctx, `SELECT max(sort) FROM dev_tasks WHERE status = $1 AND sort < $2 AND id <> $3`, status, at, id).Scan(&prev)
		if prev == nil {
			sortAt = at - 1
		} else {
			sortAt = (*prev + at) / 2
		}
	} else {
		var max *float64
		_ = tx.QueryRow(ctx, `SELECT max(sort) FROM dev_tasks WHERE status = $1 AND id <> $2`, status, id).Scan(&max)
		if max != nil {
			sortAt = *max + 1
		} else {
			sortAt = 1
		}
	}
	tag, err := tx.Exec(ctx, `UPDATE dev_tasks SET status = $2, sort = $3, updated_at = now(),
		done_at = CASE WHEN $2 = 'done' THEN COALESCE(done_at, now()) END WHERE id = $1`, id, status, sortAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return UserError("That task no longer exists.")
	}
	return tx.Commit(ctx)
}

// Delete removes a task.
func Delete(ctx context.Context, pool *pgxpool.Pool, id int64) error {
	_, err := pool.Exec(ctx, `DELETE FROM dev_tasks WHERE id = $1`, id)
	return err
}

// Person is someone a task can be assigned to.
type Person struct {
	ID   int64
	Name string
	Rank string
}

// People are the staff who hold dev.tools (through their rank).
func People(ctx context.Context, pool *pgxpool.Pool) ([]Person, error) {
	rows, err := pool.Query(ctx, `
		SELECT p.id, `+fmt.Sprintf(nameOf, "p")+`, sr.display_name
		FROM players p JOIN staff_ranks sr ON sr.id = p.staff_rank_id
		WHERE p.staff_status = 'active' AND EXISTS (SELECT 1 FROM rank_permissions rp WHERE rp.rank_id = sr.id AND rp.command_key = 'dev.tools')
		ORDER BY sr.level DESC, 2`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Person
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.ID, &p.Name, &p.Rank); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Labels lists every label in use, alphabetically.
func Labels(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	rows, err := pool.Query(ctx, `SELECT DISTINCT unnest(labels) AS l FROM dev_tasks ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var l string
		if rows.Scan(&l) == nil {
			out = append(out, l)
		}
	}
	return out, rows.Err()
}
