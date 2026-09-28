package discord

import (
	"context"
	"errors"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fakeSender struct {
	mu      sync.Mutex
	sent    []string // "channelID:content"
	dmFails map[string]error
	failing error
}

func (f *fakeSender) UserChannelCreate(recipientID string, _ ...discordgo.RequestOption) (*discordgo.Channel, error) {
	if err := f.dmFails[recipientID]; err != nil {
		return nil, err
	}
	return &discordgo.Channel{ID: "dm-" + recipientID}, nil
}

func (f *fakeSender) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failing != nil {
		return nil, f.failing
	}
	if data.AllowedMentions == nil || len(data.AllowedMentions.Parse) != 0 {
		return nil, errors.New("mentions not disabled")
	}
	f.sent = append(f.sent, channelID+":"+data.Content)
	return &discordgo.Message{}, nil
}

// Runs only with TEST_DATABASE_URL.
func TestOutbox(t *testing.T) {
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
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM discord_outbox WHERE dedupe_key LIKE 'outbox-test:%'`) })

	enqueue := func(m OutboxMessage) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := Enqueue(ctx, tx, m); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	enqueue(OutboxMessage{Kind: KindDM, Target: "111", Content: "hello @everyone", DedupeKey: "outbox-test:a"})
	enqueue(OutboxMessage{Kind: KindDM, Target: "111", Content: "duplicate", DedupeKey: "outbox-test:a"}) // ignored
	enqueue(OutboxMessage{Kind: KindDM, Target: "closed", Content: "x", DedupeKey: "outbox-test:b"})
	enqueue(OutboxMessage{Kind: KindChannelPost, Target: "chan", Content: "post", DedupeKey: "outbox-test:c"})

	closedErr := &discordgo.RESTError{Message: &discordgo.APIErrorMessage{Code: discordDMClosed}, Response: &http.Response{StatusCode: http.StatusForbidden}}
	fake := &fakeSender{dmFails: map[string]error{"closed": closedErr}}
	(&Outbox{Pool: pool, Sender: fake}).deliverDue(ctx)

	if len(fake.sent) != 2 || fake.sent[0] != "dm-111:hello @everyone" || fake.sent[1] != "chan:post" {
		t.Fatalf("sent = %v", fake.sent)
	}
	status := func(key string) (sent, gaveUp bool, attempts int) {
		pool.QueryRow(ctx, `SELECT sent_at IS NOT NULL, gave_up_at IS NOT NULL, attempts FROM discord_outbox WHERE dedupe_key = $1`, key).Scan(&sent, &gaveUp, &attempts)
		return
	}
	if s, g, _ := status("outbox-test:a"); !s || g {
		t.Errorf("a: sent=%v gaveUp=%v", s, g)
	}
	if s, g, _ := status("outbox-test:b"); s || !g {
		t.Errorf("b (DMs closed): sent=%v gaveUp=%v, want given up", s, g)
	}

	// A transient failure is retried later, not given up.
	enqueue(OutboxMessage{Kind: KindChannelPost, Target: "chan", Content: "later", DedupeKey: "outbox-test:d"})
	fake.failing = errors.New("temporary network error")
	(&Outbox{Pool: pool, Sender: fake}).deliverDue(ctx)
	if s, g, n := status("outbox-test:d"); s || g || n != 1 {
		t.Errorf("d (transient): sent=%v gaveUp=%v attempts=%d", s, g, n)
	}
	var later bool
	pool.QueryRow(ctx, `SELECT next_attempt > now() FROM discord_outbox WHERE dedupe_key = 'outbox-test:d'`).Scan(&later)
	if !later {
		t.Error("d: next_attempt should be pushed into the future")
	}
}
