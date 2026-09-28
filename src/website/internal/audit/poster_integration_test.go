package audit

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runs only with TEST_DATABASE_URL set (a dev database with the schema
// applied). The test row is back-dated 30h so a live website's own poster
// (24h window) never picks it up and posts it to a real channel.
func TestPosterPostsAndMarksOnce(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close) // registered first so it runs last, after the other cleanups

	var mu sync.Mutex
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		mu.Lock()
		bodies = append(bodies, m)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var id int64
	err = pool.QueryRow(ctx, `
		INSERT INTO staff_log (action, reason, source, created_at)
		VALUES ('integration_test', '@everyone test', 'manual', now() - interval '30 hours')
		RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM staff_log WHERE id = $1`, id)

	p := &Poster{Pool: pool, WebhookURL: srv.URL, MaxAge: 48 * time.Hour}
	p.postPending(ctx)
	p.postPending(ctx) // second pass must not re-post

	var posted *time.Time
	if err := pool.QueryRow(ctx, `SELECT discord_posted_at FROM staff_log WHERE id = $1`, id).Scan(&posted); err != nil {
		t.Fatal(err)
	}
	if posted == nil {
		t.Fatal("row was not marked posted")
	}

	mu.Lock()
	defer mu.Unlock()
	matches := 0
	for _, b := range bodies {
		content, _ := b["content"].(string)
		if strings.Contains(content, "integration\\_test") {
			matches++
			am, _ := b["allowed_mentions"].(map[string]any)
			if parse, ok := am["parse"].([]any); !ok || len(parse) != 0 {
				t.Errorf("mentions not disabled: %v", b["allowed_mentions"])
			}
			if !strings.Contains(content, "Manual database change") {
				t.Errorf("attribution missing: %q", content)
			}
		}
	}
	if matches != 1 {
		t.Fatalf("expected exactly 1 post for the test row, got %d", matches)
	}
}
