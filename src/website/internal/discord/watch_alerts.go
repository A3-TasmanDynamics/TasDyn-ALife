package discord

// Watchlist alerts: #staff-alerts gets a line when a player on the staff
// watchlist (player_watchlist) joins the game, and when one moves money
// over a lower bar than everyone else's. Like the other alerts, the bot
// remembers how far it got (state.alerts_watch_session / _log).

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	stateWatchSession = "state.alerts_watch_session" // last player_sessions.id looked at
	stateWatchLog     = "state.alerts_watch_log"     // last player_log.id looked at
)

// WatchConfig is what the settings ask for.
type WatchConfig struct {
	Join  bool  // a watched player joins
	Money int64 // dollars; 0 = off
	// General is everyone's money threshold (0 = off): a move at or over it
	// is already posted by the general alert, so it isn't posted twice.
	General  int64
	SiteBase string
}

var factionWords = map[string]string{"civ": "a civilian", "cop": "police", "medic": "EMS"}

// CollectWatchAlerts returns the watchlist alerts since the cursors
// (session id, log id) and the new cursors. A cursor below 0 starts from now.
func CollectWatchAlerts(ctx context.Context, pool *pgxpool.Pool, cfg WatchConfig, sessCur, logCur int64) ([]Alert, int64, int64, error) {
	var maxSess, maxLog int64
	if err := pool.QueryRow(ctx, `SELECT COALESCE((SELECT max(id) FROM player_sessions), 0), COALESCE((SELECT max(id) FROM player_log), 0)`).Scan(&maxSess, &maxLog); err != nil {
		return nil, sessCur, logCur, err
	}
	if sessCur < 0 || sessCur > maxSess {
		sessCur = maxSess
	}
	if logCur < 0 || logCur > maxLog {
		logCur = maxLog
	}
	link := func(path string) string {
		if cfg.SiteBase == "" {
			return ""
		}
		return " <" + strings.TrimRight(cfg.SiteBase, "/") + path + ">"
	}
	var out []Alert

	if cfg.Join && maxSess > sessCur {
		rows, err := pool.Query(ctx, `
			SELECT s.id, p.id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id), COALESCE(s.faction, ''), w.reason
			FROM player_sessions s
			JOIN player_watchlist w ON w.player_id = s.player_id
			JOIN players p ON p.id = s.player_id
			WHERE s.id > $1 AND s.id <= $2 ORDER BY s.id`, sessCur, maxSess)
		if err != nil {
			return nil, sessCur, logCur, err
		}
		for rows.Next() {
			var id, pid int64
			var name, faction, reason string
			if err := rows.Scan(&id, &pid, &name, &faction, &reason); err != nil {
				rows.Close()
				return nil, sessCur, logCur, err
			}
			as := ""
			if w, ok := factionWords[faction]; ok {
				as = " as " + w
			}
			out = append(out, Alert{fmt.Sprintf("alert:watchjoin:%d", id),
				fmt.Sprintf("👁️ Watched player **%s** joined%s — %s%s", clean(name), as, clean(clip(reason, 140)), link(fmt.Sprintf("/admin/players?id=%d", pid)))})
		}
		rows.Close()
	}

	if cfg.Money > 0 && maxLog > logCur {
		rows, err := pool.Query(ctx, `
			SELECT l.id, l.event_type, p.id, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id), COALESCE(l.details::text, '{}'),
			       COALESCE((SELECT COALESCE(NULLIF(t.name, ''), t.uid) FROM players t WHERE t.uid = l.details ->> 'to'), '')
			FROM player_log l
			JOIN player_watchlist w ON w.player_id = l.player_id
			JOIN players p ON p.id = l.player_id
			WHERE l.id > $1 AND l.id <= $2
			  AND l.event_type IN ('bank.transfer', 'give.cash', 'shop.sell', 'garage.sell', 'robbery.gas', 'evidence.robbed')
			ORDER BY l.id`, logCur, maxLog)
		if err != nil {
			return nil, sessCur, logCur, err
		}
		for rows.Next() {
			var id, pid int64
			var typ, name, details, to string
			if err := rows.Scan(&id, &typ, &pid, &name, &details, &to); err != nil {
				rows.Close()
				return nil, sessCur, logCur, err
			}
			ev, ok := moneyEvents[typ]
			if !ok {
				continue
			}
			d := map[string]any{}
			_ = json.Unmarshal([]byte(details), &d)
			amt := num(d[ev[0]])
			if amt < cfg.Money || (cfg.General > 0 && amt >= cfg.General) {
				continue
			}
			text := fmt.Sprintf("👁️ Watched player **%s** %s **%s**", clean(name), ev[1], dollars(amt))
			if to != "" {
				text += " to **" + clean(to) + "**"
			}
			out = append(out, Alert{fmt.Sprintf("alert:watchlog:%d", id), text + link(fmt.Sprintf("/admin/players?id=%d&tab=events", pid))})
		}
		rows.Close()
	}
	if len(out) > maxAlertsPerPoll {
		extra := len(out) - (maxAlertsPerPoll - 1)
		out = append(out[:maxAlertsPerPoll-1], Alert{fmt.Sprintf("alert:watchmore:%d:%d", maxSess, maxLog),
			fmt.Sprintf("…and %d more watchlist alerts. See the Watchlist page.%s", extra, link("/admin/watchlist"))})
	}
	return out, maxSess, maxLog, nil
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
