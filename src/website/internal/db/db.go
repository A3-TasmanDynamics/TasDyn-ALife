// Package db wraps the Postgres connection pool. A pool, not a fresh
// connection per call like src/server_manager/dashboard.go -- that pattern
// was correct for a desktop app doing one dashboard refresh at a time, but
// this is a web server handling concurrent requests, which is exactly what
// pgxpool exists for.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	return pool, nil
}
