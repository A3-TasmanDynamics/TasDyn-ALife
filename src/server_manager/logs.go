package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// A fresh connection per fetch, not a long-lived pool -- this is a
// single-user desktop app polling occasionally (not a web server under
// concurrent load), and it keeps "settings changed" trivially correct with
// no separate invalidation logic.
func connectDB(ctx context.Context, s Settings) (*pgx.Conn, error) {
	connStr := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s connect_timeout=5",
		s.PostgresHost, s.PostgresPort, s.PostgresDB, s.PostgresUser, s.PostgresPassword)
	return pgx.Connect(ctx, connStr)
}

// TestDatabaseConnection is a dedicated Settings-tab action, deliberately
// separate from the log fetchers below -- confirming credentials work
// shouldn't require navigating to the Logs tab first.
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

// The Logs tab shows one log type at a time, picked from a selector --
// each type gets its own row-level fetch* function (testable directly
// against a connection, see logs_test.go) plus a thin bound method that
// handles settings/connecting -- the same split the original Dashboard
// code used, kept here so the SQL stays testable without going through
// Wails or touching saved settings.

func withDBConn[T any](a *App, fetch func(context.Context, *pgx.Conn) (T, error)) (T, error) {
	var zero T
	settings, err := loadSettings()
	if err != nil {
		return zero, err
	}

	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	conn, err := connectDB(ctx, settings)
	if err != nil {
		return zero, fmt.Errorf("could not connect to Postgres: %w", err)
	}
	defer conn.Close(ctx)

	return fetch(ctx, conn)
}

type StaffLogEntry struct {
	StaffName  string `json:"staffName"`
	TargetName string `json:"targetName"`
	Action     string `json:"action"`
	Reason     string `json:"reason"`
	CreatedAt  string `json:"createdAt"`
}

func (a *App) GetStaffLog() ([]StaffLogEntry, error) {
	return withDBConn(a, fetchStaffLog)
}

func fetchStaffLog(ctx context.Context, conn *pgx.Conn) ([]StaffLogEntry, error) {
	rows, err := conn.Query(ctx, `
		SELECT COALESCE(staff.name, 'System'), COALESCE(target.name, ''), sl.action, COALESCE(sl.reason, ''), sl.created_at
		FROM staff_log sl
		LEFT JOIN players staff ON staff.id = sl.staff_player_id
		LEFT JOIN players target ON target.id = sl.target_player_id
		ORDER BY sl.created_at DESC
		LIMIT 50
	`)
	if err != nil {
		return nil, fmt.Errorf("query staff_log: %w", err)
	}
	defer rows.Close()

	var entries []StaffLogEntry
	for rows.Next() {
		var e StaffLogEntry
		var createdAt time.Time
		if err := rows.Scan(&e.StaffName, &e.TargetName, &e.Action, &e.Reason, &createdAt); err != nil {
			continue
		}
		e.CreatedAt = createdAt.Format(time.RFC3339)
		entries = append(entries, e)
	}
	return entries, nil
}

type AntiCheatLogEntry struct {
	PlayerName string `json:"playerName"`
	FlagType   string `json:"flagType"`
	Confidence string `json:"confidence"`
	Resolution string `json:"resolution"` // empty = unreviewed
	CreatedAt  string `json:"createdAt"`
}

func (a *App) GetAntiCheatLog() ([]AntiCheatLogEntry, error) {
	return withDBConn(a, fetchAntiCheatLog)
}

func fetchAntiCheatLog(ctx context.Context, conn *pgx.Conn) ([]AntiCheatLogEntry, error) {
	rows, err := conn.Query(ctx, `
		SELECT p.name, acf.flag_type, acf.confidence, COALESCE(acf.resolution, ''), acf.created_at
		FROM anti_cheat_flags acf
		JOIN players p ON p.id = acf.player_id
		ORDER BY acf.created_at DESC
		LIMIT 50
	`)
	if err != nil {
		return nil, fmt.Errorf("query anti_cheat_flags: %w", err)
	}
	defer rows.Close()

	var entries []AntiCheatLogEntry
	for rows.Next() {
		var e AntiCheatLogEntry
		var createdAt time.Time
		if err := rows.Scan(&e.PlayerName, &e.FlagType, &e.Confidence, &e.Resolution, &createdAt); err != nil {
			continue
		}
		e.CreatedAt = createdAt.Format(time.RFC3339)
		entries = append(entries, e)
	}
	return entries, nil
}

type KickLogEntry struct {
	TargetName string `json:"targetName"`
	KickedBy   string `json:"kickedBy"` // "System" = BattlEye/anti-cheat/vote, not a staff member
	KickType   string `json:"kickType"`
	Reason     string `json:"reason"`
	CreatedAt  string `json:"createdAt"`
}

func (a *App) GetKickLog() ([]KickLogEntry, error) {
	return withDBConn(a, fetchKickLog)
}

func fetchKickLog(ctx context.Context, conn *pgx.Conn) ([]KickLogEntry, error) {
	rows, err := conn.Query(ctx, `
		SELECT target.name, COALESCE(kicker.name, 'System'), kl.kick_type, COALESCE(kl.reason, ''), kl.created_at
		FROM kick_log kl
		JOIN players target ON target.id = kl.target_player_id
		LEFT JOIN players kicker ON kicker.id = kl.kicked_by
		ORDER BY kl.created_at DESC
		LIMIT 50
	`)
	if err != nil {
		return nil, fmt.Errorf("query kick_log: %w", err)
	}
	defer rows.Close()

	var entries []KickLogEntry
	for rows.Next() {
		var e KickLogEntry
		var createdAt time.Time
		if err := rows.Scan(&e.TargetName, &e.KickedBy, &e.KickType, &e.Reason, &createdAt); err != nil {
			continue
		}
		e.CreatedAt = createdAt.Format(time.RFC3339)
		entries = append(entries, e)
	}
	return entries, nil
}
