package discord

// Staff alerts: the bot posts to #staff-alerts when something needs a
// staff member's eyes -- a high-confidence anti-cheat flag, a large amount
// of money moving in one go, or the evidence vault being broken into.
// It polls the database every 30 s and remembers how far it got
// (state.alerts_flag / state.alerts_log), so each alert is posted once and
// a restart doesn't repost or flood old history.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	stateAlertsFlag = "state.alerts_flag" // last anti_cheat_flags.id looked at
	stateAlertsLog  = "state.alerts_log"  // last player_log.id looked at

	// maxAlertsPerPoll keeps a burst (or a big backlog) from flooding the
	// channel: the rest are summed up in one line.
	maxAlertsPerPoll = 15
)

// AlertConfig is what the settings ask for.
type AlertConfig struct {
	Flags    bool  // high-confidence anti-cheat flags
	Vault    bool  // evidence vault break-ins and robberies
	Money    int64 // dollars; 0 = off
	SiteBase string
}

// Alert is one message for #staff-alerts.
type Alert struct {
	Key  string // dedupe key: "alert:flag:12", "alert:log:345"
	Text string
}

// moneyEvents: event type -> the details key with the dollars, and words.
var moneyEvents = map[string][2]string{
	"bank.transfer":   {"amount", "transferred"},
	"give.cash":       {"amount", "handed over in cash"},
	"shop.sell":       {"total", "sold goods for"},
	"garage.sell":     {"price", "sold a vehicle for"},
	"robbery.gas":     {"cash", "robbed a gas station for"},
	"evidence.robbed": {"cash", "robbed the evidence vault for"},
}

// CollectAlerts returns the alerts since the cursors (flag id, log id) and
// the new cursors. A cursor below 0 (none saved yet) starts from now:
// nothing old is posted.
func CollectAlerts(ctx context.Context, pool *pgxpool.Pool, cfg AlertConfig, flagCur, logCur int64) ([]Alert, int64, int64, error) {
	var maxFlag, maxLog int64
	if err := pool.QueryRow(ctx, `SELECT COALESCE((SELECT max(id) FROM anti_cheat_flags), 0), COALESCE((SELECT max(id) FROM player_log), 0)`).Scan(&maxFlag, &maxLog); err != nil {
		return nil, flagCur, logCur, err
	}
	if flagCur < 0 || flagCur > maxFlag {
		flagCur = maxFlag
	}
	if logCur < 0 || logCur > maxLog {
		logCur = maxLog
	}
	var out []Alert
	link := func(path string) string {
		if cfg.SiteBase == "" {
			return ""
		}
		return " <" + strings.TrimRight(cfg.SiteBase, "/") + path + ">"
	}

	if cfg.Flags && maxFlag > flagCur {
		rows, err := pool.Query(ctx, `
			SELECT f.id, f.flag_type, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id), p.uid, COALESCE(f.details::text, '')
			FROM anti_cheat_flags f JOIN players p ON p.id = f.player_id
			WHERE f.id > $1 AND f.id <= $2 AND f.confidence = 'high' ORDER BY f.id`, flagCur, maxFlag)
		if err != nil {
			return nil, flagCur, logCur, err
		}
		for rows.Next() {
			var id int64
			var typ, name, uid, details string
			if err := rows.Scan(&id, &typ, &name, &uid, &details); err != nil {
				rows.Close()
				return nil, flagCur, logCur, err
			}
			out = append(out, Alert{fmt.Sprintf("alert:flag:%d", id), fmt.Sprintf("🚩 **Anti-cheat flag #%d** (high): %s on **%s** (`%s`)%s%s",
				id, strings.ReplaceAll(typ, "_", " "), clean(name), uid, short(details), link(fmt.Sprintf("/admin/anticheat?flag=%d", id)))})
		}
		rows.Close()
	}

	if (cfg.Money > 0 || cfg.Vault) && maxLog > logCur {
		rows, err := pool.Query(ctx, `
			SELECT l.id, l.event_type, l.player_id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id), COALESCE(l.details::text, '{}'),
			       COALESCE((SELECT COALESCE(NULLIF(t.name, ''), t.uid) FROM players t WHERE t.uid = l.details ->> 'to'), '')
			FROM player_log l JOIN players p ON p.id = l.player_id
			WHERE l.id > $1 AND l.id <= $2
			  AND l.event_type IN ('bank.transfer', 'give.cash', 'shop.sell', 'garage.sell', 'robbery.gas', 'evidence.robbed', 'evidence.breakin')
			ORDER BY l.id`, logCur, maxLog)
		if err != nil {
			return nil, flagCur, logCur, err
		}
		for rows.Next() {
			var id, pid int64
			var typ, name, details, to string
			if err := rows.Scan(&id, &typ, &pid, &name, &details, &to); err != nil {
				rows.Close()
				return nil, flagCur, logCur, err
			}
			d := map[string]any{}
			_ = json.Unmarshal([]byte(details), &d)
			who := "**" + clean(name) + "**"
			where := link(fmt.Sprintf("/admin/players?id=%d&tab=events", pid))
			key := fmt.Sprintf("alert:log:%d", id)
			switch {
			case typ == "evidence.breakin" && cfg.Vault:
				stage := map[string]string{"entrance": "broke into the evidence building", "vaultDoor": "broke through the vault room door"}[fmt.Sprint(d["stage"])]
				if stage == "" {
					stage = "broke into the evidence vault"
				}
				out = append(out, Alert{key, "🚨 " + who + " " + stage + where})
			case typ == "evidence.robbed" && cfg.Vault:
				out = append(out, Alert{key, fmt.Sprintf("🚨 %s **robbed the evidence vault**: %v weapons, %v gear, %v items, %s%s",
					who, num(d["weapons"]), num(d["gear"]), num(d["items"]), dollars(num(d["cash"])), where)})
			case cfg.Money > 0:
				ev, ok := moneyEvents[typ]
				if !ok {
					continue
				}
				amt := num(d[ev[0]])
				if amt < cfg.Money {
					continue
				}
				text := fmt.Sprintf("💰 %s %s **%s**", who, ev[1], dollars(amt))
				if to != "" {
					text += " to **" + clean(to) + "**"
				}
				out = append(out, Alert{key, text + where})
			}
		}
		rows.Close()
	}
	if len(out) > maxAlertsPerPoll {
		extra := len(out) - (maxAlertsPerPoll - 1)
		out = append(out[:maxAlertsPerPoll-1], Alert{fmt.Sprintf("alert:more:%d:%d", maxFlag, maxLog),
			fmt.Sprintf("…and %d more alerts. See the Anti-Cheat page and the Event Log.%s", extra, link("/admin/events"))})
	}
	return out, maxFlag, maxLog, nil
}

// RunAlerts posts staff alerts every 30 s while #staff-alerts is set.
func (b *Bot) RunAlerts(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		if err := b.pollAlerts(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("discord bot: staff alerts failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (b *Bot) pollAlerts(ctx context.Context) error {
	s := b.Settings
	ch := s.Get(ctx, ChannelStaffAlerts)
	cursor := func(key string) int64 {
		n, err := strconv.ParseInt(s.Get(ctx, key), 10, 64)
		if err != nil {
			return -1 // none saved yet: start from now
		}
		return n
	}
	flagCur, logCur := cursor(stateAlertsFlag), cursor(stateAlertsLog)
	if ch == "" {
		// Nowhere to post: keep the cursors at "now" so setting a channel
		// later doesn't post a backlog.
		flagCur, logCur = -1, -1
	}
	money, _ := strconv.ParseInt(s.Get(ctx, NumberAlertMoney), 10, 64)
	cfg := AlertConfig{Flags: s.On(ctx, ToggleAlertFlags), Vault: s.On(ctx, ToggleAlertVault), Money: money, SiteBase: b.deps.SiteBaseURL}
	alerts, newFlag, newLog, err := CollectAlerts(ctx, s.pool, cfg, flagCur, logCur)
	if err != nil {
		return err
	}
	if ch != "" {
		for _, a := range alerts {
			if err := EnqueueNow(ctx, s.pool, OutboxMessage{Kind: KindChannelPost, Target: ch, Content: a.Text, DedupeKey: a.Key}); err != nil {
				return err // cursors not moved: they're retried (dedupe keys stop doubles)
			}
		}
	}
	if err := s.setValue(ctx, stateAlertsFlag, strconv.FormatInt(newFlag, 10)); err != nil {
		return err
	}
	return s.setValue(ctx, stateAlertsLog, strconv.FormatInt(newLog, 10))
}

// setValue writes an internal state value.
func (s *Settings) setValue(ctx context.Context, key, value string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO discord_settings (key, value, updated_at) VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, key, value)
	return err
}

func num(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	}
	return 0
}

func dollars(n int64) string {
	s := strconv.FormatInt(n, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return "$" + s
}

// clean stops a player's name from pinging (@everyone) or breaking markdown.
func clean(s string) string {
	s = strings.ReplaceAll(s, "@", "@​")
	return strings.NewReplacer("*", "\\*", "_", "\\_", "`", "'", "~", "\\~", "|", "\\|", ">", "\\>").Replace(s)
}

// short is a flag's details, compact, for the alert line.
func short(details string) string {
	var m map[string]any
	if details == "" || json.Unmarshal([]byte(details), &m) != nil || len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		b, _ := json.Marshal(m[k])
		parts = append(parts, strings.ReplaceAll(k, "_", " ")+" "+strings.Trim(string(b), `"`))
	}
	out := strings.Join(parts, ", ")
	if len(out) > 160 {
		out = out[:157] + "..."
	}
	return " — " + clean(out)
}
