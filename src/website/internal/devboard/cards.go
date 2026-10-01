package devboard

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

// Card is a task with everything its detail view shows.
type Card struct {
	Task
	Checklists []Checklist
	Links      []Link
	Comments   []Comment
	Activity   []Activity
}

// Checklist is one named list of items on a card.
type Checklist struct {
	ID    int64
	Title string
	Items []Item
	Done  int
}

// Percent is how much of the checklist is done, 0-100.
func (c Checklist) Percent() int {
	if len(c.Items) == 0 {
		return 0
	}
	return c.Done * 100 / len(c.Items)
}

// Item is one checklist line.
type Item struct {
	ID     int64
	Body   string
	Done   bool
	DoneBy string
}

// Link is another card this one is linked to, described from this card's
// side ("Blocks", "Blocked by"...).
type Link struct {
	OtherID     int64
	OtherTitle  string
	OtherStatus string
	Label       string
}

// LinkKinds are the links that can be added, with how they read from the
// card that adds them.
var LinkKinds = []struct{ Key, Label string }{
	{"relates", "Relates to"}, {"blocks", "Blocks"}, {"blocked_by", "Is blocked by"}, {"duplicates", "Duplicates"},
}

// Comment is one comment on a card.
type Comment struct {
	ID       int64
	AuthorID int64
	Author   string
	Body     string
	At       time.Time
}

// Activity is one line of a card's history.
type Activity struct {
	Actor string
	What  string
	At    time.Time
}

// GetCard loads a task with its checklists, links, comments and history.
func GetCard(ctx context.Context, pool *pgxpool.Pool, id int64) (Card, error) {
	var c Card
	var err error
	if c.Task, err = Get(ctx, pool, id); err != nil {
		return c, err
	}

	rows, err := pool.Query(ctx, `
		SELECT l.id, l.title, i.id, COALESCE(i.body, ''), COALESCE(i.done, false), COALESCE(`+fmt.Sprintf(nameOf, "p")+`, '')
		FROM dev_task_checklists l
		LEFT JOIN dev_task_checklist_items i ON i.checklist_id = l.id
		LEFT JOIN players p ON p.id = i.done_by
		WHERE l.task_id = $1 ORDER BY l.sort, l.id, i.sort, i.id`, id)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var lid int64
		var title string
		var iid *int64
		var it Item
		if err := rows.Scan(&lid, &title, &iid, &it.Body, &it.Done, &it.DoneBy); err != nil {
			rows.Close()
			return c, err
		}
		if n := len(c.Checklists); n == 0 || c.Checklists[n-1].ID != lid {
			c.Checklists = append(c.Checklists, Checklist{ID: lid, Title: title})
		}
		if iid != nil {
			it.ID = *iid
			cl := &c.Checklists[len(c.Checklists)-1]
			cl.Items = append(cl.Items, it)
			if it.Done {
				cl.Done++
			}
		}
	}
	rows.Close()

	rows, err = pool.Query(ctx, `
		SELECT k.other_id, o.title, o.status, CASE k.kind WHEN 'blocks' THEN 'Blocks' WHEN 'duplicates' THEN 'Duplicates' ELSE 'Relates to' END
		FROM dev_task_links k JOIN dev_tasks o ON o.id = k.other_id WHERE k.task_id = $1
		UNION ALL
		SELECT k.task_id, o.title, o.status, CASE k.kind WHEN 'blocks' THEN 'Blocked by' WHEN 'duplicates' THEN 'Duplicated by' ELSE 'Relates to' END
		FROM dev_task_links k JOIN dev_tasks o ON o.id = k.task_id WHERE k.other_id = $1
		ORDER BY 4, 1`, id)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.OtherID, &l.OtherTitle, &l.OtherStatus, &l.Label); err != nil {
			rows.Close()
			return c, err
		}
		c.Links = append(c.Links, l)
	}
	rows.Close()

	rows, err = pool.Query(ctx, `
		SELECT m.id, COALESCE(m.author_id, 0), COALESCE(`+fmt.Sprintf(nameOf, "p")+`, 'Someone'), m.body, m.created_at
		FROM dev_task_comments m LEFT JOIN players p ON p.id = m.author_id
		WHERE m.task_id = $1 ORDER BY m.id DESC`, id)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var m Comment
		if err := rows.Scan(&m.ID, &m.AuthorID, &m.Author, &m.Body, &m.At); err != nil {
			rows.Close()
			return c, err
		}
		c.Comments = append(c.Comments, m)
	}
	rows.Close()

	rows, err = pool.Query(ctx, `
		SELECT COALESCE(`+fmt.Sprintf(nameOf, "p")+`, 'Someone'), a.what, a.created_at
		FROM dev_task_activity a LEFT JOIN players p ON p.id = a.actor_id
		WHERE a.task_id = $1 ORDER BY a.id DESC LIMIT 50`, id)
	if err != nil {
		return c, err
	}
	defer rows.Close()
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.Actor, &a.What, &a.At); err != nil {
			return c, err
		}
		c.Activity = append(c.Activity, a)
	}
	return c, rows.Err()
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// AddChecklist adds a named checklist to a card.
func AddChecklist(ctx context.Context, pool *pgxpool.Pool, actor, taskID int64, title string) error {
	title = oneLine(title)
	if title == "" {
		title = "Checklist"
	}
	if len(title) > 80 {
		return UserError("Keep the checklist name under 80 characters.")
	}
	_, err := pool.Exec(ctx, `INSERT INTO dev_task_checklists (task_id, title, sort)
		VALUES ($1, $2, COALESCE((SELECT max(sort) FROM dev_task_checklists WHERE task_id = $1), 0) + 1)`, taskID, title)
	if err != nil {
		return notFound(err)
	}
	logActivity(ctx, pool, taskID, actor, "added the checklist "+title)
	touch(ctx, pool, taskID)
	return nil
}

// DeleteChecklist removes a checklist and its items.
func DeleteChecklist(ctx context.Context, pool *pgxpool.Pool, actor, taskID, checklistID int64) error {
	var title string
	err := pool.QueryRow(ctx, `DELETE FROM dev_task_checklists WHERE id = $1 AND task_id = $2 RETURNING title`, checklistID, taskID).Scan(&title)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserError("That checklist is already gone.")
	}
	if err == nil {
		logActivity(ctx, pool, taskID, actor, "removed the checklist "+title)
		touch(ctx, pool, taskID)
	}
	return err
}

// AddItem adds an item to the end of a checklist on taskID.
func AddItem(ctx context.Context, pool *pgxpool.Pool, taskID, checklistID int64, body string) error {
	body = oneLine(body)
	if body == "" {
		return UserError("Write the item first.")
	}
	if len(body) > 300 {
		return UserError("Keep checklist items under 300 characters.")
	}
	tag, err := pool.Exec(ctx, `INSERT INTO dev_task_checklist_items (checklist_id, body, sort)
		SELECT l.id, $3, COALESCE((SELECT max(sort) FROM dev_task_checklist_items WHERE checklist_id = l.id), 0) + 1
		FROM dev_task_checklists l WHERE l.id = $2 AND l.task_id = $1`, taskID, checklistID, body)
	if err == nil && tag.RowsAffected() == 0 {
		return UserError("That checklist is gone.")
	}
	touch(ctx, pool, taskID)
	return err
}

// SetItem ticks or unticks an item. It returns the item's checklist
// progress afterwards.
func SetItem(ctx context.Context, pool *pgxpool.Pool, actor, taskID, itemID int64, done bool) (doneCount, total int, err error) {
	var body string
	err = pool.QueryRow(ctx, `
		UPDATE dev_task_checklist_items i SET done = $3,
		       done_by = CASE WHEN $3 THEN $4::bigint END, done_at = CASE WHEN $3 THEN now() END
		FROM dev_task_checklists l
		WHERE i.id = $2 AND l.id = i.checklist_id AND l.task_id = $1
		RETURNING i.body`, taskID, itemID, done, assignee(actor)).Scan(&body)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, UserError("That item is gone.")
	}
	if err != nil {
		return 0, 0, err
	}
	if done {
		logActivity(ctx, pool, taskID, actor, "completed "+body)
	} else {
		logActivity(ctx, pool, taskID, actor, "marked "+body+" incomplete")
	}
	touch(ctx, pool, taskID)
	err = pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE i.done), count(*) FROM dev_task_checklist_items i
		JOIN dev_task_checklists l ON l.id = i.checklist_id WHERE l.task_id = $1`, taskID).Scan(&doneCount, &total)
	return doneCount, total, err
}

// DeleteItem removes an item.
func DeleteItem(ctx context.Context, pool *pgxpool.Pool, taskID, itemID int64) error {
	_, err := pool.Exec(ctx, `DELETE FROM dev_task_checklist_items i USING dev_task_checklists l
		WHERE i.id = $2 AND l.id = i.checklist_id AND l.task_id = $1`, taskID, itemID)
	touch(ctx, pool, taskID)
	return err
}

// AddLink links two cards. kind is relates, blocks, blocked_by (stored as
// the other card blocking this one) or duplicates.
func AddLink(ctx context.Context, pool *pgxpool.Pool, actor, taskID, otherID int64, kind string) error {
	if otherID == taskID {
		return UserError("A card can't link to itself.")
	}
	from, to := taskID, otherID
	switch kind {
	case "relates", "blocks", "duplicates":
	case "blocked_by":
		from, to, kind = otherID, taskID, "blocks"
	default:
		return UserError("Pick how the cards are linked.")
	}
	var exists bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM dev_task_links WHERE (task_id = $1 AND other_id = $2) OR (task_id = $2 AND other_id = $1))`,
		taskID, otherID).Scan(&exists)
	if exists {
		return UserError("These cards are already linked; remove that link first to change it.")
	}
	_, err := pool.Exec(ctx, `INSERT INTO dev_task_links (task_id, other_id, kind, created_by) VALUES ($1, $2, $3, $4)`, from, to, kind, assignee(actor))
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23503" {
			return UserError(fmt.Sprintf("There's no card #%d.", otherID))
		}
		return err
	}
	label := map[string]string{"relates": "linked this card to", "blocks": "marked this card as blocking", "blocked_by": "marked this card as blocked by", "duplicates": "marked this card as a duplicate of"}
	orig := kind
	if from != taskID {
		orig = "blocked_by"
	}
	logActivity(ctx, pool, taskID, actor, fmt.Sprintf("%s #%d", label[orig], otherID))
	logActivity(ctx, pool, otherID, actor, fmt.Sprintf("linked #%d to this card", taskID))
	touch(ctx, pool, taskID)
	return nil
}

// RemoveLink removes the link between two cards, whichever way it reads.
func RemoveLink(ctx context.Context, pool *pgxpool.Pool, actor, taskID, otherID int64) error {
	tag, err := pool.Exec(ctx, `DELETE FROM dev_task_links WHERE (task_id = $1 AND other_id = $2) OR (task_id = $2 AND other_id = $1)`, taskID, otherID)
	if err == nil && tag.RowsAffected() > 0 {
		logActivity(ctx, pool, taskID, actor, fmt.Sprintf("removed the link to #%d", otherID))
		touch(ctx, pool, taskID)
	}
	return err
}

// AddComment adds a comment by actor.
func AddComment(ctx context.Context, pool *pgxpool.Pool, actor, taskID int64, body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return UserError("Write a comment first.")
	}
	if len(body) > 4000 {
		return UserError("Keep comments under 4,000 characters.")
	}
	_, err := pool.Exec(ctx, `INSERT INTO dev_task_comments (task_id, author_id, body) VALUES ($1, $2, $3)`, taskID, assignee(actor), body)
	if err != nil {
		return notFound(err)
	}
	touch(ctx, pool, taskID)
	return nil
}

// DeleteComment removes a comment; only its author can.
func DeleteComment(ctx context.Context, pool *pgxpool.Pool, actor, taskID, commentID int64) error {
	tag, err := pool.Exec(ctx, `DELETE FROM dev_task_comments WHERE id = $1 AND task_id = $2 AND author_id = $3`, commentID, taskID, actor)
	if err == nil && tag.RowsAffected() == 0 {
		return UserError("You can only delete your own comments.")
	}
	return err
}

// Options are every card's id and title, for linking.
func Options(ctx context.Context, pool *pgxpool.Pool) ([]Task, error) {
	rows, err := pool.Query(ctx, `SELECT id, title, status FROM dev_tasks ORDER BY id DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Status); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func touch(ctx context.Context, pool *pgxpool.Pool, taskID int64) {
	_, _ = pool.Exec(ctx, `UPDATE dev_tasks SET updated_at = now() WHERE id = $1`, taskID)
}

func notFound(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23503" {
		return UserError("That card no longer exists.")
	}
	return err
}
