package handlers

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// categoryOption is one row of ticket_categories -- either a top-level
// category (ParentID nil) or a subcategory of one (ParentID set). See
// database/schema.sql's comment on ticket_categories for why this is a
// self-referential lookup table rather than a hardcoded list.
type categoryOption struct {
	ID       int
	Key      string
	Label    string
	ParentID *int
}

// fetchCategoryTree loads the whole ticket_categories table, split into
// top-level categories and subcategories -- both the new-ticket form and
// the queue's category filter need the full tree, just rendered
// differently (a cascading pair of selects vs. a flat filter dropdown).
func fetchCategoryTree(ctx context.Context, pool *pgxpool.Pool) (topLevel, subcategories []categoryOption, err error) {
	rows, err := pool.Query(ctx, `
		SELECT id, key, label, parent_id FROM ticket_categories ORDER BY parent_id NULLS FIRST, sort_order
	`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var c categoryOption
		if err := rows.Scan(&c.ID, &c.Key, &c.Label, &c.ParentID); err != nil {
			continue
		}
		if c.ParentID == nil {
			topLevel = append(topLevel, c)
		} else {
			subcategories = append(subcategories, c)
		}
	}
	return topLevel, subcategories, rows.Err()
}

var (
	ErrCategoryNotTopLevel = errors.New("category must be a top-level category, not a subcategory")
	ErrSubcategoryMismatch = errors.New("that subcategory doesn't belong to the selected category")
	ErrCategoryInvalid     = errors.New("please choose a category")
)

// validateCategoryPair re-checks a submitted (category_id, subcategory_id)
// pair against the real table server-side -- never trusts that a category
// dropdown's HTML options were the ones actually rendered, since nothing
// stops a raw POST from naming an arbitrary ID. subcategoryID of 0 means
// "no subcategory chosen", always valid.
func validateCategoryPair(ctx context.Context, pool *pgxpool.Pool, categoryID, subcategoryID int) error {
	var parentID *int
	err := pool.QueryRow(ctx, `SELECT parent_id FROM ticket_categories WHERE id = $1`, categoryID).Scan(&parentID)
	if err == pgx.ErrNoRows {
		return ErrCategoryInvalid
	}
	if err != nil {
		return err
	}
	if parentID != nil {
		return ErrCategoryNotTopLevel
	}

	if subcategoryID == 0 {
		return nil
	}
	var subParentID *int
	err = pool.QueryRow(ctx, `SELECT parent_id FROM ticket_categories WHERE id = $1`, subcategoryID).Scan(&subParentID)
	if err == pgx.ErrNoRows {
		return ErrSubcategoryMismatch
	}
	if err != nil {
		return err
	}
	if subParentID == nil || *subParentID != categoryID {
		return ErrSubcategoryMismatch
	}
	return nil
}
