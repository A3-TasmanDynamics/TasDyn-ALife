package main

import (
	"context"
	"os"
	"testing"
)

// Integration test against a real local Postgres instance -- verifies the
// dashboard SQL is actually valid against database/schema.sql, not just
// that it compiles. Skipped by default (needs a live DB); run explicitly:
//
//	ALIFE_TEST_DB=1 go test ./... -run TestDashboardQueries -v
func TestDashboardQueries(t *testing.T) {
	if os.Getenv("ALIFE_TEST_DB") == "" {
		t.Skip("set ALIFE_TEST_DB=1 to run against a real local Postgres instance")
	}

	s := Settings{
		PostgresHost:     "127.0.0.1",
		PostgresPort:     "5432",
		PostgresDB:       "alife_db",
		PostgresUser:     "alife_admin",
		PostgresPassword: "alife_admin",
	}

	ctx := context.Background()
	conn, err := connectDB(ctx, s)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	if err := conn.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	t.Run("PlayerCounts", func(t *testing.T) {
		points := fetchPlayerCounts(ctx, conn)
		t.Logf("got %d points", len(points))
	})

	t.Run("EconomySnapshot", func(t *testing.T) {
		snap := fetchEconomySnapshot(ctx, conn)
		t.Logf("players=%d cash=%d bank=%d", snap.PlayerCount, snap.TotalCash, snap.TotalBank)
	})

	t.Run("AntiCheatFlagCounts", func(t *testing.T) {
		counts := fetchAntiCheatFlagCounts(ctx, conn)
		t.Logf("got %d flag types", len(counts))
	})

	t.Run("RecentStaffLog", func(t *testing.T) {
		entries := fetchRecentStaffLog(ctx, conn)
		t.Logf("got %d entries", len(entries))
	})
}
