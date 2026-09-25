package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// A fresh connection per dashboard fetch, not a long-lived pool -- this is
// a single-user desktop app polling occasionally (not a web server under
// concurrent load), and it keeps "settings changed" trivially correct with
// no separate invalidation logic.
func connectDB(ctx context.Context, s Settings) (*pgx.Conn, error) {
	connStr := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s connect_timeout=5",
		s.PostgresHost, s.PostgresPort, s.PostgresDB, s.PostgresUser, s.PostgresPassword)
	return pgx.Connect(ctx, connStr)
}

// TestDatabaseConnection is a dedicated Settings-tab action, deliberately
// separate from GetDashboardData -- confirming credentials work shouldn't
// require navigating to the Dashboard tab first.
func (a *App) TestDatabaseConnection() error {
	settings, err := loadSettings()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()

	conn, err := connectDB(ctx, settings)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	return conn.Ping(ctx)
}

type PlayerCountPoint struct {
	Bucket string `json:"bucket"`
	Count  int    `json:"count"`
}

type EconomySnapshot struct {
	TotalCash   int64 `json:"totalCash"`
	TotalBank   int64 `json:"totalBank"`
	PlayerCount int64 `json:"playerCount"`
}

type AntiCheatFlagCount struct {
	FlagType   string `json:"flagType"`
	Total      int    `json:"total"`
	Unreviewed int    `json:"unreviewed"`
}

type StaffLogEntry struct {
	Action    string `json:"action"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"createdAt"`
}

// DashboardData is one shot of everything the Dashboard tab renders. Error
// is set only for a connection-level failure (can't reach Postgres at
// all) -- an empty *_db_ with no rows yet (this project hasn't launched,
// nobody's played) is not an error, it's just an honestly empty graph.
type DashboardData struct {
	PlayerCounts   []PlayerCountPoint   `json:"playerCounts"`
	Economy        EconomySnapshot      `json:"economy"`
	AntiCheatFlags []AntiCheatFlagCount `json:"antiCheatFlags"`
	RecentStaffLog []StaffLogEntry      `json:"recentStaffLog"`
	Error          string               `json:"error,omitempty"`
}

func (a *App) GetDashboardData() DashboardData {
	settings, err := loadSettings()
	if err != nil {
		return DashboardData{Error: err.Error()}
	}

	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	conn, err := connectDB(ctx, settings)
	if err != nil {
		return DashboardData{Error: fmt.Sprintf("could not connect to Postgres: %v", err)}
	}
	defer conn.Close(ctx)

	data := DashboardData{}

	data.PlayerCounts = fetchPlayerCounts(ctx, conn)
	data.Economy = fetchEconomySnapshot(ctx, conn)
	data.AntiCheatFlags = fetchAntiCheatFlagCounts(ctx, conn)
	data.RecentStaffLog = fetchRecentStaffLog(ctx, conn)

	return data
}

// Each fetch* function swallows its own query error and returns a zero
// value rather than failing the whole dashboard -- one table being empty
// or briefly locked shouldn't blank out every other graph.

func fetchPlayerCounts(ctx context.Context, conn *pgx.Conn) []PlayerCountPoint {
	rows, err := conn.Query(ctx, `
		SELECT date_trunc('day', connected_at) AS bucket, count(*)
		FROM player_sessions
		WHERE connected_at > now() - interval '7 days'
		GROUP BY bucket
		ORDER BY bucket
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var points []PlayerCountPoint
	for rows.Next() {
		var bucket time.Time
		var count int
		if err := rows.Scan(&bucket, &count); err != nil {
			continue
		}
		points = append(points, PlayerCountPoint{Bucket: bucket.Format("2006-01-02"), Count: count})
	}
	return points
}

func fetchEconomySnapshot(ctx context.Context, conn *pgx.Conn) EconomySnapshot {
	var snap EconomySnapshot
	_ = conn.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(civ_cash + cop_cash + medic_cash), 0),
			COALESCE(SUM(civ_bank + cop_bank + medic_bank), 0),
			count(*)
		FROM players
	`).Scan(&snap.TotalCash, &snap.TotalBank, &snap.PlayerCount)
	return snap
}

func fetchAntiCheatFlagCounts(ctx context.Context, conn *pgx.Conn) []AntiCheatFlagCount {
	rows, err := conn.Query(ctx, `
		SELECT flag_type,
			count(*),
			count(*) FILTER (WHERE reviewed_at IS NULL)
		FROM anti_cheat_flags
		GROUP BY flag_type
		ORDER BY flag_type
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var counts []AntiCheatFlagCount
	for rows.Next() {
		var c AntiCheatFlagCount
		if err := rows.Scan(&c.FlagType, &c.Total, &c.Unreviewed); err != nil {
			continue
		}
		counts = append(counts, c)
	}
	return counts
}

func fetchRecentStaffLog(ctx context.Context, conn *pgx.Conn) []StaffLogEntry {
	rows, err := conn.Query(ctx, `
		SELECT action, COALESCE(reason, ''), created_at
		FROM staff_log
		ORDER BY created_at DESC
		LIMIT 20
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var entries []StaffLogEntry
	for rows.Next() {
		var e StaffLogEntry
		var createdAt time.Time
		if err := rows.Scan(&e.Action, &e.Reason, &createdAt); err != nil {
			continue
		}
		e.CreatedAt = createdAt.Format(time.RFC3339)
		entries = append(entries, e)
	}
	return entries
}
