// Package discord covers this app's two Discord integrations, deliberately
// kept separate (docs/WEBSITE.md §9): one-way webhook logs (this file, no
// bot needed) and the two-way bot (bot.go, a persistent gateway
// connection).
package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// SendWebhook posts a plain-content message to a Discord webhook URL.
// Fire-and-forget by design: a webhook failure is logged, never returned to
// the caller as something that should block or undo the action that
// triggered it (a ban still applies even if Discord is unreachable) -- see
// docs/WEBSITE.md §9.
func SendWebhook(ctx context.Context, webhookURL, content string) {
	if err := PostWebhook(ctx, webhookURL, content); err != nil {
		slog.Error("discord webhook: post failed", "error", err)
	}
}

// PostWebhook is SendWebhook that reports failure, for callers that retry
// (internal/audit's staff-log poster marks a row posted only on success).
// An empty webhookURL is a no-op success: the category isn't configured.
//
// Mentions are always disabled (allowed_mentions.parse = []): content here
// includes player-chosen text -- names, ticket subjects -- and without this
// a ticket titled "@everyone" would ping the whole server.
func PostWebhook(ctx context.Context, webhookURL, content string) error {
	if webhookURL == "" {
		return nil
	}

	body, err := json.Marshal(map[string]any{
		"content":          content,
		"allowed_mentions": map[string]any{"parse": []string{}},
	})
	if err != nil {
		return fmt.Errorf("discord webhook: marshal: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discord webhook: build request failed")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Not wrapping err: *url.Error embeds the full webhook URL, whose
		// path contains the webhook's secret token.
		return fmt.Errorf("discord webhook: request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Escape neutralises Discord markdown in user-supplied text so a player
// name like "**admin**" or "`x`" renders literally.
func Escape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `*`, `\*`, `_`, `\_`, "`", "\\`", `~`, `\~`, `|`, `\|`, `>`, `\>`, `#`, `\#`)
	return r.Replace(s)
}
