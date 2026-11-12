// Package db wraps the Postgres connection pool. A pool, not a fresh
// connection per call like src/server_manager/logs.go -- that pattern
// was correct for a desktop app doing one dashboard refresh at a time, but
// this is a web server handling concurrent requests, which is exactly what
// pgxpool exists for.
//
// Set DB_SLOW_QUERY_MS (e.g. 50) to log every query slower than that, with
// its duration and the start of its SQL -- for finding slow pages.
package db

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}
	if ms, _ := strconv.Atoi(os.Getenv("DB_SLOW_QUERY_MS")); ms > 0 {
		cfg.ConnConfig.Tracer = &slowLog{threshold: time.Duration(ms) * time.Millisecond}
		slog.Info("db: logging slow queries", "over_ms", ms)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	return pool, nil
}

// slowLog is a pgx tracer that logs queries slower than threshold.
type slowLog struct{ threshold time.Duration }

type slowKey struct{}

type slowStart struct {
	at  time.Time
	sql string
}

func (t *slowLog) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, slowKey{}, slowStart{time.Now(), data.SQL})
}

func (t *slowLog) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	st, ok := ctx.Value(slowKey{}).(slowStart)
	if !ok {
		return
	}
	if d := time.Since(st.at); d >= t.threshold {
		sql := strings.Join(strings.Fields(st.sql), " ")
		if len(sql) > 240 {
			sql = sql[:240] + "..."
		}
		slog.Warn("db: slow query", "ms", d.Milliseconds(), "sql", sql, "error", data.Err)
	}
}
