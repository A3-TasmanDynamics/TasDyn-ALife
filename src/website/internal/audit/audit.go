// Package audit is the single path for recording staff actions and
// attributing rank/role changes (docs/INTEGRATIONS.md §3).
//
// Rank/role changes need no explicit logging call: a database trigger on
// players records them in rank_changes and staff_log. What the app must do
// is attribute them -- call SetActor in the same transaction before the
// UPDATE, or the change is recorded as a 'manual' one.
//
// Other staff actions (bans, notes, ...) call LogStaffAction. Either way,
// internal/audit's Poster sends each staff_log row to #staff-log.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Sources, matching the CHECK constraints on staff_log.source and
// rank_changes.source.
const (
	SourceWebsite = "website"
	SourceDiscord = "discord"
	SourceGame    = "game"
	SourceManual  = "manual"
)

// SetActor attributes every change made later in tx to actorID (0 = no
// player, e.g. a system job), from source, for reason. The settings are
// transaction-local, so they can't leak into another request's writes on
// a pooled connection.
func SetActor(ctx context.Context, tx pgx.Tx, actorID int64, source, reason string) error {
	actor := ""
	if actorID > 0 {
		actor = strconv.FormatInt(actorID, 10)
	}
	_, err := tx.Exec(ctx, `
		SELECT set_config('tasdyn.actor_id', $1, true),
		       set_config('tasdyn.source',   $2, true),
		       set_config('tasdyn.reason',   $3, true)
	`, actor, source, reason)
	return err
}

// Entry is one staff action for LogStaffAction.
type Entry struct {
	StaffID  int64  // 0 = system
	TargetID int64  // 0 = no target
	Action   string // e.g. "ban", "devlog_post"
	Reason   string
	Before   any // marshalled to JSONB; nil = NULL
	After    any
	Source   string // defaults to SourceWebsite
}

// LogStaffAction writes one staff_log row inside tx, so it commits or rolls
// back with the action it describes.
func LogStaffAction(ctx context.Context, tx pgx.Tx, e Entry) error {
	if e.Source == "" {
		e.Source = SourceWebsite
	}
	before, err := jsonOrNil(e.Before)
	if err != nil {
		return err
	}
	after, err := jsonOrNil(e.After)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO staff_log (staff_player_id, target_player_id, action, reason, before_value, after_value, source)
		VALUES (NULLIF($1, 0), NULLIF($2, 0), $3, NULLIF($4, ''), $5, $6, $7)
	`, e.StaffID, e.TargetID, e.Action, e.Reason, before, after, e.Source)
	return err
}

func jsonOrNil(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("audit: marshal: %w", err)
	}
	return b, nil
}

var fieldNames = map[string]string{
	"staff_rank_id": "staff rank",
	"staff_status":  "staff status",
	"staff_team":    "staff team",
	"cop_level":     "police level",
	"medic_level":   "EMS level",
}

// Describe turns a staff_log action and its before/after JSON into the
// short phrase shown on the admin panel and in #staff-log, e.g.
// "changed staff rank: Moderator → Admin". Unknown actions are shown as-is.
func Describe(action string, before, after []byte) string {
	const prefix = "rank_change:"
	if len(action) > len(prefix) && action[:len(prefix)] == prefix {
		field := action[len(prefix):]
		name, ok := fieldNames[field]
		if !ok {
			name = field
		}
		return fmt.Sprintf("changed %s: %s → %s", name, label(before), label(after))
	}
	for prefix, verb := range map[string]string{
		"rank_edit:":          "edited the %s rank",
		"rank_create:":        "created the %s rank",
		"rank_delete:":        "deleted the %s rank",
		"faction_rank_names:": "updated %s rank names",
	} {
		if rest, ok := strings.CutPrefix(action, prefix); ok {
			return fmt.Sprintf(verb, rest)
		}
	}
	if action == "rank_reorder" {
		return "reordered ranks"
	}
	if verb, ok := discordModeration[action]; ok {
		var v struct {
			User string `json:"user"`
		}
		_ = json.Unmarshal(after, &v)
		if v.User != "" {
			return verb + " " + v.User + " in Discord"
		}
		return verb + " someone in Discord"
	}
	if action == "discord.settings" {
		return "updated the Discord bot settings"
	}
	if action == "rules.update" {
		return "updated the server rules"
	}
	if phrase, ok := moderationPhrases[action]; ok {
		return phrase
	}
	return action
}

// moderationPhrases describe the case/ban/anti-cheat actions (internal/cases).
var moderationPhrases = map[string]string{
	"case.open":                  "opened a case",
	"case.close":                 "closed a case",
	"case.reopen":                "reopened a case",
	"player.warn":                "issued punishment points",
	"player.warn_revoke":         "revoked punishment points",
	"ban.issue":                  "banned",
	"ban.lift":                   "lifted a ban",
	"ban.appeal_accept":          "accepted a ban appeal",
	"ban.appeal_reject":          "rejected a ban appeal",
	"anticheat.dismiss":          "dismissed an anti-cheat flag",
	"anticheat.watch":            "put a player on watch from an anti-cheat flag",
	"anticheat.reopen":           "reopened an anti-cheat flag",
	"application.interview":      "moved a staff application to interview",
	"application.accept":         "accepted a staff application",
	"application.reject":         "rejected a staff application",
	"faction.application_reject": "turned down a faction application",
	"notice.post":                "sent an essential notice",
}

// discordModeration are the staff_log actions the bot writes for
// moderation done directly in Discord (DISCORD_BOT.md §5.3). after_value
// holds {"user": target's Discord name, "by": moderator's Discord name}.
var discordModeration = map[string]string{
	"discord.kick":        "kicked",
	"discord.ban":         "banned",
	"discord.unban":       "unbanned",
	"discord.timeout":     "timed out",
	"discord.timeout_end": "removed the timeout on",
}

// discordActor is the moderator's Discord name from a discord.* row, used
// when the moderator hasn't linked a website account.
func discordActor(after []byte) string {
	var v struct {
		By string `json:"by"`
	}
	if len(after) == 0 || json.Unmarshal(after, &v) != nil {
		return ""
	}
	return v.By
}

func label(raw []byte) string {
	var v struct {
		Label string `json:"label"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &v) != nil || v.Label == "" {
		return "none"
	}
	return v.Label
}
