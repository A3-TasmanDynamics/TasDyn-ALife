// Package discord covers this app's two Discord integrations, deliberately
// kept separate (docs/WEBSITE.md §9): one-way webhook logs (this file, no
// bot needed) and the two-way bot (bot.go, a persistent gateway
// connection).
package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// SendWebhook posts a plain-content message to a Discord webhook URL.
// Fire-and-forget by design: a webhook failure is logged, never returned to
// the caller as something that should block or undo the action that
// triggered it (a ban still applies even if Discord is unreachable) -- see
// docs/WEBSITE.md §9.
func SendWebhook(ctx context.Context, webhookURL, content string) {
	if webhookURL == "" {
		return // this log category simply isn't configured -- not an error
	}

	body, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		slog.Error("discord webhook: marshal failed", "error", err)
		return
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		slog.Error("discord webhook: build request failed", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("discord webhook: request failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		slog.Error("discord webhook: non-2xx response", "status", resp.StatusCode)
	}
}
