package main

import (
	"context"
	"os"
	"testing"
)

// Integration test against a real local Postgres instance -- verifies the
// Logs tab's SQL is actually valid against database/schema.sql, not just
// that it compiles. Skipped by default (needs a live DB); run explicitly:
//
//	ALIFE_TEST_DB=1 go test ./... -run TestLogQueries -v
func TestLogQueries(t *testing.T) {
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

	t.Run("StaffLog", func(t *testing.T) {
		entries, err := fetchStaffLog(ctx, conn)
		if err != nil {
			t.Fatalf("fetchStaffLog: %v", err)
		}
		t.Logf("got %d entries", len(entries))
	})

	t.Run("AntiCheatLog", func(t *testing.T) {
		entries, err := fetchAntiCheatLog(ctx, conn)
		if err != nil {
			t.Fatalf("fetchAntiCheatLog: %v", err)
		}
		t.Logf("got %d entries", len(entries))
	})

	t.Run("KickLog", func(t *testing.T) {
		entries, err := fetchKickLog(ctx, conn)
		if err != nil {
			t.Fatalf("fetchKickLog: %v", err)
		}
		t.Logf("got %d entries", len(entries))
	})
}
